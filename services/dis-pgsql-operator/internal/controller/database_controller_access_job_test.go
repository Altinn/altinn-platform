package controller

import (
	"testing"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
)

func TestDatabaseAccessHashChangesWithEachInput(t *testing.T) {
	type inputs struct {
		host       string
		uids       databaseAccessTargetUIDs
		admin      resolvedAdminIdentity
		principals []dbUtil.AccessPrincipal
	}
	base := func() inputs {
		return inputs{
			host: "server-one.postgres.database.azure.com",
			uids: databaseAccessTargetUIDs{
				DatabaseServer: "server-uid",
				FlexibleServer: "flexible-server-uid",
				Database:       "database-uid",
			},
			admin: testDebugAdminIdentity(),
			principals: []dbUtil.AccessPrincipal{{
				Role:          dbUtil.AccessRoleWriter,
				Name:          "app-one-mi",
				PrincipalID:   "00000000-0000-0000-0000-000000000001",
				PrincipalType: dbUtil.PrincipalTypeService,
			}},
		}
	}
	hash := func(in inputs) string {
		database := &storagev1alpha1.Database{}
		database.Status.DatabaseName = "app-one"
		database.Status.Host = in.host
		return databaseAccessHash(database, "server-one", in.uids, in.admin, in.principals)
	}

	want := hash(base())
	if again := hash(base()); again != want {
		t.Fatalf("expected the same hash for the same inputs, got %q and %q", want, again)
	}

	changes := map[string]func(*inputs){
		"DatabaseServer UID": func(in *inputs) { in.uids.DatabaseServer = "server-uid-2" },
		"FlexibleServer UID": func(in *inputs) { in.uids.FlexibleServer = "flexible-server-uid-2" },
		"database UID":       func(in *inputs) { in.uids.Database = "database-uid-2" },
		"host":               func(in *inputs) { in.host = "server-two.postgres.database.azure.com" },
		"admin identity":     func(in *inputs) { in.admin.Name = "admin-mi-2" },
		"admin account":      func(in *inputs) { in.admin.ServiceAccountName = "admin-sa-2" },
		"principal role":     func(in *inputs) { in.principals[0].Role = dbUtil.AccessRoleReader },
		"principal id":       func(in *inputs) { in.principals[0].PrincipalID = "00000000-0000-0000-0000-000000000002" },
	}
	for name, change := range changes {
		in := base()
		change(&in)
		if got := hash(in); got == want {
			t.Errorf("expected a new hash when the %s changes", name)
		}
	}
}
