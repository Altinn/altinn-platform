package database

import (
	"strings"
	"testing"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
)

func TestResolveExtensionSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		input                []storagev1alpha1.DatabaseServerExtension
		wantExtensionsValue  string
		wantPreloadLibsValue string
		wantErrContains      string
	}{
		{
			name:                 "empty list",
			input:                nil,
			wantExtensionsValue:  "",
			wantPreloadLibsValue: "",
		},
		{
			name: "all supported extensions sorted and deduplicated",
			input: []storagev1alpha1.DatabaseServerExtension{
				storagev1alpha1.DatabaseServerExtensionPgAudit,
				storagev1alpha1.DatabaseServerExtensionPgCron,
				storagev1alpha1.DatabaseServerExtensionHstore,
				storagev1alpha1.DatabaseServerExtensionPgCron,
				storagev1alpha1.DatabaseServerExtensionUUIDOSSP,
				storagev1alpha1.DatabaseServerExtensionPgStatStatements,
			},
			wantExtensionsValue:  "hstore,pg_cron,pg_stat_statements,pgaudit,uuid-ossp",
			wantPreloadLibsValue: "pg_cron,pg_stat_statements,pgaudit",
		},
		{
			name: "extensions without preload requirements",
			input: []storagev1alpha1.DatabaseServerExtension{
				storagev1alpha1.DatabaseServerExtensionHstore,
				storagev1alpha1.DatabaseServerExtensionUUIDOSSP,
			},
			wantExtensionsValue:  "hstore,uuid-ossp",
			wantPreloadLibsValue: "",
		},
		{
			name: "unknown extension is rejected",
			input: []storagev1alpha1.DatabaseServerExtension{
				storagev1alpha1.DatabaseServerExtension("mycrazyextension"),
			},
			wantErrContains: "unsupported extension",
		},
		{
			name: "wrong casing is rejected",
			input: []storagev1alpha1.DatabaseServerExtension{
				storagev1alpha1.DatabaseServerExtension("HSTORE"),
			},
			wantErrContains: "unsupported extension",
		},
	}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotExtensionsValue, gotPreloadLibsValue, err := ResolveExtensionSettings(tc.input)

			if tc.wantErrContains != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrContains)
				}
				if !strings.Contains(err.Error(), tc.wantErrContains) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErrContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotExtensionsValue != tc.wantExtensionsValue {
				t.Fatalf("extensions value mismatch: got %q, want %q", gotExtensionsValue, tc.wantExtensionsValue)
			}
			if gotPreloadLibsValue != tc.wantPreloadLibsValue {
				t.Fatalf("shared_preload_libraries mismatch: got %q, want %q", gotPreloadLibsValue, tc.wantPreloadLibsValue)
			}
		})
	}
}
