package database

import (
	"context"
	"strings"
	"testing"
)

const (
	testPgMonitor      = "pg_monitor"
	testPgReadAllData  = "pg_read_all_data"
	testDebugServer    = "my-app-db"
	testDebugPrincipal = "svc"
)

func TestListConnectableDatabasesSQL(t *testing.T) {
	got := listConnectableDatabasesSQL()
	want := "SELECT datname FROM pg_database WHERE NOT datistemplate AND datallowconn AND has_database_privilege(current_user, oid, 'CONNECT WITH GRANT OPTION');"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNormalizeBuiltinRoles(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: []string{}},
		{name: "single", in: testPgMonitor, want: []string{testPgMonitor}},
		{
			name: "sorted and trimmed",
			in:   " pg_read_all_data , pg_monitor ",
			want: []string{testPgMonitor, testPgReadAllData},
		},
		{
			name: "dedup",
			in:   "pg_monitor,pg_monitor,pg_read_all_data",
			want: []string{testPgMonitor, testPgReadAllData},
		},
		{name: "drops blanks", in: "pg_monitor,,  ,pg_read_all_data", want: []string{testPgMonitor, testPgReadAllData}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeBuiltinRoles(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %#v, want %#v", got, tc.want)
				}
			}
		})
	}
}

func TestManagedDebugRoleName(t *testing.T) {
	roles := []string{testPgMonitor, testPgReadAllData}

	name := managedDebugRoleName(testDebugServer, roles)
	if !strings.HasPrefix(name, "dispg-") {
		t.Fatalf("expected dispg- prefix, got %q", name)
	}
	if len(name) == 0 || len(name) > 63 {
		t.Fatalf("expected non-empty name within 63 chars, got %q (len=%d)", name, len(name))
	}
	if !strings.Contains(name, "debug") {
		t.Fatalf("expected name to contain \"debug\", got %q", name)
	}

	// Deterministic across calls and independent of built-in role ordering.
	if again := managedDebugRoleName(testDebugServer, []string{testPgReadAllData, testPgMonitor}); again != name {
		t.Fatalf("expected deterministic, order-independent name, got %q vs %q", again, name)
	}

	// Distinct per server.
	if other := managedDebugRoleName("other-db", roles); other == name {
		t.Fatalf("expected distinct name for a different server, got %q for both", name)
	}

	// Distinct per built-in role set.
	if other := managedDebugRoleName(testDebugServer, []string{testPgMonitor}); other == name {
		t.Fatalf("expected distinct name for a different built-in role set, got %q for both", name)
	}
}

func TestManagedDebugRoleNameLongServerFits(t *testing.T) {
	long := strings.Repeat("a", 200)
	name := managedDebugRoleName(long, []string{testPgMonitor})
	if len(name) > 63 {
		t.Fatalf("expected name within 63 chars, got len=%d (%q)", len(name), name)
	}
	if !strings.HasPrefix(name, "dispg-") {
		t.Fatalf("expected dispg- prefix, got %q", name)
	}
}

