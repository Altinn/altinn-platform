package database

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestUserProvisionSQL(t *testing.T) {
	user := "app-user"
	dbName := "app-db"
	schema := "app-db"

	cases := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "role exists query",
			got:  roleExistsSQL(),
			want: "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)",
		},
		{
			name: "create role",
			got:  createRoleSQL(user),
			want: `CREATE ROLE "app-user" LOGIN;`,
		},
		{
			name: "create no-login role",
			got:  createNoLoginRoleSQL("managed-role"),
			want: `CREATE ROLE "managed-role" NOLOGIN;`,
		},
		{
			name: "create Entra principal",
			got:  createAADPrincipalSQL(),
			want: "SELECT * FROM pgaadauth_create_principal_with_oid($1, $2, $3, false, false)",
		},
		{
			name: "grant role membership",
			got:  grantRoleSQL("managed-role", user),
			want: `GRANT "managed-role" TO "app-user";`,
		},
		{
			name: "revoke role membership",
			got:  revokeRoleSQL("managed-role", user),
			want: `REVOKE "managed-role" FROM "app-user";`,
		},
		{
			name: "grant connect",
			got:  grantConnectSQL(dbName, user),
			want: `GRANT CONNECT ON DATABASE "app-db" TO "app-user";`,
		},
		{
			name: "create schema",
			got:  createSchemaSQL(schema, user),
			want: `CREATE SCHEMA IF NOT EXISTS "app-db" AUTHORIZATION "app-user";`,
		},
		{
			name: "alter schema owner",
			got:  alterSchemaOwnerSQL(schema, user),
			want: `ALTER SCHEMA "app-db" OWNER TO "app-user";`,
		},
		{
			name: "set search path",
			got:  setSearchPathSQL(user, dbName, schema, false),
			want: `ALTER ROLE "app-user" SET search_path = "app-db", public;`,
		},
		{
			name: "set database scoped search path",
			got:  setSearchPathSQL(user, dbName, schema, true),
			want: `ALTER ROLE "app-user" IN DATABASE "app-db" SET search_path = "app-db", public;`,
		},
		{
			name: "revoke public connect",
			got:  revokePublicConnectSQL(dbName),
			want: `REVOKE CONNECT ON DATABASE "app-db" FROM PUBLIC;`,
		},
		{
			name: "grant schema usage",
			got:  grantSchemaUsageSQL(schema, "reader-role"),
			want: `GRANT USAGE ON SCHEMA "app-db" TO "reader-role";`,
		},
		{
			name: "grant table read",
			got:  grantAllTablesReadSQL(schema, "reader-role"),
			want: `GRANT SELECT ON ALL TABLES IN SCHEMA "app-db" TO "reader-role";`,
		},
		{
			name: "grant table write",
			got:  grantAllTablesWriteSQL(schema, "writer-role"),
			want: `GRANT INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA "app-db" TO "writer-role";`,
		},
		{
			name: "grant default table read",
			got:  alterDefaultTableReadPrivilegesSQL("owner-role", schema, "reader-role"),
			want: `ALTER DEFAULT PRIVILEGES FOR ROLE "owner-role" IN SCHEMA "app-db" GRANT SELECT ON TABLES TO "reader-role";`,
		},
		{
			name: "grant default sequence write",
			got:  alterDefaultSequenceWritePrivilegesSQL("owner-role", schema, "writer-role"),
			want: `ALTER DEFAULT PRIVILEGES FOR ROLE "owner-role" IN SCHEMA "app-db" GRANT USAGE, UPDATE ON SEQUENCES TO "writer-role";`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestUserProvisionSQLQuoting(t *testing.T) {
	user := `weird"name`
	dbName := `db-name`
	schema := `schema-name`

	if got, want := createRoleSQL(user), `CREATE ROLE "weird""name" LOGIN;`; got != want {
		t.Fatalf("createRoleSQL got %q, want %q", got, want)
	}
	if got, want := grantConnectSQL(dbName, user), `GRANT CONNECT ON DATABASE "db-name" TO "weird""name";`; got != want {
		t.Fatalf("grantConnectSQL got %q, want %q", got, want)
	}
	if got, want := createSchemaSQL(schema, user), `CREATE SCHEMA IF NOT EXISTS "schema-name" AUTHORIZATION "weird""name";`; got != want {
		t.Fatalf("createSchemaSQL got %q, want %q", got, want)
	}
	if got, want := alterSchemaOwnerSQL(schema, user), `ALTER SCHEMA "schema-name" OWNER TO "weird""name";`; got != want {
		t.Fatalf("alterSchemaOwnerSQL got %q, want %q", got, want)
	}
	if got, want := setSearchPathSQL(user, dbName, schema, false), `ALTER ROLE "weird""name" SET search_path = "schema-name", public;`; got != want {
		t.Fatalf("setSearchPathSQL got %q, want %q", got, want)
	}
	if got, want := setSearchPathSQL(user, dbName, schema, true), `ALTER ROLE "weird""name" IN DATABASE "db-name" SET search_path = "schema-name", public;`; got != want {
		t.Fatalf("setSearchPathSQL scoped got %q, want %q", got, want)
	}
}

func TestEnsurePrincipalCreatesRoleWhenMissing(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("new mock: %v", err)
	}
	defer func() {
		_ = mock.Close(context.Background())
	}()

	mock.ExpectQuery("SELECT EXISTS \\(SELECT 1 FROM pg_roles WHERE rolname = \\$1\\)").
		WithArgs("app-user").
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

	mock.ExpectExec(`CREATE ROLE "app-user" LOGIN;`).
		WillReturnResult(pgxmock.NewResult("CREATE", 1))

	if err := ensurePrincipal(context.Background(), mock, "app-user", "", "service", false); err != nil {
		t.Fatalf("ensurePrincipal: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestEnsurePrincipalCreatesAADRoleWhenMissing(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("new mock: %v", err)
	}
	defer func() {
		_ = mock.Close(context.Background())
	}()

	mock.ExpectQuery("SELECT EXISTS \\(SELECT 1 FROM pg_roles WHERE rolname = \\$1\\)").
		WithArgs("app-user").
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

	mock.ExpectExec("SELECT \\* FROM pgaadauth_create_principal_with_oid\\(\\$1, \\$2, \\$3, false, false\\)").
		WithArgs("app-user", "principal-id", "service").
		WillReturnResult(pgxmock.NewResult("SELECT", 1))

	if err := ensurePrincipal(context.Background(), mock, "app-user", "principal-id", "service", true); err != nil {
		t.Fatalf("ensurePrincipal: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestEnsurePrincipalSkipsCreateWhenExists(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("new mock: %v", err)
	}
	defer func() {
		_ = mock.Close(context.Background())
	}()

	mock.ExpectQuery("SELECT EXISTS \\(SELECT 1 FROM pg_roles WHERE rolname = \\$1\\)").
		WithArgs("app-user").
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	if err := ensurePrincipal(context.Background(), mock, "app-user", "", "service", false); err != nil {
		t.Fatalf("ensurePrincipal: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestEnsureAccessCreatesManagedRolesAndPrincipalMemberships(t *testing.T) {
	conn := &recordingConn{}

	if err := ensureAccess(context.Background(), conn, accessOptions{
		DatabaseName: "app-db",
		SchemaName:   "app-db",
		Principals: []AccessPrincipal{
			{
				Role:          AccessRoleWriter,
				Name:          "app-user",
				PrincipalID:   "app-principal-id",
				PrincipalType: PrincipalTypeService,
			},
			{
				Role:          AccessRoleOwner,
				Name:          "owner-group",
				PrincipalID:   "owner-principal-id",
				PrincipalType: PrincipalTypeGroup,
			},
		},
		UseAAD:                   true,
		RevokePublicConnect:      true,
		DatabaseScopedSearchPath: true,
	}); err != nil {
		t.Fatalf("ensureAccess: %v", err)
	}

	if len(conn.execs) == 0 {
		t.Fatal("expected execs")
	}
	if got, want := conn.execs[0].sql, revokePublicConnectSQL("app-db"); got != want {
		t.Fatalf("first exec got %q, want %q", got, want)
	}

	roles := managedAccessRolesFor("app-db", "app-db")
	requireExec(t, conn, createNoLoginRoleSQL(roles.Reader))
	requireExec(t, conn, createNoLoginRoleSQL(roles.Writer))
	requireExec(t, conn, createNoLoginRoleSQL(roles.Owner))
	requireExec(t, conn, createAADPrincipalSQL(), "app-user", "app-principal-id", "service")
	requireExec(t, conn, createAADPrincipalSQL(), "owner-group", "owner-principal-id", "group")
	requireExec(t, conn, grantConnectSQL("app-db", roles.Reader))
	requireExec(t, conn, alterSchemaOwnerSQL("app-db", roles.Owner))
	requireExec(t, conn, grantSchemaUsageSQL("app-db", roles.Reader))
	requireExec(t, conn, grantAllTablesReadSQL("app-db", roles.Reader))
	requireExec(t, conn, grantAllTablesWriteSQL("app-db", roles.Writer))
	requireExec(t, conn, grantRoleSQL(roles.Reader, roles.Writer))
	requireExec(t, conn, grantRoleSQL(roles.Writer, roles.Owner))
	requireExec(t, conn, grantRoleSQL(roles.Writer, "app-user"))
	requireExec(t, conn, grantRoleSQL(roles.Owner, "owner-group"))
	requireExec(t, conn, alterDefaultTableReadPrivilegesSQL("owner-group", "app-db", roles.Reader))
	requireExec(t, conn, alterDefaultTableWritePrivilegesSQL("owner-group", "app-db", roles.Writer))
	requireExec(t, conn, setSearchPathSQL("app-user", "app-db", "app-db", true))
	requireExec(t, conn, setSearchPathSQL("owner-group", "app-db", "app-db", true))
}

func TestEnsureAccessRevokesRemovedManagedRoleMembers(t *testing.T) {
	roles := managedAccessRolesFor("app-db", "app-db")
	conn := &recordingConn{
		members: map[string][]string{
			roles.Reader: {"old-reader", roles.Writer},
			roles.Writer: {roles.Owner},
			roles.Owner:  {"old-owner"},
		},
	}

	if err := ensureAccess(context.Background(), conn, accessOptions{
		DatabaseName: "app-db",
		SchemaName:   "app-db",
		Principals: []AccessPrincipal{
			{
				Role:          AccessRoleReader,
				Name:          "current-reader",
				PrincipalType: PrincipalTypeService,
			},
		},
		UseAAD: false,
	}); err != nil {
		t.Fatalf("ensureAccess: %v", err)
	}

	requireExec(t, conn, revokeRoleSQL(roles.Reader, "old-reader"))
	requireExec(t, conn, revokeRoleSQL(roles.Owner, "old-owner"))
	requireExec(t, conn, grantRoleSQL(roles.Reader, "current-reader"))
}

type recordingConn struct {
	execs   []execCall
	members map[string][]string
}

type execCall struct {
	sql  string
	args []any
}

func (c *recordingConn) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	c.execs = append(c.execs, execCall{
		sql:  sql,
		args: append([]any(nil), args...),
	})

	return pgconn.CommandTag{}, nil
}

func (c *recordingConn) Query(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("recordingConn.Query: expected 1 arg, got %d", len(args))
	}
	roleName, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("recordingConn.Query: expected string arg, got %T", args[0])
	}
	return &recordingRows{values: append([]string(nil), c.members[roleName]...)}, nil
}

func (c *recordingConn) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return recordingRow{}
}

