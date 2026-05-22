package k8s

import "testing"

type testSpec struct {
	Name  string
	Count int
}

func TestSyncSpecAndLabels(t *testing.T) {
	t.Parallel()

	t.Run("returns false when spec and labels already match", func(t *testing.T) {
		t.Parallel()

		existingSpec := testSpec{Name: "same", Count: 1}
		desiredSpec := testSpec{Name: "same", Count: 1}
		existingLabels := map[string]string{
			"dis.altinn.cloud/database-name": "my-db",
		}
		desiredLabels := map[string]string{
			"dis.altinn.cloud/database-name": "my-db",
		}

		labels, updated := SyncSpecAndLabels(&existingSpec, desiredSpec, existingLabels, desiredLabels)

		if updated {
			t.Fatalf("expected updated=false, got true")
		}
		if existingSpec != desiredSpec {
			t.Fatalf("spec mismatch: got %#v, want %#v", existingSpec, desiredSpec)
		}
		if labels["dis.altinn.cloud/database-name"] != "my-db" {
			t.Fatalf("label mismatch: got %q, want %q", labels["dis.altinn.cloud/database-name"], "my-db")
		}
	})

	t.Run("returns true when spec changes", func(t *testing.T) {
		t.Parallel()

		existingSpec := testSpec{Name: "old", Count: 1}
		desiredSpec := testSpec{Name: "new", Count: 2}

		labels, updated := SyncSpecAndLabels(&existingSpec, desiredSpec, map[string]string{}, map[string]string{})

		if !updated {
			t.Fatalf("expected updated=true, got false")
		}
		if existingSpec != desiredSpec {
			t.Fatalf("spec mismatch: got %#v, want %#v", existingSpec, desiredSpec)
		}
		if labels == nil {
			t.Fatalf("expected labels map to be initialized")
		}
	})

	t.Run("returns true when desired label is missing", func(t *testing.T) {
		t.Parallel()

		existingSpec := testSpec{Name: "same", Count: 1}
		desiredSpec := testSpec{Name: "same", Count: 1}
		desiredLabels := map[string]string{
			"dis.altinn.cloud/database-name": "my-db",
		}

		labels, updated := SyncSpecAndLabels(&existingSpec, desiredSpec, nil, desiredLabels)

		if !updated {
			t.Fatalf("expected updated=true, got false")
		}
		if labels["dis.altinn.cloud/database-name"] != "my-db" {
			t.Fatalf("label mismatch: got %q, want %q", labels["dis.altinn.cloud/database-name"], "my-db")
		}
	})

	t.Run("keeps extra existing labels and returns false when desired set already satisfied", func(t *testing.T) {
		t.Parallel()

		existingSpec := testSpec{Name: "same", Count: 1}
		desiredSpec := testSpec{Name: "same", Count: 1}
		existingLabels := map[string]string{
			"dis.altinn.cloud/database-name": "my-db",
			"custom":                         "keep-me",
		}
		desiredLabels := map[string]string{
			"dis.altinn.cloud/database-name": "my-db",
		}

		labels, updated := SyncSpecAndLabels(&existingSpec, desiredSpec, existingLabels, desiredLabels)

		if updated {
			t.Fatalf("expected updated=false, got true")
		}
		if labels["custom"] != "keep-me" {
			t.Fatalf("extra label should be preserved, got %q", labels["custom"])
		}
	})

	t.Run("initializes nil labels without forcing update when no desired labels", func(t *testing.T) {
		t.Parallel()

		existingSpec := testSpec{Name: "same", Count: 1}
		desiredSpec := testSpec{Name: "same", Count: 1}

		labels, updated := SyncSpecAndLabels(&existingSpec, desiredSpec, nil, map[string]string{})

		if updated {
			t.Fatalf("expected updated=false, got true")
		}
		if labels == nil {
			t.Fatalf("expected labels map to be initialized")
		}
		if len(labels) != 0 {
			t.Fatalf("expected no labels, got %d", len(labels))
		}
	})
}
