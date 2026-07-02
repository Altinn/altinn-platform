package controller

import (
	"strings"
	"testing"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	corev1 "k8s.io/api/core/v1"
)

const (
	testDebugJobHost     = "my-app-db-xyz.postgres.database.azure.com"
	testDebugAdminMIName = "admin-mi"
)

func testDebugJobServer() *storagev1alpha1.DatabaseServer {
	db := &storagev1alpha1.DatabaseServer{}
	db.Name = testDbgServerName
	db.Namespace = testDbgNamespace
	db.Status.Host = testDebugJobHost
	return db
}

func testDebugAdminIdentity() resolvedAdminIdentity {
	return resolvedAdminIdentity{
		resolvedIdentity:   resolvedIdentity{Name: testDebugAdminMIName, PrincipalID: "admin-oid"},
		ServiceAccountName: "admin-sa",
	}
}

func testDebugPrincipals() []dbUtil.AccessPrincipal {
	return []dbUtil.AccessPrincipal{
		{Name: "debug-group", PrincipalID: "11111111-1111-1111-1111-111111111111", PrincipalType: dbUtil.PrincipalTypeGroup},
	}
}

func TestDebugAccessProvisionJobNameDeterministicAndBounded(t *testing.T) {
	db := testDebugJobServer()
	admin := testDebugAdminIdentity()
	principals := testDebugPrincipals()
	builtin := []string{debugBuiltinRolePgMonitor, debugBuiltinRolePgReadAllData}

	first := debugAccessProvisionJobName(db, admin, principals, builtin)
	second := debugAccessProvisionJobName(db, admin, principals, builtin)

	if first != second {
		t.Fatalf("expected deterministic job name, got %q vs %q", first, second)
	}
	if len(first) == 0 || len(first) > 63 {
		t.Fatalf("expected non-empty name within 63 chars, got %q (len=%d)", first, len(first))
	}
	if first[0] < 'a' || first[0] > 'z' {
		t.Fatalf("expected name to start with a lowercase letter, got %q", first)
	}
	if !strings.Contains(first, "debug-provision") {
		t.Fatalf("expected name to contain \"debug-provision\", got %q", first)
	}
}

func TestDebugAccessProvisionJobNameChangesWithPrincipalSet(t *testing.T) {
	db := testDebugJobServer()
	admin := testDebugAdminIdentity()
	builtin := []string{debugBuiltinRolePgMonitor, debugBuiltinRolePgReadAllData}

	base := debugAccessProvisionJobName(db, admin, testDebugPrincipals(), builtin)

	added := debugAccessProvisionJobName(db, admin, append(testDebugPrincipals(), dbUtil.AccessPrincipal{
		Name:          "another",
		PrincipalID:   "22222222-2222-2222-2222-222222222222",
		PrincipalType: dbUtil.PrincipalTypeService,
	}), builtin)

	if base == added {
		t.Fatalf("expected job name to change when a principal is added, got %q for both", base)
	}
}

func TestDebugAccessProvisionJobNameLongServerFits(t *testing.T) {
	db := testDebugJobServer()
	db.Name = strings.Repeat("a", 200)
	name := debugAccessProvisionJobName(db, testDebugAdminIdentity(), testDebugPrincipals(), []string{debugBuiltinRolePgMonitor})
	if len(name) > 63 {
		t.Fatalf("expected name within 63 chars, got len=%d (%q)", len(name), name)
	}
}

func TestDebugAccessProvisionJobLabels(t *testing.T) {
	labels := debugAccessProvisionJobLabels(testDbgServerName)
	if labels[databaseServerNameLabelKey] != testDbgServerName {
		t.Fatalf("expected server-name label, got %#v", labels)
	}
	if labels[debugAccessComponentLabelKey] != debugAccessComponentLabelValue {
		t.Fatalf("expected debug-access component label, got %#v", labels)
	}
	if labels[databaseAccessProvisionLabelKey] != labelValueTrue {
		t.Fatalf("expected access-provision label, got %#v", labels)
	}
}

