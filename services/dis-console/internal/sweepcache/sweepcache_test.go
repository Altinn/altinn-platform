package sweepcache

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"

	"github.com/valkey-io/valkey-go"
)

func testClient(prefix string) *Client { return &Client{prefix: prefix} }

func TestSplitSeparatesUnchangedObjects(t *testing.T) {
	t.Parallel()

	resources := []flux.Resource{
		{Kind: "Kustomization", Namespace: "team-a", Name: "app-one", ContentHash: "aaa"},
		{Kind: "Kustomization", Namespace: "team-a", Name: "app-two", ContentHash: "bbb"},
		{Kind: "HelmRelease", Namespace: "team-b", Name: "app-three", ContentHash: "ccc"},
	}
	c := testClient("v1")
	known := map[string]string{
		key("v1", &resources[0]): fingerprint(&resources[0]), // same: unchanged
		key("v1", &resources[1]): "old|",                     // different hash: changed
		// app-three has no entry: changed
	}

	changed, unchanged := c.Split(resources, known)

	if len(unchanged) != 1 || unchanged[0].Name != "app-one" {
		t.Errorf("unchanged: want [app-one], got %v", names(unchanged))
	}
	if len(changed) != 2 || changed[0].Name != "app-two" || changed[1].Name != "app-three" {
		t.Errorf("changed: want [app-two app-three], got %v", names(changed))
	}
}

func TestSplitSeesAChangedApplier(t *testing.T) {
	t.Parallel()

	before := flux.Resource{Kind: "HelmRelease", Namespace: "team-a", Name: "app-one", ContentHash: "aaa",
		AppliedBy: &flux.AppliedBy{Name: "release-one", Namespace: "team-a"}}
	after := before
	after.AppliedBy = &flux.AppliedBy{Name: "release-two", Namespace: "team-a"}
	c := testClient("v1")
	known := map[string]string{key("v1", &before): fingerprint(&before)}

	changed, unchanged := c.Split([]flux.Resource{after}, known)
	if len(changed) != 1 || len(unchanged) != 0 {
		t.Errorf("a new applier with the same hash must count as changed, got changed=%d unchanged=%d", len(changed), len(unchanged))
	}
}

func TestSplitTreatsAnotherBuildAsChanged(t *testing.T) {
	t.Parallel()

	resources := []flux.Resource{{Kind: "Kustomization", Namespace: "team-a", Name: "app-one", ContentHash: "aaa"}}
	known := map[string]string{key("v1", &resources[0]): fingerprint(&resources[0])}

	changed, unchanged := testClient("v2").Split(resources, known)
	if len(changed) != 1 || len(unchanged) != 0 {
		t.Errorf("a new prefix must miss every entry, got changed=%d unchanged=%d", len(changed), len(unchanged))
	}
}

func TestNilClientIsANoop(t *testing.T) {
	t.Parallel()

	var c *Client
	resources := []flux.Resource{{Kind: "Kustomization", Namespace: "team-a", Name: "app-one", ContentHash: "aaa"}}

	hashes, err := c.Hashes(context.Background(), resources)
	if err != nil || len(hashes) != 0 {
		t.Errorf("nil client Hashes: want empty, got %v, %v", hashes, err)
	}
	if err := c.Remember(context.Background(), resources); err != nil {
		t.Errorf("nil client Remember: %v", err)
	}
	changed, unchanged := c.Split(resources, map[string]string{key("v1", &resources[0]): fingerprint(&resources[0])})
	if len(changed) != 1 || len(unchanged) != 0 {
		t.Errorf("nil client Split: want everything changed, got changed=%d unchanged=%d", len(changed), len(unchanged))
	}
	c.Close()
}

func TestNewWithoutAddressIsDisabled(t *testing.T) {
	t.Parallel()

	c, err := New(context.Background(), Config{}, "v1")
	if err != nil || c != nil {
		t.Errorf("empty address: want nil client and no error, got %v, %v", c, err)
	}
}

func names(resources []flux.Resource) []string {
	out := make([]string, len(resources))
	for i := range resources {
		out[i] = resources[i].Name
	}

	return out
}

func TestFileCredentialsReadsTheFileOnEveryCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("first-password"), 0o600); err != nil {
		t.Fatal(err)
	}
	credentials := FileCredentials("app", path)

	got, err := credentials(valkey.AuthCredentialsContext{})
	if err != nil || got.Username != "app" || got.Password != "first-password" {
		t.Fatalf("first call: want app/first-password, got %q/%q (%v)", got.Username, got.Password, err)
	}

	// A rotation replaces the file content; the next connection reads it.
	if err := os.WriteFile(path, []byte("second-password\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = credentials(valkey.AuthCredentialsContext{})
	if err != nil || got.Password != "second-password" {
		t.Fatalf("after the rotation: want second-password, got %q (%v)", got.Password, err)
	}
}

func TestFileCredentialsFailsWithoutAPassword(t *testing.T) {
	dir := t.TempDir()
	if _, err := FileCredentials("app", filepath.Join(dir, "missing"))(valkey.AuthCredentialsContext{}); err == nil {
		t.Error("a missing file: want an error")
	}

	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := FileCredentials("app", empty)(valkey.AuthCredentialsContext{}); err == nil {
		t.Error("an empty file: want an error")
	}
}

func TestClientOptionPrefersThePasswordFile(t *testing.T) {
	withFile := clientOption(Config{Address: "cache:6379", Username: "app", Password: "from-env", PasswordFile: "/run/password"})
	if withFile.AuthCredentialsFn == nil || withFile.Password != "" {
		t.Errorf("with a file: want the credentials function and no fixed password, got fn=%v password=%q",
			withFile.AuthCredentialsFn != nil, withFile.Password)
	}

	withoutFile := clientOption(Config{Address: "cache:6379", Username: "app", Password: "from-env"})
	if withoutFile.AuthCredentialsFn != nil || withoutFile.Password != "from-env" {
		t.Errorf("without a file: want the fixed password, got fn=%v password=%q",
			withoutFile.AuthCredentialsFn != nil, withoutFile.Password)
	}
}
