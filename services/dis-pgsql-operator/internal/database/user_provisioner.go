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
	appIdentity := firstNonEmptyEnv("DISPG_APP_IDENTITY_NAME", "DISPG_USER_APP_IDENTITY")
	appPrincipalID := firstNonEmptyEnv("DISPG_APP_IDENTITY_ID", "DISPG_USER_APP_PRINCIPAL_ID")
	ownerIdentity := strings.TrimSpace(os.Getenv("DISPG_OWNER_IDENTITY_NAME"))
	ownerPrincipalID := strings.TrimSpace(os.Getenv("DISPG_OWNER_IDENTITY_ID"))
	adminAppIdentity := strings.TrimSpace(os.Getenv("DISPG_ADMIN_APP_IDENTITY"))
	schemaName := strings.TrimSpace(os.Getenv("DISPG_DB_SCHEMA"))
	disableAAD := parseBoolEnv(os.Getenv("DISPG_DISABLE_AAD"))
	revokePublicConnect := parseBoolEnv(os.Getenv("DISPG_REVOKE_PUBLIC_CONNECT"))
	databaseScopedSearchPath := strings.EqualFold(strings.TrimSpace(os.Getenv("DISPG_DB_SEARCH_PATH_SCOPE")), "database")
	sslMode := strings.TrimSpace(os.Getenv("DISPG_DB_SSLMODE"))

	if serverName == "" {
		return fmt.Errorf("DISPG_DATABASE_NAME must be set")
	}
	if appIdentity == "" {
		return fmt.Errorf("DISPG_APP_IDENTITY_NAME or DISPG_USER_APP_IDENTITY must be set")
	}
	if appPrincipalID == "" && !disableAAD {
		return fmt.Errorf("DISPG_APP_IDENTITY_ID or DISPG_USER_APP_PRINCIPAL_ID must be set")
	}
	if ownerIdentity != "" || ownerPrincipalID != "" {
		if ownerIdentity == "" {
			return fmt.Errorf("DISPG_OWNER_IDENTITY_NAME must be set when owner access is configured")
		}
		if ownerPrincipalID == "" && !disableAAD {
			return fmt.Errorf("DISPG_OWNER_IDENTITY_ID must be set when owner access is configured")
		}
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

	if err := ensureAccess(ctx, conn, accessOptions{
		DatabaseName:             dbName,
		SchemaName:               schemaName,
		AppName:                  appIdentity,
		AppPrincipalID:           appPrincipalID,
		OwnerName:                ownerIdentity,
		OwnerPrincipalID:         ownerPrincipalID,
		UseAAD:                   !disableAAD,
		RevokePublicConnect:      revokePublicConnect,
		DatabaseScopedSearchPath: databaseScopedSearchPath,
	}); err != nil {
		return err
	}

	return nil
}

type pgxConn interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type accessOptions struct {
	DatabaseName             string
	SchemaName               string
	AppName                  string
	AppPrincipalID           string
	OwnerName                string
	OwnerPrincipalID         string
	UseAAD                   bool
	RevokePublicConnect      bool
	DatabaseScopedSearchPath bool
}

func ensureUser(ctx context.Context, conn pgxConn, user, principalID, dbName, schemaName string, useAAD bool) error {
	return ensureAccess(ctx, conn, accessOptions{
		DatabaseName:   dbName,
		SchemaName:     schemaName,
		AppName:        user,
		AppPrincipalID: principalID,
		UseAAD:         useAAD,
	})
}

func ensureAccess(ctx context.Context, conn pgxConn, opts accessOptions) error {
	if opts.RevokePublicConnect {
		if _, err := conn.Exec(ctx, revokePublicConnectSQL(opts.DatabaseName)); err != nil {
			return fmt.Errorf("revoke public connect: %w", err)
		}
	}

	if err := ensurePrincipal(ctx, conn, opts.AppName, opts.AppPrincipalID, "service", opts.UseAAD); err != nil {
		return err
	}

	if opts.OwnerName != "" {
		if err := ensurePrincipal(ctx, conn, opts.OwnerName, opts.OwnerPrincipalID, "group", opts.UseAAD); err != nil {
			return err
		}
	}

	if _, err := conn.Exec(ctx, grantConnectSQL(opts.DatabaseName, opts.AppName)); err != nil {
		return fmt.Errorf("grant app connect: %w", err)
	}

	if opts.OwnerName != "" {
		if _, err := conn.Exec(ctx, grantConnectSQL(opts.DatabaseName, opts.OwnerName)); err != nil {
			return fmt.Errorf("grant owner connect: %w", err)
		}
	}

	if _, err := conn.Exec(ctx, createSchemaSQL(opts.SchemaName, opts.AppName)); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	if _, err := conn.Exec(ctx, alterSchemaOwnerSQL(opts.SchemaName, opts.AppName)); err != nil {
		return fmt.Errorf("alter schema owner: %w", err)
	}

	if opts.OwnerName != "" {
		if err := grantOwnerSchemaAccess(ctx, conn, opts); err != nil {
			return err
		}
	}

	if _, err := conn.Exec(ctx, setSearchPathSQL(opts.AppName, opts.DatabaseName, opts.SchemaName, opts.DatabaseScopedSearchPath)); err != nil {
		return fmt.Errorf("set app role search_path: %w", err)
	}

	if opts.OwnerName != "" {
		if _, err := conn.Exec(ctx, setSearchPathSQL(opts.OwnerName, opts.DatabaseName, opts.SchemaName, opts.DatabaseScopedSearchPath)); err != nil {
			return fmt.Errorf("set owner role search_path: %w", err)
		}
	}

	return nil
}

