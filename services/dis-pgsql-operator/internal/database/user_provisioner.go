package database

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	userProvisionScope = "https://ossrdbms-aad.database.windows.net/.default"

	// maintenanceDatabase holds the cluster-global pgaadauth_* helper functions on
	// Azure Flexible Server; they do not exist in freshly created databases, so
	// principal creation must run here even though the resulting role is global.
	maintenanceDatabase = "postgres"
)

// RunUserProvisioner connects to the database using workload identity and
// ensures the normal user exists with appropriate permissions.
func RunUserProvisioner(ctx context.Context) error {
	serverName := strings.TrimSpace(os.Getenv(DatabaseServerNameEnv))
	adminAppIdentity := strings.TrimSpace(os.Getenv(AdminAppIdentityEnv))
	schemaName := strings.TrimSpace(os.Getenv(DBSchemaEnv))
	disableAAD := parseBoolEnv(os.Getenv(DisableAADEnv))
	revokePublicConnect := parseBoolEnv(os.Getenv(RevokePublicConnectEnv))
	databaseScopedSearchPath := strings.EqualFold(strings.TrimSpace(os.Getenv(DBSearchPathScopeEnv)), "database")
	sslMode := strings.TrimSpace(os.Getenv("DISPG_DB_SSLMODE"))

	if serverName == "" {
		return fmt.Errorf("%s must be set", DatabaseServerNameEnv)
	}
	if adminAppIdentity == "" && !disableAAD {
		return fmt.Errorf("%s must be set", AdminAppIdentityEnv)
	}
	if schemaName == "" {
		schemaName = serverName
	}

	accessPrincipals, err := accessPrincipalsFromEnv(disableAAD)
	if err != nil {
		return err
	}

	host := strings.TrimSpace(os.Getenv(DBHostEnv))
	if host == "" {
		if disableAAD {
			host = "postgres.default.svc"
		} else {
			// In Azure mode the controller publishes the real Flexible Server FQDN
			// from DatabaseServer.Status.Host (server.Status.FullyQualifiedDomainName).
			// Deriving "<serverName>.postgres.database.azure.com" is no longer correct
			// because AzureName now carries a stable uniqueness suffix.
			return fmt.Errorf("%s must be set when AAD is enabled", DBHostEnv)
		}
	}
	dbName := strings.TrimSpace(os.Getenv(DBNameEnv))
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
		adminUser := strings.TrimSpace(os.Getenv(DBAdminUserEnv))
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

	principalConn := conn
	if !disableAAD && !strings.EqualFold(dbName, maintenanceDatabase) {
		maintenanceCfg := cfg.Copy()
		maintenanceCfg.Database = maintenanceDatabase
		maintenanceConn, connErr := pgx.ConnectConfig(ctx, maintenanceCfg)
		if connErr != nil {
			return fmt.Errorf("connect maintenance database %q: %w", maintenanceDatabase, connErr)
		}
		defer func() {
			if err := maintenanceConn.Close(ctx); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "close maintenance connection: %v\n", err)
			}
		}()
		principalConn = maintenanceConn
	}

	if err := ensureAccess(ctx, conn, principalConn, accessOptions{
		DatabaseName:             dbName,
		SchemaName:               schemaName,
		Principals:               accessPrincipals,
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
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type accessOptions struct {
	DatabaseName             string
	SchemaName               string
	Principals               []AccessPrincipal
	UseAAD                   bool
	RevokePublicConnect      bool
	DatabaseScopedSearchPath bool
}

func ensureAccess(ctx context.Context, conn pgxConn, principalConn pgxConn, opts accessOptions) error {
	if opts.RevokePublicConnect {
		if _, err := conn.Exec(ctx, revokePublicConnectSQL(opts.DatabaseName)); err != nil {
			return fmt.Errorf("revoke public connect: %w", err)
		}
	}

	accessRoles := managedAccessRolesFor(opts.DatabaseName, opts.SchemaName)
	for _, roleName := range accessRoles.all() {
		if err := ensureManagedRole(ctx, conn, roleName); err != nil {
			return err
		}
	}

	if _, err := conn.Exec(ctx, createSchemaSQL(opts.SchemaName, accessRoles.Owner)); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	if _, err := conn.Exec(ctx, alterSchemaOwnerSQL(opts.SchemaName, accessRoles.Owner)); err != nil {
		return fmt.Errorf("alter schema owner: %w", err)
	}

	if err := ensureManagedRoleGrants(ctx, conn, opts, accessRoles); err != nil {
		return err
	}

	desiredMembers := newManagedRoleMemberships(accessRoles)
	for _, principal := range opts.Principals {
		principal = normalizeAccessPrincipal(principal)
		if err := validateAccessPrincipal(principal, opts.UseAAD); err != nil {
			return err
		}
		if err := ensurePrincipal(ctx, principalConn, principal.Name, principal.PrincipalID, string(principal.PrincipalType), opts.UseAAD); err != nil {
			return err
		}
		roleName, err := accessRoles.roleFor(principal.Role)
		if err != nil {
			return err
		}
		desiredMembers.add(roleName, principal.Name)
		if _, err := conn.Exec(ctx, setSearchPathSQL(principal.Name, opts.DatabaseName, opts.SchemaName, opts.DatabaseScopedSearchPath)); err != nil {
			return fmt.Errorf("set principal role search_path: %w", err)
		}
		if principal.Role == AccessRoleOwner {
			if err := grantDefaultPrivilegesForCreator(ctx, conn, principal.Name, opts.SchemaName, accessRoles); err != nil {
				return err
			}
		}
	}

	if err := grantDefaultPrivilegesForCreator(ctx, conn, accessRoles.Owner, opts.SchemaName, accessRoles); err != nil {
		return err
	}

	if err := reconcileManagedRoleMemberships(ctx, conn, desiredMembers); err != nil {
		return err
	}

	return nil
}

func accessPrincipalsFromEnv(disableAAD bool) ([]AccessPrincipal, error) {
	principals, err := ParseAccessPrincipalsPayload(os.Getenv(AccessPrincipalsEnv))
	if err != nil {
		return nil, err
	}
	return validateAccessPrincipals(principals, !disableAAD)
}

func validateAccessPrincipals(principals []AccessPrincipal, useAAD bool) ([]AccessPrincipal, error) {
	if len(principals) == 0 {
		return nil, fmt.Errorf("%s must contain at least one principal", AccessPrincipalsEnv)
	}

	normalized := make([]AccessPrincipal, 0, len(principals))
	seen := map[string]struct{}{}
	for i, principal := range principals {
		principal = normalizeAccessPrincipal(principal)
		if err := validateAccessPrincipal(principal, useAAD); err != nil {
			return nil, fmt.Errorf("access principal %d: %w", i, err)
		}
		keyValue := principal.PrincipalID
		if keyValue == "" {
			keyValue = principal.Name
		}
		key := string(principal.PrincipalType) + ":" + strings.ToLower(keyValue)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("access principal %d duplicates another principal", i)
		}
		seen[key] = struct{}{}
		normalized = append(normalized, principal)
	}
	return normalized, nil
}

func normalizeAccessPrincipal(principal AccessPrincipal) AccessPrincipal {
	principal.Role = AccessRole(strings.TrimSpace(string(principal.Role)))
	principal.Name = strings.TrimSpace(principal.Name)
	principal.PrincipalID = strings.TrimSpace(principal.PrincipalID)
	principal.PrincipalType = PrincipalType(strings.TrimSpace(string(principal.PrincipalType)))
	return principal
}

func validateAccessPrincipal(principal AccessPrincipal, useAAD bool) error {
	if principal.Name == "" {
		return fmt.Errorf("name must be set")
	}
	if principal.PrincipalID == "" && useAAD {
		return fmt.Errorf("principalId must be set")
	}
	switch principal.PrincipalType {
	case PrincipalTypeService, PrincipalTypeGroup:
	default:
		return fmt.Errorf("principalType must be service or group")
	}
	switch principal.Role {
	case AccessRoleReader, AccessRoleWriter, AccessRoleOwner:
	default:
		return fmt.Errorf("role must be Reader, Writer, or Owner")
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

type managedAccessRoles struct {
	Reader string
	Writer string
	Owner  string
}

func managedAccessRolesFor(databaseName, schemaName string) managedAccessRoles {
	return managedAccessRoles{
		Reader: managedRoleName(databaseName, schemaName, AccessRoleReader),
		Writer: managedRoleName(databaseName, schemaName, AccessRoleWriter),
		Owner:  managedRoleName(databaseName, schemaName, AccessRoleOwner),
	}
}

func managedRoleName(databaseName, schemaName string, role AccessRole) string {
	roleName := strings.ToLower(string(role))
	source := strings.Join([]string{databaseName, schemaName, roleName}, ":")
	slug := naming.SanitizeLowerHyphen(strings.Join([]string{databaseName, schemaName, roleName}, "-"))
	if slug == "" {
		slug = roleName
	}
	hash := naming.StableSHA256Hex(source)[:8]
	return naming.WithRequiredSuffix("dispg-"+slug, "-"+hash, 63, "dispg-"+roleName)
}

func (roles managedAccessRoles) all() []string {
	return []string{roles.Reader, roles.Writer, roles.Owner}
}

func (roles managedAccessRoles) roleFor(role AccessRole) (string, error) {
	switch role {
	case AccessRoleReader:
		return roles.Reader, nil
	case AccessRoleWriter:
		return roles.Writer, nil
	case AccessRoleOwner:
		return roles.Owner, nil
	default:
		return "", fmt.Errorf("unsupported access role %q", role)
	}
}

func ensureManagedRole(ctx context.Context, conn pgxConn, roleName string) error {
	var exists bool
	if err := conn.QueryRow(ctx, roleExistsSQL(), roleName).Scan(&exists); err != nil {
		return fmt.Errorf("check managed role existence: %w", err)
	}
	if exists {
		return nil
	}
	if _, err := conn.Exec(ctx, createNoLoginRoleSQL(roleName)); err != nil {
		return fmt.Errorf("create managed role: %w", err)
	}
	return nil
}

func ensureManagedRoleGrants(
	ctx context.Context,
	conn pgxConn,
	opts accessOptions,
	roles managedAccessRoles,
) error {
	statements := []struct {
		name string
		sql  string
	}{
		{name: "grant reader connect", sql: grantConnectSQL(opts.DatabaseName, roles.Reader)},
		{name: "grant reader schema usage", sql: grantSchemaUsageSQL(opts.SchemaName, roles.Reader)},
		{name: "grant reader table read", sql: grantAllTablesReadSQL(opts.SchemaName, roles.Reader)},
		{name: "grant reader sequence read", sql: grantAllSequencesReadSQL(opts.SchemaName, roles.Reader)},
		{name: "grant writer table write", sql: grantAllTablesWriteSQL(opts.SchemaName, roles.Writer)},
		{name: "grant writer sequence write", sql: grantAllSequencesWriteSQL(opts.SchemaName, roles.Writer)},
		{name: "grant owner schema usage", sql: grantSchemaUsageSQL(opts.SchemaName, roles.Owner)},
		{name: "grant owner schema create", sql: grantSchemaCreateSQL(opts.SchemaName, roles.Owner)},
	}

	for _, statement := range statements {
		if _, err := conn.Exec(ctx, statement.sql); err != nil {
			return fmt.Errorf("%s: %w", statement.name, err)
		}
	}

	return nil
}

func grantDefaultPrivilegesForCreator(
	ctx context.Context,
	conn pgxConn,
	creator,
	schemaName string,
	roles managedAccessRoles,
) error {
	statements := []struct {
		name string
		sql  string
	}{
		{name: "grant default reader table read", sql: alterDefaultTableReadPrivilegesSQL(creator, schemaName, roles.Reader)},
		{name: "grant default reader sequence read", sql: alterDefaultSequenceReadPrivilegesSQL(creator, schemaName, roles.Reader)},
		{name: "grant default writer table write", sql: alterDefaultTableWritePrivilegesSQL(creator, schemaName, roles.Writer)},
		{name: "grant default writer sequence write", sql: alterDefaultSequenceWritePrivilegesSQL(creator, schemaName, roles.Writer)},
	}

	for _, statement := range statements {
		if _, err := conn.Exec(ctx, statement.sql); err != nil {
			return fmt.Errorf("%s: %w", statement.name, err)
		}
	}
	return nil
}

type managedRoleMemberships struct {
	roles   []string
	members map[string]map[string]struct{}
}

func newManagedRoleMemberships(roles managedAccessRoles) *managedRoleMemberships {
	memberships := &managedRoleMemberships{
		roles:   roles.all(),
		members: map[string]map[string]struct{}{},
	}
	for _, role := range memberships.roles {
		memberships.members[role] = map[string]struct{}{}
	}
	memberships.add(roles.Reader, roles.Writer)
	memberships.add(roles.Writer, roles.Owner)
	return memberships
}

func (memberships *managedRoleMemberships) add(roleName, memberName string) {
	if memberships.members[roleName] == nil {
		memberships.members[roleName] = map[string]struct{}{}
	}
	memberships.members[roleName][memberName] = struct{}{}
}

func reconcileManagedRoleMemberships(ctx context.Context, conn pgxConn, memberships *managedRoleMemberships) error {
	for _, roleName := range memberships.roles {
		desired := memberships.members[roleName]
		actual, err := managedRoleMembers(ctx, conn, roleName)
		if err != nil {
			return err
		}

		for memberName := range desired {
			if _, err := conn.Exec(ctx, grantRoleSQL(roleName, memberName)); err != nil {
				return fmt.Errorf("grant managed role %s to %s: %w", roleName, memberName, err)
			}
		}
		for memberName := range actual {
			if _, ok := desired[memberName]; ok {
				continue
			}
			if _, err := conn.Exec(ctx, revokeRoleSQL(roleName, memberName)); err != nil {
				return fmt.Errorf("revoke managed role %s from %s: %w", roleName, memberName, err)
			}
		}
	}
	return nil
}

func managedRoleMembers(ctx context.Context, conn pgxConn, roleName string) (map[string]struct{}, error) {
	rows, err := conn.Query(ctx, roleMembersSQL(), roleName)
	if err != nil {
		return nil, fmt.Errorf("list members for managed role %s: %w", roleName, err)
	}
	defer rows.Close()

	members := map[string]struct{}{}
	for rows.Next() {
		var memberName string
		if err := rows.Scan(&memberName); err != nil {
			return nil, fmt.Errorf("scan member for managed role %s: %w", roleName, err)
		}
		members[memberName] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate members for managed role %s: %w", roleName, err)
	}
	return members, nil
}

func roleExistsSQL() string {
	return "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)"
}

func createRoleSQL(user string) string {
	return fmt.Sprintf("CREATE ROLE %s LOGIN;", pgx.Identifier{user}.Sanitize())
}

func createNoLoginRoleSQL(role string) string {
	return fmt.Sprintf("CREATE ROLE %s NOLOGIN;", pgx.Identifier{role}.Sanitize())
}

func createAADPrincipalSQL() string {
	return "SELECT * FROM pgaadauth_create_principal_with_oid($1, $2, $3, false, false)"
}

func roleMembersSQL() string {
	// CURRENT_USER is excluded: PostgreSQL auto-grants the creating role membership
	// WITH ADMIN OPTION on CREATE ROLE, so the connecting admin shows up as a member
	// of every managed role it created. Reconciling that membership away fails with
	// "dependent privileges exist" (2BP01) because the admin used it to grant the
	// role onward, so it must never be treated as a revocable member.
	return `SELECT member_role.rolname
FROM pg_auth_members membership
JOIN pg_roles role_role ON role_role.oid = membership.roleid
JOIN pg_roles member_role ON member_role.oid = membership.member
WHERE role_role.rolname = $1
  AND member_role.rolname <> CURRENT_USER`
}

func grantRoleSQL(role, member string) string {
	return fmt.Sprintf("GRANT %s TO %s;",
		pgx.Identifier{role}.Sanitize(),
		pgx.Identifier{member}.Sanitize(),
	)
}

func revokeRoleSQL(role, member string) string {
	return fmt.Sprintf("REVOKE %s FROM %s;",
		pgx.Identifier{role}.Sanitize(),
		pgx.Identifier{member}.Sanitize(),
	)
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

func grantSchemaUsageSQL(schemaName, role string) string {
	return fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s;",
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{role}.Sanitize(),
	)
}

func grantSchemaCreateSQL(schemaName, role string) string {
	return fmt.Sprintf("GRANT CREATE ON SCHEMA %s TO %s;",
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{role}.Sanitize(),
	)
}

func grantAllTablesReadSQL(schemaName, role string) string {
	return fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA %s TO %s;",
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{role}.Sanitize(),
	)
}

func grantAllTablesWriteSQL(schemaName, role string) string {
	return fmt.Sprintf("GRANT INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %s TO %s;",
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{role}.Sanitize(),
	)
}

func grantAllSequencesReadSQL(schemaName, role string) string {
	return fmt.Sprintf("GRANT SELECT ON ALL SEQUENCES IN SCHEMA %s TO %s;",
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{role}.Sanitize(),
	)
}

func grantAllSequencesWriteSQL(schemaName, role string) string {
	return fmt.Sprintf("GRANT USAGE, UPDATE ON ALL SEQUENCES IN SCHEMA %s TO %s;",
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{role}.Sanitize(),
	)
}

func alterDefaultTableReadPrivilegesSQL(creator, schemaName, role string) string {
	return fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA %s GRANT SELECT ON TABLES TO %s;",
		pgx.Identifier{creator}.Sanitize(),
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{role}.Sanitize(),
	)
}

func alterDefaultTableWritePrivilegesSQL(creator, schemaName, role string) string {
	return fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA %s GRANT INSERT, UPDATE, DELETE ON TABLES TO %s;",
		pgx.Identifier{creator}.Sanitize(),
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{role}.Sanitize(),
	)
}

func alterDefaultSequenceReadPrivilegesSQL(creator, schemaName, role string) string {
	return fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA %s GRANT SELECT ON SEQUENCES TO %s;",
		pgx.Identifier{creator}.Sanitize(),
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{role}.Sanitize(),
	)
}

func alterDefaultSequenceWritePrivilegesSQL(creator, schemaName, role string) string {
	return fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA %s GRANT USAGE, UPDATE ON SEQUENCES TO %s;",
		pgx.Identifier{creator}.Sanitize(),
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{role}.Sanitize(),
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

func parseBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
