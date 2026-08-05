package connection

import (
	"fmt"
	"strings"
	"testing"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	testDB       = "appdb"
	testIdentity = "payments-api"
	testHost     = "appdb-srv.postgres.database.azure.com"
)

func TestDeterministicConfigMapName(t *testing.T) {
	t.Parallel()

	got := DeterministicConfigMapName("router", "myproduct-router-dev")
	want := "router-myproduct-router-dev-dis-pgsql"
	if got != want {
		t.Fatalf("DeterministicConfigMapName() = %q, want %q", got, want)
	}
}

func TestDeterministicConfigMapNameOverflow(t *testing.T) {
	t.Parallel()

	db := strings.Repeat("very-long-database-name-", 3)
	ref := strings.Repeat("very-long-identity-name-", 3)

	got := DeterministicConfigMapName(db, ref)
	got2 := DeterministicConfigMapName(db, ref)

	if got != got2 {
		t.Fatalf("expected deterministic output, got %q and %q", got, got2)
	}
	if len(got) > configMapMaxLen {
		t.Fatalf("expected name length <= %d, got %d for %q", configMapMaxLen, len(got), got)
	}
	if !strings.Contains(got, "-"+configMapSuffix+"-") {
		t.Fatalf("expected hashed dis-pgsql suffix, got %q", got)
	}
	if errs := validation.IsDNS1123Subdomain(got); len(errs) > 0 {
		t.Fatalf("expected DNS-1123 compliant name, got %q: %v", got, errs)
	}
}

func TestDeterministicConfigMapNameDistinct(t *testing.T) {
	t.Parallel()

	// Distinct (database, identity) pairs must not collide, including in the
	// hashed overflow case.
	long := strings.Repeat("x", 60)
	pairs := [][2]string{
		{testDB, testIdentity},
		{testDB, "analytics"},
		{"other", testIdentity},
		{long, testIdentity},
		{long, "analytics"},
	}

	seen := map[string]string{}
	for _, p := range pairs {
		name := DeterministicConfigMapName(p[0], p[1])
		key := fmt.Sprintf("%s/%s", p[0], p[1])
		if other, ok := seen[name]; ok {
			t.Fatalf("name collision %q between %q and %q", name, other, key)
		}
		seen[name] = key
	}
}

func TestBuildConnectionConfigMap(t *testing.T) {
	t.Parallel()

	database := &storagev1alpha1.Database{}
	database.Name = testDB
	database.Namespace = "team-a"

	const user = "payments-api-mi" // resolved managed identity name (differs from IdentityRef)
	coords := Coordinates{
		Host:        testHost,
		Port:        5432,
		DBName:      testDB,
		User:        user,
		IdentityRef: testIdentity,
	}

	cm, err := BuildConnectionConfigMap(database, coords)
	if err != nil {
		t.Fatalf("BuildConnectionConfigMap() error: %v", err)
	}

	if want := DeterministicConfigMapName(testDB, testIdentity); cm.Name != want {
		t.Fatalf("name = %q, want %q", cm.Name, want)
	}
	if cm.Namespace != "team-a" {
		t.Fatalf("namespace = %q, want %q", cm.Namespace, "team-a")
	}

	wantLabels := map[string]string{
		LabelDatabase:  testDB,
		LabelPrincipal: testIdentity,
		LabelComponent: ComponentValue,
	}
	for k, want := range wantLabels {
		if got := cm.Labels[k]; got != want {
			t.Fatalf("label %q = %q, want %q", k, got, want)
		}
	}

	wantData := map[string]string{
		DataKeyHost:    testHost,
		DataKeyPort:    "5432",
		DataKeyDBName:  testDB,
		DataKeyUser:    user,
		DataKeySSLMode: SSLModeRequire,
		DataKeyURI:     fmt.Sprintf("postgresql://%s@%s:5432/%s?sslmode=require", user, testHost, testDB),
	}
	for k, want := range wantData {
		if got := cm.Data[k]; got != want {
			t.Fatalf("data %q = %q, want %q", k, got, want)
		}
	}
}

func TestBuildConnectionConfigMapValidation(t *testing.T) {
	t.Parallel()

	if _, err := BuildConnectionConfigMap(nil, Coordinates{IdentityRef: "x"}); err == nil {
		t.Fatalf("expected error for nil database")
	}

	database := &storagev1alpha1.Database{}
	database.Name = testDB
	if _, err := BuildConnectionConfigMap(database, Coordinates{IdentityRef: "  "}); err == nil {
		t.Fatalf("expected error for empty IdentityRef")
	}
}