func TestDebugAccessPrincipalTypeToPayload(t *testing.T) {
	if got := debugAccessPrincipalTypeToPayload(authorizationv1.RoleAssignmentProperties_PrincipalType_Group); got != dbUtil.PrincipalTypeGroup {
		t.Fatalf("group should map to group, got %q", got)
	}
	if got := debugAccessPrincipalTypeToPayload(authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal); got != dbUtil.PrincipalTypeService {
		t.Fatalf("service principal should map to service, got %q", got)
	}
	if got := debugAccessPrincipalTypeToPayload(authorizationv1.RoleAssignmentProperties_PrincipalType_User); got != dbUtil.PrincipalTypeService {
		t.Fatalf("user should default to service, got %q", got)
	}
}

func TestUserProvisionJobEnvServerDebugMode(t *testing.T) {
	env := userProvisionJobEnv(userProvisionJobSpec{
		AdminIdentityName: testDebugAdminMIName,
		ServerName:        testDbgServerName,
		DatabaseHost:      testDebugJobHost,
		DatabaseName:      "postgres",
		AccessPrincipals:  testDebugPrincipals(),
		ServerDebugAccess: true,
		DebugBuiltinRoles: []string{debugBuiltinRolePgMonitor, debugBuiltinRolePgReadAllData},
	})

	values := envToMap(env)
	if values[dbUtil.ServerDebugAccessEnv] != "1" {
		t.Fatalf("expected %s=1, got %q", dbUtil.ServerDebugAccessEnv, values[dbUtil.ServerDebugAccessEnv])
	}
	if values[dbUtil.DebugBuiltinRolesEnv] != debugBuiltinRolePgMonitor+","+debugBuiltinRolePgReadAllData {
		t.Fatalf("expected built-in roles env, got %q", values[dbUtil.DebugBuiltinRolesEnv])
	}
	if values[dbUtil.DBNameEnv] != "postgres" {
		t.Fatalf("expected maintenance DB name env, got %q", values[dbUtil.DBNameEnv])
	}
	if values[dbUtil.DBHostEnv] != testDebugJobHost {
		t.Fatalf("expected host env, got %q", values[dbUtil.DBHostEnv])
	}
	if _, ok := values[dbUtil.DBSearchPathScopeEnv]; ok {
		t.Fatalf("did not expect search-path-scope env in debug mode, got %q", values[dbUtil.DBSearchPathScopeEnv])
	}
}

func TestUserProvisionJobEnvNonDebugOmitsDebugVars(t *testing.T) {
	env := userProvisionJobEnv(userProvisionJobSpec{
		AdminIdentityName: testDebugAdminMIName,
		ServerName:        testDbgServerName,
		SchemaName:        "app",
		AccessPrincipals: []dbUtil.AccessPrincipal{
			{Role: dbUtil.AccessRoleReader, Name: "svc", PrincipalID: "oid", PrincipalType: dbUtil.PrincipalTypeService},
		},
	})

	values := envToMap(env)
	if _, ok := values[dbUtil.ServerDebugAccessEnv]; ok {
		t.Fatalf("did not expect debug env in non-debug mode")
	}
	if _, ok := values[dbUtil.DebugBuiltinRolesEnv]; ok {
		t.Fatalf("did not expect built-in roles env in non-debug mode")
	}
}

func TestValidateUserProvisionJobSpecServerDebug(t *testing.T) {
	valid := userProvisionJobSpec{
		ServiceAccountName: "admin-sa",
		AdminIdentityName:  testDebugAdminMIName,
		ServerName:         testDbgServerName,
		AccessPrincipals:   testDebugPrincipals(),
		ServerDebugAccess:  true,
		DebugBuiltinRoles:  []string{debugBuiltinRolePgMonitor},
	}
	if err := validateUserProvisionJobSpec(valid, false); err != nil {
		t.Fatalf("expected valid server-debug spec, got %v", err)
	}

	// Debug mode does not require SchemaName.
	if valid.SchemaName != "" {
		t.Fatalf("test precondition: SchemaName should be empty")
	}

	// Debug mode requires at least one built-in role.
	noBuiltin := valid
	noBuiltin.DebugBuiltinRoles = nil
	if err := validateUserProvisionJobSpec(noBuiltin, false); err == nil {
		t.Fatalf("expected error when no built-in roles set in debug mode")
	}

	// Debug mode ignores the per-principal Role field (unset is fine).
	if valid.AccessPrincipals[0].Role != "" {
		t.Fatalf("test precondition: debug principal Role should be empty")
	}
}

func envToMap(env []corev1.EnvVar) map[string]string {
	out := map[string]string{}
	for _, e := range env {
		out[e.Name] = e.Value
	}
	return out
}
