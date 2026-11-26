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

	hash := debugAccessHash(db, "fs-uid-1", admin, principals, builtin, nil)
	if hash != debugAccessHash(db, "fs-uid-1", admin, principals, builtin, nil) {
		t.Fatalf("expected deterministic debug access hash")
	}
	if len(hash) != 64 {
		t.Fatalf("expected a full SHA-256 hex hash, got %q", hash)
	}

	name := debugAccessProvisionJobName(db, hash)
	if len(name) == 0 || len(name) > 63 {
		t.Fatalf("expected non-empty name within 63 chars, got %q (len=%d)", name, len(name))
	}
	if name[0] < 'a' || name[0] > 'z' {
		t.Fatalf("expected name to start with a lowercase letter, got %q", name)
	}
	if !strings.Contains(name, "debug-provision") || !strings.HasSuffix(name, "-"+hash[:8]) {
		t.Fatalf("expected name with \"debug-provision\" and the hash suffix, got %q", name)
	}
}

func TestDebugAccessHashChangesWithPrincipalSet(t *testing.T) {
	db := testDebugJobServer()
	admin := testDebugAdminIdentity()
	builtin := []string{debugBuiltinRolePgMonitor, debugBuiltinRolePgReadAllData}

	base := debugAccessHash(db, "fs-uid-1", admin, testDebugPrincipals(), builtin, nil)

	added := debugAccessHash(db, "fs-uid-1", admin, append(testDebugPrincipals(), dbUtil.AccessPrincipal{
		Name:          "another",
		PrincipalID:   "22222222-2222-2222-2222-222222222222",
		PrincipalType: dbUtil.PrincipalTypeService,
	}), builtin, nil)

	if base == added {
		t.Fatalf("expected hash to change when a principal is added, got %q for both", base)
	}
}

func TestDebugAccessHashChangesWithTargets(t *testing.T) {
	db := testDebugJobServer()
	admin := testDebugAdminIdentity()
	principals := testDebugPrincipals()
	builtin := []string{debugBuiltinRolePgMonitor, debugBuiltinRolePgReadAllData}

	base := debugAccessHash(db, "fs-uid-1", admin, principals, builtin, nil)
	withDB := debugAccessHash(db, "fs-uid-1", admin, principals, builtin, []string{"router/uid-1"})
	recreatedDB := debugAccessHash(db, "fs-uid-1", admin, principals, builtin, []string{"router/uid-2"})
	withMoreDBs := debugAccessHash(db, "fs-uid-1", admin, principals, builtin, []string{"billing/uid-3", "router/uid-2"})

	if base == withDB || withDB == recreatedDB || recreatedDB == withMoreDBs {
		t.Fatalf("expected hash to change when the database list changes, got %q / %q / %q / %q", base, withDB, recreatedDB, withMoreDBs)
	}

	recreatedServer := debugAccessHash(db, "fs-uid-2", admin, principals, builtin, nil)
	if base == recreatedServer {
		t.Fatalf("expected hash to change when the FlexibleServer is created again, got %q", base)
	}
}

func TestDebugAccessProvisionJobNameLongServerFits(t *testing.T) {
	db := testDebugJobServer()
	db.Name = strings.Repeat("a", 200)
	hash := debugAccessHash(db, "fs-uid-1", testDebugAdminIdentity(), testDebugPrincipals(), []string{debugBuiltinRolePgMonitor}, nil)
	name := debugAccessProvisionJobName(db, hash)
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

	// Debug mode allows zero principals: the revocation Job runs with an empty
	// set so the membership reconcile revokes everyone.
	revocation := valid
	revocation.AccessPrincipals = nil
	if err := validateUserProvisionJobSpec(revocation, false); err != nil {
		t.Fatalf("expected zero-principal server-debug spec to be valid, got %v", err)
	}

	// Non-debug provisioning still requires at least one principal.
	nonDebug := valid
	nonDebug.ServerDebugAccess = false
	nonDebug.DebugBuiltinRoles = nil
	nonDebug.SchemaName = "app"
	nonDebug.AccessPrincipals = nil
	if err := validateUserProvisionJobSpec(nonDebug, false); err == nil {
		t.Fatalf("expected error for zero principals outside debug mode")
	}
}

func envToMap(env []corev1.EnvVar) map[string]string {
	out := map[string]string{}
	for _, e := range env {
		out[e.Name] = e.Value
	}
	return out
}
