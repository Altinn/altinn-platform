package database

import (
	"strings"
	"testing"
)

func TestDeriveLogicalDatabaseName(t *testing.T) {
	tests := []struct {
		name              string
		tenantID          string
		tenantEnvironment string
		databaseKey       string
		want              string
	}{
		{
			name:              "joins spec fields",
			tenantID:          "tenant123",
			tenantEnvironment: "dev",
			databaseKey:       "app-db",
			want:              "tenant123-dev-app-db",
		},
		{
			name:              "lowercases trims and replaces invalid runs",
			tenantID:          " Tenant_123 ",
			tenantEnvironment: "DEV",
			databaseKey:       " app$db ",
			want:              "tenant-123-dev-app-db",
		},
		{
			name:              "prefixes when first character is not a letter",
			tenantID:          "123",
			tenantEnvironment: "dev",
			databaseKey:       "app-db",
			want:              "db-123-dev-app-db",
		},
		{
			name:              "falls back for empty sanitized input",
			tenantID:          "!!!",
			tenantEnvironment: "@@@",
			databaseKey:       "###",
			want:              "db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveLogicalDatabaseName(tt.tenantID, tt.tenantEnvironment, tt.databaseKey)
			if got != tt.want {
				t.Fatalf("DeriveLogicalDatabaseName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveLogicalDatabaseNameTruncatesWithHash(t *testing.T) {
	got := DeriveLogicalDatabaseName(
		strings.Repeat("tenant", 8),
		"production",
		strings.Repeat("database", 8),
	)

	if len(got) != maxLogicalDatabaseNameLength {
		t.Fatalf("len(DeriveLogicalDatabaseName()) = %d, want %d", len(got), maxLogicalDatabaseNameLength)
	}
	if got != "tenanttenanttenanttenanttenanttenanttenanttenant-produ-3d3b217c" {
		t.Fatalf("DeriveLogicalDatabaseName() = %q", got)
	}
	if strings.Contains(got[:maxLogicalDatabaseNameLength-9], "--") {
		t.Fatalf("truncated base contains collapsed separator violation: %q", got)
	}
}
