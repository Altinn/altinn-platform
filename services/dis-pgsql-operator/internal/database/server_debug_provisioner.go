package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
)

// serverDebugOptions configures server-wide debug access provisioning: one
// managed NOLOGIN role that holds the built-in monitoring roles and CONNECT on
// every database, with each principal granted membership in it.
type serverDebugOptions struct {
	ServerName   string
	BuiltinRoles []string
	Principals   []AccessPrincipal
	UseAAD       bool
}

// ensureServerDebugAccess grants read-only debug access across the whole server.
// It is idempotent: it ensures a single managed debug role, grants the built-in
// roles and CONNECT on all non-template databases to it, creates each principal,
// makes each principal a member of the managed role, and reconciles memberships
// so principals no longer requested are revoked. It mirrors ensureAccess but
// operates at server scope (no schema/ownership) against the maintenance
// database, where the pgaadauth_* helpers live and CONNECT can be granted on
// every database.
func ensureServerDebugAccess(ctx context.Context, conn pgxConn, principalConn pgxConn, opts serverDebugOptions) error {
	builtinRoles := NormalizeBuiltinRoles(strings.Join(opts.BuiltinRoles, ","))
	if len(builtinRoles) == 0 {
		return fmt.Errorf("server debug access requires at least one built-in role")
	}

	debugRole := managedDebugRoleName(opts.ServerName, builtinRoles)

	if err := ensureManagedRole(ctx, conn, debugRole); err != nil {
		return err
	}

	// Grant the built-in monitoring roles (pg_monitor, pg_read_all_data, ...) to
	// the managed role. On Azure the connecting Entra admin is a member of
	// azure_pg_admin, which holds ADMIN OPTION on these predefined roles, so the
	// grant succeeds; a locked-down admin without that admin option surfaces the
	// failure here rather than silently skipping.
	for _, builtinRole := range builtinRoles {
		if _, err := conn.Exec(ctx, grantRoleSQL(builtinRole, debugRole)); err != nil {
			return fmt.Errorf("grant built-in role %s to debug role %s: %w", builtinRole, debugRole, err)
		}
	}

	databases, err := connectableDatabases(ctx, conn)
	if err != nil {
		return err
	}
	for _, dbName := range databases {
		if _, err := conn.Exec(ctx, grantConnectSQL(dbName, debugRole)); err != nil {
			return fmt.Errorf("grant connect on database %s to debug role %s: %w", dbName, debugRole, err)
		}
	}

	desiredMembers := newSingleRoleMemberships(debugRole)
	for i, principal := range opts.Principals {
		principal = normalizeAccessPrincipal(principal)
		if err := validateDebugAccessPrincipal(principal, opts.UseAAD); err != nil {
			return fmt.Errorf("debug access principal %d: %w", i, err)
		}
		if err := ensurePrincipal(ctx, principalConn, principal.Name, principal.PrincipalID, string(principal.PrincipalType), opts.UseAAD); err != nil {
			return err
		}
		desiredMembers.add(debugRole, principal.Name)
	}

	if err := reconcileManagedRoleMemberships(ctx, conn, desiredMembers); err != nil {
		return err
	}

	return nil
}

// validateDebugAccessPrincipal mirrors validateAccessPrincipal but ignores the
// Role field, which is unused in server debug mode (all principals map to the
// single managed debug role).
func validateDebugAccessPrincipal(principal AccessPrincipal, useAAD bool) error {
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
	return nil
}

// managedDebugRoleName returns the deterministic NOLOGIN role name for a server's
// debug access. The hash covers the server name and the built-in role set so a
// change to either yields a distinct role, mirroring managedRoleName.
func managedDebugRoleName(serverName string, builtinRoles []string) string {
	builtin := strings.Join(NormalizeBuiltinRoles(strings.Join(builtinRoles, ",")), ",")
	source := strings.Join([]string{serverName, "debug", builtin}, ":")
	slug := naming.SanitizeLowerHyphen(serverName + "-debug")
	if slug == "" {
		slug = "debug"
	}
	hash := naming.StableSHA256Hex(source)[:8]
	return naming.WithRequiredSuffix("dispg-"+slug, "-"+hash, 63, "dispg-debug")
}

// newSingleRoleMemberships builds a memberships set for a single managed role
// with no seeded members, reusing the reconcile machinery used by ensureAccess.
func newSingleRoleMemberships(roleName string) *managedRoleMemberships {
	return &managedRoleMemberships{
		roles: []string{roleName},
		members: map[string]map[string]struct{}{
			roleName: {},
		},
	}
}

// connectableDatabases returns the names of all non-template databases that
// allow connections, so CONNECT can be granted on each to the debug role.
func connectableDatabases(ctx context.Context, conn pgxConn) ([]string, error) {
	rows, err := conn.Query(ctx, listConnectableDatabasesSQL())
	if err != nil {
		return nil, fmt.Errorf("list connectable databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan database name: %w", err)
		}
		databases = append(databases, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate databases: %w", err)
	}
	return databases, nil
}

func listConnectableDatabasesSQL() string {
	return "SELECT datname FROM pg_database WHERE NOT datistemplate AND datallowconn;"
}