type recordingRow struct{}

func (recordingRow) Scan(dest ...any) error {
	exists := dest[0].(*bool)
	*exists = false

	return nil
}

type recordingRows struct {
	values []string
	index  int
}

func (r *recordingRows) Close() {}

func (r *recordingRows) Err() error { return nil }

func (r *recordingRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *recordingRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *recordingRows) Next() bool {
	if r.index >= len(r.values) {
		return false
	}
	r.index++
	return true
}

func (r *recordingRows) Scan(dest ...any) error {
	value := r.values[r.index-1]
	target := dest[0].(*string)
	*target = value
	return nil
}

func (r *recordingRows) Values() ([]any, error) {
	if r.index == 0 || r.index > len(r.values) {
		return nil, nil
	}
	return []any{r.values[r.index-1]}, nil
}

func (r *recordingRows) RawValues() [][]byte {
	if r.index == 0 || r.index > len(r.values) {
		return nil
	}
	return [][]byte{[]byte(r.values[r.index-1])}
}

func (r *recordingRows) Conn() *pgx.Conn { return nil }

func requireExec(t *testing.T, conn *recordingConn, sql string, args ...any) {
	t.Helper()

	for _, exec := range conn.execs {
		if exec.sql == sql && equalArgs(exec.args, args) {
			return
		}
	}

	t.Fatalf("missing exec %q with args %#v; got %#v", sql, args, conn.execs)
}

func equalArgs(got, want []any) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}
