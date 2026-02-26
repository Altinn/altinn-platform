package database

import (
	"testing"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestResolveMaxConnections(t *testing.T) {
	t.Run("maps Standard_B1ms to 50", func(t *testing.T) {
		got, err := ResolveMaxConnections(devProfile)
		if err != nil {
			t.Fatalf("ResolveMaxConnections(devProfile) returned error: %v", err)
		}
		if got != 50 {
			t.Fatalf("ResolveMaxConnections(devProfile) = %d, want 50", got)
		}
	})

	t.Run("maps Standard_D4s_v3 to 1718", func(t *testing.T) {
		got, err := ResolveMaxConnections(prodProfile)
		if err != nil {
			t.Fatalf("ResolveMaxConnections(prodProfile) returned error: %v", err)
		}
		if got != 1718 {
			t.Fatalf("ResolveMaxConnections(prodProfile) = %d, want 1718", got)
		}
	})

	t.Run("caps high memory at Azure maximum", func(t *testing.T) {
		profile := Profile{SkuName: "fake-high-memory", MemoryGB: 64}
		got, err := ResolveMaxConnections(profile)
		if err != nil {
			t.Fatalf("ResolveMaxConnections(profile) returned error: %v", err)
		}
		if got != maxConnectionsLimit {
			t.Fatalf("ResolveMaxConnections(profile) = %d, want %d", got, maxConnectionsLimit)
		}
	})

	t.Run("fails for invalid memory", func(t *testing.T) {
		_, err := ResolveMaxConnections(Profile{SkuName: "bad", MemoryGB: 0})
		if err == nil {
			t.Fatalf("expected error for invalid memory, got nil")
		}
	})
}

func TestResolveServerParameters(t *testing.T) {
	toMap := func(params []ServerParameter) map[string]string {
		out := make(map[string]string, len(params))
		for i := range params {
			out[params[i].Name] = params[i].Value
		}
		return out
	}

	t.Run("includes non-overridable defaults for dev profile", func(t *testing.T) {
		got, err := ResolveServerParameters("dev", nil)
		if err != nil {
			t.Fatalf("ResolveServerParameters(dev, nil) returned error: %v", err)
		}

		values := toMap(got)
		if values[ServerParameterPgBouncerEnabled] != "true" {
			t.Fatalf("pgbouncer.enabled = %q, want %q", values[ServerParameterPgBouncerEnabled], "true")
		}
		if values[ServerParameterPgBouncerMaxPrepared] != "5000" {
			t.Fatalf("pgbouncer.max_prepared_statements = %q, want %q", values[ServerParameterPgBouncerMaxPrepared], "5000")
		}
		if values[ServerParameterPgBouncerPoolMode] != "transaction" {
			t.Fatalf("pgbouncer.pool_mode = %q, want %q", values[ServerParameterPgBouncerPoolMode], "transaction")
		}
		if values[ServerParameterMaxConnections] != "50" {
			t.Fatalf("max_connections = %q, want %q", values[ServerParameterMaxConnections], "50")
		}
	})

	t.Run("includes profile-specific max_connections for prod profile", func(t *testing.T) {
		got, err := ResolveServerParameters("prod", nil)
		if err != nil {
			t.Fatalf("ResolveServerParameters(prod, nil) returned error: %v", err)
		}

		values := toMap(got)
		if values[ServerParameterMaxConnections] != "1718" {
			t.Fatalf("max_connections = %q, want %q", values[ServerParameterMaxConnections], "1718")
		}
	})

	t.Run("merges user provided server parameters", func(t *testing.T) {
		got, err := ResolveServerParameters("dev", []storagev1alpha1.DatabaseServerParameter{
			{
				Name:  "autovacuum_naptime",
				Value: intstr.FromInt(15),
			},
			{
				Name:  "log_connections",
				Value: intstr.FromString("on"),
			},
		})
		if err != nil {
			t.Fatalf("ResolveServerParameters(dev, requested) returned error: %v", err)
		}

		values := toMap(got)
		if values["autovacuum_naptime"] != "15" {
			t.Fatalf("autovacuum_naptime = %q, want %q", values["autovacuum_naptime"], "15")
		}
		if values["log_connections"] != "on" {
			t.Fatalf("log_connections = %q, want %q", values["log_connections"], "on")
		}
	})

	t.Run("rejects extension managed parameters", func(t *testing.T) {
		_, err := ResolveServerParameters("dev", []storagev1alpha1.DatabaseServerParameter{
			{
				Name:  ServerParameterAzureExtensions,
				Value: intstr.FromString("hstore"),
			},
		})
		if err == nil {
			t.Fatalf("expected error for %q override, got nil", ServerParameterAzureExtensions)
		}
	})

	t.Run("rejects non-overridable parameters", func(t *testing.T) {
		_, err := ResolveServerParameters("dev", []storagev1alpha1.DatabaseServerParameter{
			{
				Name:  ServerParameterMaxConnections,
				Value: intstr.FromInt(100),
			},
		})
		if err == nil {
			t.Fatalf("expected error for %q override, got nil", ServerParameterMaxConnections)
		}
	})

	t.Run("rejects empty names and values", func(t *testing.T) {
		_, err := ResolveServerParameters("dev", []storagev1alpha1.DatabaseServerParameter{
			{
				Name:  " ",
				Value: intstr.FromInt(1),
			},
		})
		if err == nil {
			t.Fatalf("expected error for empty parameter name, got nil")
		}

		_, err = ResolveServerParameters("dev", []storagev1alpha1.DatabaseServerParameter{
			{
				Name:  "autovacuum_naptime",
				Value: intstr.FromString(" "),
			},
		})
		if err == nil {
			t.Fatalf("expected error for empty string parameter value, got nil")
		}
	})
}
