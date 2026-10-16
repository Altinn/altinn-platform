package config

import (
	"maps"
	"strings"
	"testing"
)

const (
	testSubscriptionID = "00000000-0000-0000-0000-000000000001"
	testTenantID       = "00000000-0000-0000-0000-000000000002"
	testNamespaceID    = "/subscriptions/" + testSubscriptionID +
		"/resourceGroups/rg-messaging/providers/Microsoft.ServiceBus/namespaces/sbns-cluster-a"
)

func getenv(overrides map[string]string) func(string) string {
	env := map[string]string{
		envSubscriptionID:      testSubscriptionID,
		envTenantID:            testTenantID,
		envNamespaceResourceID: testNamespaceID,
		envEnvironment:         "test",
	}
	maps.Copy(env, overrides)
	return func(key string) string { return env[key] }
}

func TestLoad(t *testing.T) {
	t.Parallel()

	cfg, err := Load(getenv(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := OperatorConfig{
		SubscriptionID:        testSubscriptionID,
		TenantID:              testTenantID,
		Environment:           "test",
		NamespaceResourceID:   testNamespaceID,
		NamespaceName:         "sbns-cluster-a",
		Endpoint:              "sbns-cluster-a.servicebus.windows.net",
		MaxQueuesPerNamespace: 50,
	}
	if *cfg != want {
		t.Fatalf("got %+v, want %+v", *cfg, want)
	}

	// ARM IDs are case-insensitive. The endpoint is always lower case.
	cfg, err = Load(getenv(map[string]string{
		envNamespaceResourceID:   strings.ToUpper(testNamespaceID),
		envMaxQueuesPerNamespace: " 10 ",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Endpoint != want.Endpoint || cfg.MaxQueuesPerNamespace != 10 {
		t.Fatalf("got endpoint %q and queue limit %d", cfg.Endpoint, cfg.MaxQueuesPerNamespace)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	_, err := Load(func(string) string { return " " })
	wantMissing := "missing required configuration: " +
		"DISMSG_AZURE_SUBSCRIPTION_ID, DISMSG_AZURE_TENANT_ID, DISMSG_NAMESPACE_RESOURCE_ID, DISMSG_ENV"
	if err == nil || err.Error() != wantMissing {
		t.Fatalf("expected %q, got %v", wantMissing, err)
	}

	namespaceID := func(subscriptionID, name string) string {
		return "/subscriptions/" + subscriptionID + "/resourceGroups/rg-messaging/providers/Microsoft.ServiceBus/namespaces/" + name
	}
	keyVaultID := strings.Replace(testNamespaceID, "Microsoft.ServiceBus/namespaces", "Microsoft.KeyVault/vaults", 1)

	tests := map[string]struct {
		key, value, wantErr string
	}{
		"subscription is not a UUID":        {envSubscriptionID, "sub-a", envSubscriptionID},
		"tenant is not a UUID":              {envTenantID, "tenant-a", envTenantID},
		"another resource type":             {envNamespaceResourceID, keyVaultID, "Microsoft.ServiceBus/namespaces"},
		"a queue instead of the namespace":  {envNamespaceResourceID, testNamespaceID + "/queues/app-one", "Microsoft.ServiceBus/namespaces"},
		"namespace in another subscription": {envNamespaceResourceID, namespaceID("00000000-0000-0000-0000-000000000009", "sbns-cluster-a"), "subscription"},
		"namespace name too short":          {envNamespaceResourceID, namespaceID(testSubscriptionID, "sbns"), "namespace name"},
		"namespace name ends with a hyphen": {envNamespaceResourceID, namespaceID(testSubscriptionID, "sbns-cluster-"), "namespace name"},
		"queue limit is zero":               {envMaxQueuesPerNamespace, "0", envMaxQueuesPerNamespace},
		"queue limit is not a number":       {envMaxQueuesPerNamespace, "fifty", envMaxQueuesPerNamespace},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, err := Load(getenv(map[string]string{tt.key: tt.value}))
			if err == nil {
				t.Fatalf("expected an error, got config %+v", *cfg)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error to contain %q, got %q", tt.wantErr, err)
			}
		})
	}
}
