package controller

import (
	"testing"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	"github.com/google/uuid"
)

const (
	testDbgServerName  = "my-app-db"
	testDbgNamespace   = "default"
	testDbgPrincipalID = "11111111-1111-1111-1111-111111111111"
)

func newDebugAccessDatabaseServer() *storagev1alpha1.DatabaseServer {
	db := &storagev1alpha1.DatabaseServer{}
	db.Name = testDbgServerName
	db.Namespace = testDbgNamespace
	return db
}

func TestBuildDebugAccessRoleAssignmentReaderRole(t *testing.T) {
	t.Parallel()

	db := newDebugAccessDatabaseServer()
	ra, err := buildDebugAccessRoleAssignment(db, testDbgPrincipalID, authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal)
	if err != nil {
		t.Fatalf("expected builder to succeed, got error: %v", err)
	}
	if ra == nil {
		t.Fatalf("expected builder to return a role assignment")
	}

	if ra.Spec.RoleDefinitionReference == nil ||
		ra.Spec.RoleDefinitionReference.WellKnownName != debugAccessReaderRole {
		t.Fatalf("expected RoleDefinitionReference.WellKnownName=%q, got %#v", debugAccessReaderRole, ra.Spec.RoleDefinitionReference)
	}
	if debugAccessReaderRole != "Reader" {
		t.Fatalf("expected debug access role to be the built-in Azure Reader role, got %q", debugAccessReaderRole)
	}
	if ra.Spec.PrincipalId == nil || *ra.Spec.PrincipalId != testDbgPrincipalID {
		t.Fatalf("expected PrincipalId=%q, got %#v", testDbgPrincipalID, ra.Spec.PrincipalId)
	}
	if _, err := uuid.Parse(ra.Spec.AzureName); err != nil {
		t.Fatalf("expected AzureName to be a GUID, got %q: %v", ra.Spec.AzureName, err)
	}
	if ra.Namespace != testDbgNamespace {
		t.Fatalf("expected namespace %q, got %q", testDbgNamespace, ra.Namespace)
	}
	if ra.Labels[databaseServerNameLabelKey] != testDbgServerName {
		t.Fatalf("expected owner label %q=%q", databaseServerNameLabelKey, testDbgServerName)
	}
	if ra.Labels[debugAccessComponentLabelKey] != debugAccessComponentLabelValue {
		t.Fatalf("expected component label %q=%q", debugAccessComponentLabelKey, debugAccessComponentLabelValue)
	}
}

func TestBuildDebugAccessRoleAssignmentOwnerIsFlexibleServer(t *testing.T) {
	t.Parallel()

	db := newDebugAccessDatabaseServer()
	ra, err := buildDebugAccessRoleAssignment(db, testDbgPrincipalID, authorizationv1.RoleAssignmentProperties_PrincipalType_Group)
	if err != nil {
		t.Fatalf("expected builder to succeed, got error: %v", err)
	}

	if ra.Spec.Owner == nil {
		t.Fatalf("expected an ArbitraryOwnerReference owner")
	}
	if ra.Spec.Owner.Kind != "FlexibleServer" {
		t.Fatalf("expected owner Kind=FlexibleServer, got %q", ra.Spec.Owner.Kind)
	}
	if ra.Spec.Owner.Group != dbforpostgresqlv1.GroupVersion.Group {
		t.Fatalf("expected owner Group=%q, got %q", dbforpostgresqlv1.GroupVersion.Group, ra.Spec.Owner.Group)
	}
	// The FlexibleServer Kubernetes object name is the DatabaseServer name.
	if ra.Spec.Owner.Name != db.Name {
		t.Fatalf("expected owner Name=%q (FlexibleServer k8s name), got %q", db.Name, ra.Spec.Owner.Name)
	}
}

func TestBuildDebugAccessRoleAssignmentPrincipalType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   authorizationv1.RoleAssignmentProperties_PrincipalType
	}{
		{"group", authorizationv1.RoleAssignmentProperties_PrincipalType_Group},
		{"servicePrincipal", authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := newDebugAccessDatabaseServer()
			ra, err := buildDebugAccessRoleAssignment(db, testDbgPrincipalID, tc.in)
			if err != nil {
				t.Fatalf("expected builder to succeed, got error: %v", err)
			}
			if ra.Spec.PrincipalType == nil || *ra.Spec.PrincipalType != tc.in {
				t.Fatalf("expected PrincipalType=%q, got %#v", tc.in, ra.Spec.PrincipalType)
			}
		})
	}
}

func TestBuildDebugAccessRoleAssignmentDeterministic(t *testing.T) {
	t.Parallel()

	db := newDebugAccessDatabaseServer()
	first, err := buildDebugAccessRoleAssignment(db, testDbgPrincipalID, authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal)
	if err != nil {
		t.Fatalf("expected first build to succeed, got error: %v", err)
	}
	second, err := buildDebugAccessRoleAssignment(db, testDbgPrincipalID, authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal)
	if err != nil {
		t.Fatalf("expected second build to succeed, got error: %v", err)
	}

	if first.Name != second.Name {
		t.Fatalf("expected deterministic Kubernetes name across builds, got %q vs %q", first.Name, second.Name)
	}
	if first.Spec.AzureName != second.Spec.AzureName {
		t.Fatalf("expected deterministic AzureName across builds, got %q vs %q", first.Spec.AzureName, second.Spec.AzureName)
	}
}

func TestBuildDebugAccessRoleAssignmentUniquePerPrincipal(t *testing.T) {
	t.Parallel()

	db := newDebugAccessDatabaseServer()
	otherPrincipalID := "22222222-2222-2222-2222-222222222222"

	first, err := buildDebugAccessRoleAssignment(db, testDbgPrincipalID, authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal)
	if err != nil {
		t.Fatalf("expected first build to succeed, got error: %v", err)
	}
	second, err := buildDebugAccessRoleAssignment(db, otherPrincipalID, authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal)
	if err != nil {
		t.Fatalf("expected second build to succeed, got error: %v", err)
	}

	if first.Name == second.Name {
		t.Fatalf("expected distinct Kubernetes names for different principals, got %q for both", first.Name)
	}
	if first.Spec.AzureName == second.Spec.AzureName {
		t.Fatalf("expected distinct AzureNames for different principals, got %q for both", first.Spec.AzureName)
	}
}

func TestBuildDebugAccessRoleAssignmentRejectsEmptyPrincipal(t *testing.T) {
	t.Parallel()

	db := newDebugAccessDatabaseServer()
	if _, err := buildDebugAccessRoleAssignment(db, "  ", authorizationv1.RoleAssignmentProperties_PrincipalType_Group); err == nil {
		t.Fatalf("expected error when principalID is empty")
	}
}

func TestBuildDebugAccessRoleAssignmentRejectsNilServer(t *testing.T) {
	t.Parallel()

	if _, err := buildDebugAccessRoleAssignment(nil, testDbgPrincipalID, authorizationv1.RoleAssignmentProperties_PrincipalType_Group); err == nil {
		t.Fatalf("expected error when databaseServer is nil")
	}
}

func TestDebugAccessRoleAssignmentNameIsDNSCompatible(t *testing.T) {
	t.Parallel()

	name := debugAccessRoleAssignmentName(testDbgServerName, testDbgPrincipalID)
	if len(name) == 0 || len(name) > roleAssignmentMaxNameLen {
		t.Fatalf("expected a non-empty name within %d chars, got %q (len=%d)", roleAssignmentMaxNameLen, name, len(name))
	}
	if name[0] < 'a' || name[0] > 'z' {
		t.Fatalf("expected name to start with a lowercase letter, got %q", name)
	}
}