func ensurePrincipal(ctx context.Context, conn pgxConn, name, principalID, principalType string, useAAD bool) error {
	var exists bool
	if err := conn.QueryRow(ctx, roleExistsSQL(), name).Scan(&exists); err != nil {
		return fmt.Errorf("check role existence: %w", err)
	}

	if !exists {
		if useAAD {
			if principalID == "" {
				return fmt.Errorf("principal id is required for Entra user provisioning")
			}
			if _, err := conn.Exec(ctx, createAADPrincipalSQL(), name, principalID, principalType); err != nil {
				return fmt.Errorf("create Entra principal: %w", err)
			}
		} else {
			if _, err := conn.Exec(ctx, createRoleSQL(name)); err != nil {
				return fmt.Errorf("create role: %w", err)
			}
		}
	}

	return nil
}

func grantOwnerSchemaAccess(ctx context.Context, conn pgxConn, opts accessOptions) error {
	statements := []struct {
		name string
		sql  string
	}{
		{name: "grant owner schema access", sql: grantSchemaAccessSQL(opts.SchemaName, opts.OwnerName)},
		{name: "grant owner table access", sql: grantAllTablesSQL(opts.SchemaName, opts.OwnerName)},
		{name: "grant owner sequence access", sql: grantAllSequencesSQL(opts.SchemaName, opts.OwnerName)},
		{name: "grant owner function access", sql: grantAllFunctionsSQL(opts.SchemaName, opts.OwnerName)},
		{name: "grant owner default table access", sql: alterDefaultTablePrivilegesSQL(opts.AppName, opts.SchemaName, opts.OwnerName)},
		{name: "grant owner default sequence access", sql: alterDefaultSequencePrivilegesSQL(opts.AppName, opts.SchemaName, opts.OwnerName)},
		{name: "grant owner default function access", sql: alterDefaultFunctionPrivilegesSQL(opts.AppName, opts.SchemaName, opts.OwnerName)},
	}

	for _, statement := range statements {
		if _, err := conn.Exec(ctx, statement.sql); err != nil {
			return fmt.Errorf("%s: %w", statement.name, err)
		}
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

func revokePublicConnectSQL(dbName string) string {
	return fmt.Sprintf("REVOKE CONNECT ON DATABASE %s FROM PUBLIC;",
		pgx.Identifier{dbName}.Sanitize(),
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

func grantSchemaAccessSQL(schemaName, owner string) string {
	return fmt.Sprintf("GRANT USAGE, CREATE ON SCHEMA %s TO %s;",
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{owner}.Sanitize(),
	)
}

func grantAllTablesSQL(schemaName, owner string) string {
	return fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER ON ALL TABLES IN SCHEMA %s TO %s;",
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{owner}.Sanitize(),
	)
}

func grantAllSequencesSQL(schemaName, owner string) string {
	return fmt.Sprintf("GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA %s TO %s;",
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{owner}.Sanitize(),
	)
}

func grantAllFunctionsSQL(schemaName, owner string) string {
	return fmt.Sprintf("GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA %s TO %s;",
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{owner}.Sanitize(),
	)
}

func alterDefaultTablePrivilegesSQL(appName, schemaName, owner string) string {
	return fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA %s GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER ON TABLES TO %s;",
		pgx.Identifier{appName}.Sanitize(),
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{owner}.Sanitize(),
	)
}

func alterDefaultSequencePrivilegesSQL(appName, schemaName, owner string) string {
	return fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA %s GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO %s;",
		pgx.Identifier{appName}.Sanitize(),
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{owner}.Sanitize(),
	)
}

func alterDefaultFunctionPrivilegesSQL(appName, schemaName, owner string) string {
	return fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA %s GRANT EXECUTE ON FUNCTIONS TO %s;",
		pgx.Identifier{appName}.Sanitize(),
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{owner}.Sanitize(),
	)
}

func setSearchPathSQL(user, dbName, schemaName string, databaseScoped bool) string {
	if databaseScoped {
		return fmt.Sprintf("ALTER ROLE %s IN DATABASE %s SET search_path = %s, public;",
			pgx.Identifier{user}.Sanitize(),
			pgx.Identifier{dbName}.Sanitize(),
			pgx.Identifier{schemaName}.Sanitize(),
		)
	}
	return fmt.Sprintf("ALTER ROLE %s SET search_path = %s, public;",
		pgx.Identifier{user}.Sanitize(),
		pgx.Identifier{schemaName}.Sanitize(),
	)
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func parseBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