func TestValidateDebugAccessPrincipal(t *testing.T) {
	cases := []struct {
		name      string
		principal AccessPrincipal
		useAAD    bool
		wantErr   bool
	}{
		{
			name:      "valid group without AAD",
			principal: AccessPrincipal{Name: "grp", PrincipalType: PrincipalTypeGroup},
			useAAD:    false,
			wantErr:   false,
		},
		{
			name:      "valid service with AAD",
			principal: AccessPrincipal{Name: testDebugPrincipal, PrincipalID: "oid", PrincipalType: PrincipalTypeService},
			useAAD:    true,
			wantErr:   false,
		},
		{
			name:      "role field ignored",
			principal: AccessPrincipal{Name: testDebugPrincipal, PrincipalType: PrincipalTypeService, Role: "NotARole"},
			useAAD:    false,
			wantErr:   false,
		},
		{
			name:      "missing name",
			principal: AccessPrincipal{PrincipalType: PrincipalTypeService},
			useAAD:    false,
			wantErr:   true,
		},
		{
			name:      "missing principalId with AAD",
			principal: AccessPrincipal{Name: testDebugPrincipal, PrincipalType: PrincipalTypeService},
			useAAD:    true,
			wantErr:   true,
		},
		{
			name:      "bad principal type",
			principal: AccessPrincipal{Name: testDebugPrincipal, PrincipalType: PrincipalType("user")},
			useAAD:    false,
			wantErr:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDebugAccessPrincipal(tc.principal, tc.useAAD)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestEnsureServerDebugAccessGrantsBuiltinsConnectAndMembership(t *testing.T) {
	conn := &recordingConn{databases: []string{maintenanceDatabase, "app1", "app2"}}
	principalConn := &recordingConn{}

	builtin := []string{testPgMonitor, testPgReadAllData}
	if err := ensureServerDebugAccess(context.Background(), conn, principalConn, serverDebugOptions{
		ServerName:   testDebugServer,
		BuiltinRoles: builtin,
		Principals: []AccessPrincipal{
			{Name: "debug-group", PrincipalID: "group-oid", PrincipalType: PrincipalTypeGroup},
		},
		UseAAD: true,
	}); err != nil {
		t.Fatalf("ensureServerDebugAccess: %v", err)
	}

	debugRole := managedDebugRoleName(testDebugServer, builtin)

	// Managed NOLOGIN debug role is created.
	requireExec(t, conn, createNoLoginRoleSQL(debugRole))

	// Built-in roles are granted to the managed role.
	requireExec(t, conn, grantRoleSQL(testPgMonitor, debugRole))
	requireExec(t, conn, grantRoleSQL(testPgReadAllData, debugRole))

	// CONNECT is granted on every enumerated database.
	requireExec(t, conn, grantConnectSQL(maintenanceDatabase, debugRole))
	requireExec(t, conn, grantConnectSQL("app1", debugRole))
	requireExec(t, conn, grantConnectSQL("app2", debugRole))

	// The principal is created against the principal connection (AAD) and made a
	// member of the managed role.
	requireExec(t, principalConn, createAADPrincipalSQL(), "debug-group", "group-oid", "group")
	requireExec(t, conn, grantRoleSQL(debugRole, "debug-group"))
}

func TestEnsureServerDebugAccessRevokesRemovedMembers(t *testing.T) {
	builtin := []string{testPgMonitor, testPgReadAllData}
	debugRole := managedDebugRoleName(testDebugServer, builtin)

	conn := &recordingConn{
		databases: []string{maintenanceDatabase},
		members: map[string][]string{
			debugRole: {"stale-principal", "current-principal"},
		},
	}

	if err := ensureServerDebugAccess(context.Background(), conn, &recordingConn{}, serverDebugOptions{
		ServerName:   testDebugServer,
		BuiltinRoles: builtin,
		Principals: []AccessPrincipal{
			{Name: "current-principal", PrincipalType: PrincipalTypeService},
		},
		UseAAD: false,
	}); err != nil {
		t.Fatalf("ensureServerDebugAccess: %v", err)
	}

	// The principal no longer requested is revoked; the current one is (re)granted.
	requireExec(t, conn, revokeRoleSQL(debugRole, "stale-principal"))
	requireExec(t, conn, grantRoleSQL(debugRole, "current-principal"))
	requireNoExec(t, conn, revokeRoleSQL(debugRole, "current-principal"))
}

func TestEnsureServerDebugAccessRequiresBuiltinRoles(t *testing.T) {
	err := ensureServerDebugAccess(context.Background(), &recordingConn{}, &recordingConn{}, serverDebugOptions{
		ServerName:   testDebugServer,
		BuiltinRoles: nil,
		Principals:   []AccessPrincipal{{Name: testDebugPrincipal, PrincipalType: PrincipalTypeService}},
		UseAAD:       false,
	})
	if err == nil {
		t.Fatalf("expected error when no built-in roles are provided")
	}
}
