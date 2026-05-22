package database

import (
	"context"
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
			name: "create Entra principal",
			got:  createAADPrincipalSQL(),
			want: "SELECT * FROM pgaadauth_create_principal_with_oid($1, $2, $3, false, false)",
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
			name: "grant schema access",
			got:  grantSchemaAccessSQL(schema, "owner-group"),
			want: `GRANT USAGE, CREATE ON SCHEMA "app-db" TO "owner-group";`,
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

func TestEnsureUserCreatesRoleWhenMissing(t *testing.T) {
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
	mock.ExpectExec(`GRANT CONNECT ON DATABASE "app-db" TO "app-user";`).
		WillReturnResult(pgxmock.NewResult("GRANT", 1))
	mock.ExpectExec(`CREATE SCHEMA IF NOT EXISTS "app-db" AUTHORIZATION "app-user";`).
		WillReturnResult(pgxmock.NewResult("CREATE", 1))
	mock.ExpectExec(`ALTER SCHEMA "app-db" OWNER TO "app-user";`).
		WillReturnResult(pgxmock.NewResult("ALTER", 1))
	mock.ExpectExec(`ALTER ROLE "app-user" SET search_path = "app-db", public;`).
		WillReturnResult(pgxmock.NewResult("ALTER", 1))

	if err := ensureUser(context.Background(), mock, "app-user", "", "app-db", "app-db", false); err != nil {
		t.Fatalf("ensureUser: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestEnsureUserCreatesAADRoleWhenMissing(t *testing.T) {
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
	mock.ExpectExec(`GRANT CONNECT ON DATABASE "app-db" TO "app-user";`).
		WillReturnResult(pgxmock.NewResult("GRANT", 1))
	mock.ExpectExec(`CREATE SCHEMA IF NOT EXISTS "app-db" AUTHORIZATION "app-user";`).
		WillReturnResult(pgxmock.NewResult("CREATE", 1))
	mock.ExpectExec(`ALTER SCHEMA "app-db" OWNER TO "app-user";`).
		WillReturnResult(pgxmock.NewResult("ALTER", 1))
	mock.ExpectExec(`ALTER ROLE "app-user" SET search_path = "app-db", public;`).
		WillReturnResult(pgxmock.NewResult("ALTER", 1))

	if err := ensureUser(context.Background(), mock, "app-user", "principal-id", "app-db", "app-db", true); err != nil {
		t.Fatalf("ensureUser: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestEnsureUserSkipsCreateWhenExists(t *testing.T) {
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

	mock.ExpectExec(`GRANT CONNECT ON DATABASE "app-db" TO "app-user";`).
		WillReturnResult(pgxmock.NewResult("GRANT", 1))
	mock.ExpectExec(`CREATE SCHEMA IF NOT EXISTS "app-db" AUTHORIZATION "app-user";`).
		WillReturnResult(pgxmock.NewResult("CREATE", 1))
	mock.ExpectExec(`ALTER SCHEMA "app-db" OWNER TO "app-user";`).
		WillReturnResult(pgxmock.NewResult("ALTER", 1))
	mock.ExpectExec(`ALTER ROLE "app-user" SET search_path = "app-db", public;`).
		WillReturnResult(pgxmock.NewResult("ALTER", 1))

	if err := ensureUser(context.Background(), mock, "app-user", "", "app-db", "app-db", false); err != nil {
		t.Fatalf("ensureUser: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestEnsureAccessCreatesDatabaseAppAndOwnerAccess(t *testing.T) {
	conn := &recordingConn{}

	if err := ensureAccess(context.Background(), conn, accessOptions{
		DatabaseName:             "app-db",
		SchemaName:               "app-db",
		AppName:                  "app-user",
		AppPrincipalID:           "app-principal-id",
		OwnerName:                "owner-group",
		OwnerPrincipalID:         "owner-principal-id",
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

	requireExec(t, conn, createAADPrincipalSQL(), "app-user", "app-principal-id", "service")
	requireExec(t, conn, createAADPrincipalSQL(), "owner-group", "owner-principal-id", "group")
	requireExec(t, conn, grantConnectSQL("app-db", "app-user"))
	requireExec(t, conn, grantConnectSQL("app-db", "owner-group"))
	requireExec(t, conn, alterSchemaOwnerSQL("app-db", "app-user"))
	requireExec(t, conn, grantSchemaAccessSQL("app-db", "owner-group"))
	requireExec(t, conn, alterDefaultTablePrivilegesSQL("app-user", "app-db", "owner-group"))
	requireExec(t, conn, setSearchPathSQL("app-user", "app-db", "app-db", true))
	requireExec(t, conn, setSearchPathSQL("owner-group", "app-db", "app-db", true))
}

type recordingConn struct {
	execs []execCall
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

func (c *recordingConn) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return recordingRow{}
}

type recordingRow struct{}

func (recordingRow) Scan(dest ...any) error {
	exists := dest[0].(*bool)
	*exists = false

	return nil
}

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
