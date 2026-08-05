package controller

import (
	"testing"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
)

func TestDatabaseServerReferencesIdentity(t *testing.T) {
	adminServer := &storagev1alpha1.DatabaseServer{
		Spec: storagev1alpha1.DatabaseServerSpec{
			Auth: storagev1alpha1.DatabaseServerAuth{
				Admin: storagev1alpha1.AdminIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: "admin-id"},
					},
				},
			},
		},
	}

	debugServer := &storagev1alpha1.DatabaseServer{
		Spec: storagev1alpha1.DatabaseServerSpec{
			DebugAccess: &storagev1alpha1.DatabaseServerDebugAccessSpec{
				Principals: []storagev1alpha1.DebugAccessPrincipalSpec{
					{IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: "debug-id"}},
					{Group: &storagev1alpha1.DatabaseGroupPrincipalSpec{Name: "grp", PrincipalId: "obj"}},
				},
			},
		},
	}

	tests := []struct {
		name     string
		db       *storagev1alpha1.DatabaseServer
		identity string
		want     bool
	}{
		{"admin identityRef match", adminServer, "admin-id", true},
		{"admin identityRef no match", adminServer, "other", false},
		{"debugAccess identityRef match", debugServer, "debug-id", true},
		{"debugAccess identityRef no match", debugServer, "other", false},
		{"no identity refs at all", &storagev1alpha1.DatabaseServer{}, "anything", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := databaseServerReferencesIdentity(tt.db, tt.identity); got != tt.want {
				t.Fatalf("databaseServerReferencesIdentity(%q) = %v, want %v", tt.identity, got, tt.want)
			}
		})
	}
}
