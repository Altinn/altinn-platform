package database

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	userProvisionScope = "https://ossrdbms-aad.database.windows.net/.default"
)

// RunUserProvisioner connects to the database using workload identity and
// ensures the normal user exists with appropriate permissions.
func RunUserProvisioner(ctx context.Context) error {
	serverName := strings.TrimSpace(os.Getenv("DISPG_DATABASE_NAME"))
	userAppIdentity := strings.TrimSpace(os.Getenv("DISPG_USER_APP_IDENTITY"))
	userAppPrincipalId := strings.TrimSpace(os.Getenv("DISPG_USER_APP_PRINCIPAL_ID"))
	adminAppIdentity := strings.TrimSpace(os.Getenv("DISPG_ADMIN_APP_IDENTITY"))
	schemaName := strings.TrimSpace(os.Getenv("DISPG_DB_SCHEMA"))
	disableAAD := parseBoolEnv(os.Getenv("DISPG_DISABLE_AAD"))
	sslMode := strings.TrimSpace(os.Getenv("DISPG_DB_SSLMODE"))

	if serverName == "" {
		return fmt.Errorf("DISPG_DATABASE_NAME must be set")
	}
	if userAppIdentity == "" {
		return fmt.Errorf("DISPG_USER_APP_IDENTITY must be set")
	}
	if userAppPrincipalId == "" && !disableAAD {
		return fmt.Errorf("DISPG_USER_APP_PRINCIPAL_ID must be set")
	}
	if adminAppIdentity == "" && !disableAAD {
		return fmt.Errorf("DISPG_ADMIN_APP_IDENTITY must be set")
	}
	if schemaName == "" {
		schemaName = serverName
	}

	host := strings.TrimSpace(os.Getenv("DISPG_DB_HOST"))
	if host == "" {
		if disableAAD {
			host = "postgres.default.svc"
		} else {
			host = fmt.Sprintf("%s.postgres.database.azure.com", serverName)
		}
	}
	dbName := strings.TrimSpace(os.Getenv("DISPG_DB_NAME"))
	if dbName == "" {
		dbName = "postgres"
	}
	if sslMode == "" {
		if disableAAD {
			sslMode = "disable"
		} else {
			sslMode = "require"
		}
	}
	connStr := fmt.Sprintf(
		"host=%s port=5432 dbname=%s sslmode=%s",
		host,
		dbName,
		sslMode,
	)

	cfg, err := pgx.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("parse pgx config: %w", err)
	}
	if disableAAD {
		adminUser := strings.TrimSpace(os.Getenv("DISPG_DB_ADMIN_USER"))
		if adminUser == "" {
			adminUser = "postgres"
		}
		cfg.User = adminUser
		cfg.Password = strings.TrimSpace(os.Getenv("DISPG_DB_PASSWORD"))
	} else {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return fmt.Errorf("azure credential error: %w", err)
		}
		token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
			Scopes: []string{userProvisionScope},
		})
		if err != nil {
			return fmt.Errorf("get azure token: %w", err)
		}
		cfg.User = adminAppIdentity
		cfg.Password = token.Token
	}

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer func() {
		if err := conn.Close(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "close postgres connection: %v\n", err)
		}
	}()

	if err := ensureUser(ctx, conn, userAppIdentity, userAppPrincipalId, dbName, schemaName, !disableAAD); err != nil {
		return err
	}

	return nil
}

type pgxConn interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func ensureUser(ctx context.Context, conn pgxConn, user, principalID, dbName, schemaName string, useAAD bool) error {
	var exists bool
	if err := conn.QueryRow(ctx, roleExistsSQL(), user).Scan(&exists); err != nil {
		return fmt.Errorf("check role existence: %w", err)
	}

	if !exists {
		if useAAD {
			if principalID == "" {
				return fmt.Errorf("principal id is required for Entra user provisioning")
			}
			if _, err := conn.Exec(ctx, createAADPrincipalSQL(), user, principalID, "service"); err != nil {
				return fmt.Errorf("create Entra principal: %w", err)
			}
		} else {
			if _, err := conn.Exec(ctx, createRoleSQL(user)); err != nil {
				return fmt.Errorf("create role: %w", err)
			}
		}
	}

	if _, err := conn.Exec(ctx, grantConnectSQL(dbName, user)); err != nil {
		return fmt.Errorf("grant connect: %w", err)
	}

	if _, err := conn.Exec(ctx, createSchemaSQL(schemaName, user)); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	if _, err := conn.Exec(ctx, alterSchemaOwnerSQL(schemaName, user)); err != nil {
		return fmt.Errorf("alter schema owner: %w", err)
	}

	if _, err := conn.Exec(ctx, setSearchPathSQL(user, schemaName)); err != nil {
		return fmt.Errorf("set role search_path: %w", err)
	}

	return nil
}

func roleExistsSQL() string {
	return "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)"
}

func createRoleSQL(user string) string {
	return fmt.Sprintf("CREATE ROLE %s LOGIN;", pgx.Identifier{user}.Sanitize())
}

func createAADPrincipalSQL() string {
	return "SELECT * FROM pgaadauth_create_principal_with_oid($1, $2, $3, false, false)"
}

func grantConnectSQL(dbName, user string) string {
	return fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s;",
		pgx.Identifier{dbName}.Sanitize(),
		pgx.Identifier{user}.Sanitize(),
	)
}

func createSchemaSQL(schemaName, user string) string {
	return fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s AUTHORIZATION %s;",
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{user}.Sanitize(),
	)
}

func alterSchemaOwnerSQL(schemaName, user string) string {
	return fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s;",
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{user}.Sanitize(),
	)
}

func setSearchPathSQL(user, schemaName string) string {
	return fmt.Sprintf("ALTER ROLE %s SET search_path = %s, public;",
		pgx.Identifier{user}.Sanitize(),
		pgx.Identifier{schemaName}.Sanitize(),
	)
}

func parseBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
