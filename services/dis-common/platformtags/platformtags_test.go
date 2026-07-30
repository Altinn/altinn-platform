package platformtags

import (
	"maps"
	"testing"
)

const (
	keyEnv             = "env"
	testEnv            = "at22"
	testClusterProduct = "dis-core"
	testProduct        = "dialogporten"
)

func TestParseBase(t *testing.T) {
	t.Parallel()

	got, err := ParseBase(`{"finops_environment":"at22","env":"at22"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"finops_environment": testEnv, keyEnv: testEnv}
	if !maps.Equal(got, want) {
		t.Fatalf("want %#v, got %#v", want, got)
	}

	for name, raw := range map[string]string{"empty": "", "empty object": "{}"} {
		if got, err := ParseBase(raw); err != nil || got != nil {
			t.Fatalf("%s: expected tagging disabled (nil, nil), got %#v, %v", name, got, err)
		}
	}

	for name, raw := range map[string]string{
		"unsubstituted flux placeholder": "${DISPG_BASE_TAGS}",
		"not a JSON object":              `["env"]`,
	} {
		if got, err := ParseBase(raw); err == nil {
			t.Fatalf("%s: expected error, got %#v", name, got)
		}
	}
}

func TestForNamespace(t *testing.T) {
	t.Parallel()

	if got := ForNamespace(nil, "product-"+testProduct); got != nil {
		t.Fatalf("expected nil platform tags when base is unset, got %#v", got)
	}

	base := map[string]string{
		keyEnv:           testEnv,
		keyFinopsProduct: testClusterProduct,
		keyProduct:       testClusterProduct,
	}
	baseBefore := maps.Clone(base)

	got := ForNamespace(base, "product-"+testProduct)
	want := map[string]string{
		keyEnv:           testEnv,
		keyFinopsProduct: testProduct,
		keyProduct:       testProduct,
		keyRepository:    RepositoryURL,
	}
	if !maps.Equal(got, want) {
		t.Fatalf("product namespace: want %#v, got %#v", want, got)
	}
	if !maps.Equal(base, baseBefore) {
		t.Fatalf("base tags map was mutated: %#v", base)
	}

	got = ForNamespace(base, "platform-system")
	want = map[string]string{
		keyEnv:           testEnv,
		keyFinopsProduct: testClusterProduct,
		keyProduct:       testClusterProduct,
		keyRepository:    RepositoryURL,
	}
	if !maps.Equal(got, want) {
		t.Fatalf("non-product namespace: want %#v, got %#v", want, got)
	}
}

func TestMerge(t *testing.T) {
	t.Parallel()

	if got := Merge(nil, nil); got != nil {
		t.Fatalf("expected nil for empty inputs, got %#v", got)
	}

	tenant := map[string]string{"team": "dp-backend", keyFinopsProduct: "not-my-product"}
	platform := map[string]string{keyFinopsProduct: testProduct}

	got := Merge(tenant, platform)

	want := map[string]string{"team": "dp-backend", keyFinopsProduct: testProduct}
	if !maps.Equal(got, want) {
		t.Fatalf("expected platform keys to win: want %#v, got %#v", want, got)
	}
	if tenant[keyFinopsProduct] != "not-my-product" {
		t.Fatalf("tenant map was mutated: %#v", tenant)
	}
}
