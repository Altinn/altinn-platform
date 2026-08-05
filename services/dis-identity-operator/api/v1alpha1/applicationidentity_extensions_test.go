package v1alpha1

import (
	"maps"
	"testing"

	managedidentity "github.com/Azure/azure-service-operator/v2/api/managedidentity/v1api20230131"
)

const (
	testTagsTeamKey    = "team"
	testTagsTeamValue  = "dp-backend"
	testTagsEnvKey     = "env"
	testTagsEnvValue   = "at22"
	testTagsProductKey = "finops_product"
	testTagsProduct    = "dialogporten"
)

func newTagsApplicationIdentity(tags map[string]string) *ApplicationIdentity {
	a := &ApplicationIdentity{}
	a.Name = "my-app"
	a.Namespace = "product-dialogporten"
	a.Spec.Tags = tags
	return a
}

func TestGetUserAssignedIdentityTagsWithoutPlatformTags(t *testing.T) {
	t.Parallel()

	a := newTagsApplicationIdentity(map[string]string{testTagsTeamKey: testTagsTeamValue})

	got := a.GetUserAssignedIdentityTags(nil)

	want := map[string]string{
		testTagsTeamKey:         testTagsTeamValue,
		managedByDisIdentityTag: managedByDisIdentityTagValue,
	}
	if !maps.Equal(got, want) {
		t.Fatalf("want %#v, got %#v", want, got)
	}
}

func TestGetUserAssignedIdentityTagsPlatformTagsWin(t *testing.T) {
	t.Parallel()

	a := newTagsApplicationIdentity(map[string]string{
		testTagsTeamKey:         testTagsTeamValue,
		testTagsProductKey:      "not-my-product",
		managedByDisIdentityTag: "false",
	})
	platformTags := map[string]string{
		testTagsProductKey: testTagsProduct,
		testTagsEnvKey:     testTagsEnvValue,
	}

	got := a.GetUserAssignedIdentityTags(platformTags)

	want := map[string]string{
		testTagsTeamKey:         testTagsTeamValue,
		testTagsProductKey:      testTagsProduct,
		testTagsEnvKey:          testTagsEnvValue,
		managedByDisIdentityTag: managedByDisIdentityTagValue,
	}
	if !maps.Equal(got, want) {
		t.Fatalf("expected platform tags to win over tenant tags: want %#v, got %#v", want, got)
	}
}

func TestOutdatedUserAssignedIdentity(t *testing.T) {
	t.Parallel()

	a := newTagsApplicationIdentity(map[string]string{testTagsTeamKey: testTagsTeamValue})
	platformTags := map[string]string{testTagsEnvKey: testTagsEnvValue}

	if !a.OutdatedUserAssignedIdentity(nil, platformTags) {
		t.Fatalf("expected nil identity to be outdated")
	}

	identity := &managedidentity.UserAssignedIdentity{}
	identity.Spec.Tags = a.GetUserAssignedIdentityTags(platformTags)
	if a.OutdatedUserAssignedIdentity(identity, platformTags) {
		t.Fatalf("expected identity with expected tags to be up to date")
	}

	// An identity created before platform tagging lacks the platform tags and
	// must be detected as outdated so the reconciler patches it.
	identity.Spec.Tags = a.GetUserAssignedIdentityTags(nil)
	if !a.OutdatedUserAssignedIdentity(identity, platformTags) {
		t.Fatalf("expected identity without platform tags to be outdated")
	}
}
