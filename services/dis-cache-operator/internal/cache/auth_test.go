package cache

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestBuildAuthSecret(t *testing.T) {
	t.Parallel()

	secret := BuildAuthSecret(newTestCache(nil), "secret-password-123")

	if secret.Name != "app-one-cache-cache-auth" || secret.Namespace != "team-a" {
		t.Fatalf("unexpected name/namespace: %s/%s", secret.Namespace, secret.Name)
	}
	if secret.Type != corev1.SecretTypeOpaque {
		t.Errorf("type: want Opaque, got %s", secret.Type)
	}
	if secret.Labels[ManagedByLabel] != ManagedByValue {
		t.Errorf("missing managed-by label, got %v", secret.Labels)
	}
	if got := string(secret.Data[AuthSecretUsernameKey]); got != AuthUsername {
		t.Errorf("username: want %s, got %q", AuthUsername, got)
	}
	if got := string(secret.Data[AuthSecretPasswordKey]); got != "secret-password-123" {
		t.Errorf("password: want secret-password-123, got %q", got)
	}
}
