package vault

import (
	"context"
	"testing"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const identityNotReadyReason = "IdentityNotReady"

func TestResolveOwnerIdentityForReadyApplicationIdentity(t *testing.T) {
	t.Parallel()

	scheme := newIdentityTestScheme(t)
	readyPrincipalID := "principal-123"
	readyName := "managed-identity-name"

	readyIdentity := &identityv1alpha1.ApplicationIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: "app-ready", Namespace: "default"},
		Status: identityv1alpha1.ApplicationIdentityStatus{
			ManagedIdentityName: &readyName,
			PrincipalID:         &readyPrincipalID,
			Conditions: []metav1.Condition{{
				Type:   string(identityv1alpha1.ConditionReady),
				Status: metav1.ConditionTrue,
				Reason: "Ready",
			}},
		},
	}
	vaultObj := &vaultv1alpha1.Vault{
		ObjectMeta: metav1.ObjectMeta{Name: "vault-sample", Namespace: "default"},
		Spec: vaultv1alpha1.VaultSpec{
			IdentityRef: &vaultv1alpha1.ApplicationIdentityRef{Name: "app-ready"},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(readyIdentity).WithObjects(readyIdentity).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, vaultObj)
	if err != nil {
		t.Fatalf("expected identity resolver to succeed, got error: %v", err)
	}
	if requeue {
		t.Fatalf("expected requeue=false for ready identity")
	}
	if resolved.PrincipalID != readyPrincipalID {
		t.Fatalf("expected principalId %q, got %q", readyPrincipalID, resolved.PrincipalID)
	}
	if resolved.SourceKind != IdentitySourceApplicationIdentity {
		t.Fatalf("expected source kind %q, got %q", IdentitySourceApplicationIdentity, resolved.SourceKind)
	}
	if resolved.AuthReferenceName != "app-ready" || resolved.ServiceAccountName != "app-ready" {
		t.Fatalf("expected auth reference and service account names to match app-ready, got %#v", resolved)
	}
}

func TestResolveOwnerIdentityForUnreadyApplicationIdentity(t *testing.T) {
	t.Parallel()

	scheme := newIdentityTestScheme(t)
	identity := &identityv1alpha1.ApplicationIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: "app-pending", Namespace: "default"},
	}
	vaultObj := &vaultv1alpha1.Vault{
		ObjectMeta: metav1.ObjectMeta{Name: "vault-sample", Namespace: "default"},
		Spec: vaultv1alpha1.VaultSpec{
			IdentityRef: &vaultv1alpha1.ApplicationIdentityRef{Name: "app-pending"},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(identity).WithObjects(identity).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, vaultObj)
	if err != nil {
		t.Fatalf("expected identity resolver to return pending status, got error: %v", err)
	}
	if !requeue {
		t.Fatalf("expected requeue=true for unready identity")
	}
	if resolved.PendingReason != identityNotReadyReason {
		t.Fatalf("expected pending reason %s, got %q", identityNotReadyReason, resolved.PendingReason)
	}
}

func TestResolveOwnerIdentityForServiceAccount(t *testing.T) {
	t.Parallel()

	scheme := newIdentityTestScheme(t)
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vault-owner-sa",
			Namespace: "default",
			Annotations: map[string]string{
				ServiceAccountClientIDAnnotation:    "client-123",
				ServiceAccountPrincipalIDAnnotation: "principal-123",
			},
		},
	}
	vaultObj := &vaultv1alpha1.Vault{
		ObjectMeta: metav1.ObjectMeta{Name: "vault-sample", Namespace: "default"},
		Spec: vaultv1alpha1.VaultSpec{
			ServiceAccountRef: &vaultv1alpha1.ServiceAccountRef{Name: "vault-owner-sa"},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(serviceAccount).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, vaultObj)
	if err != nil {
		t.Fatalf("expected service account resolver to succeed, got error: %v", err)
	}
	if requeue {
		t.Fatalf("expected requeue=false for annotated service account")
	}
	if resolved.PrincipalID != "principal-123" {
		t.Fatalf("expected principalId %q, got %q", "principal-123", resolved.PrincipalID)
	}
	if resolved.SourceKind != IdentitySourceServiceAccount {
		t.Fatalf("expected source kind %q, got %q", IdentitySourceServiceAccount, resolved.SourceKind)
	}
	if resolved.ServiceAccountName != "vault-owner-sa" || resolved.AuthReferenceName != "vault-owner-sa" {
		t.Fatalf("expected auth reference and service account names to match vault-owner-sa, got %#v", resolved)
	}
}

func TestResolveOwnerIdentityForServiceAccountMissingAnnotations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations map[string]string
		wantMessage string
	}{
		{
			name:        "missing client id",
			annotations: map[string]string{ServiceAccountPrincipalIDAnnotation: "principal-123"},
			wantMessage: `ServiceAccount "vault-owner-sa" is missing annotation "azure.workload.identity/client-id"`,
		},
		{
			name:        "missing principal id",
			annotations: map[string]string{ServiceAccountClientIDAnnotation: "client-123"},
			wantMessage: `ServiceAccount "vault-owner-sa" is missing annotation "dis.altinn.cloud/principal-id"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scheme := newIdentityTestScheme(t)
			serviceAccount := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "vault-owner-sa",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}
			vaultObj := &vaultv1alpha1.Vault{
				ObjectMeta: metav1.ObjectMeta{Name: "vault-sample", Namespace: "default"},
				Spec: vaultv1alpha1.VaultSpec{
					ServiceAccountRef: &vaultv1alpha1.ServiceAccountRef{Name: "vault-owner-sa"},
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(serviceAccount).Build()

			resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, vaultObj)
			if err != nil {
				t.Fatalf("expected pending status, got error: %v", err)
			}
			if !requeue {
				t.Fatalf("expected requeue=true when service account annotations are incomplete")
			}
			if resolved.PendingMessage != tt.wantMessage {
				t.Fatalf("expected pending message %q, got %q", tt.wantMessage, resolved.PendingMessage)
			}
		})
	}
}

func TestResolveOwnerIdentityForMissingServiceAccount(t *testing.T) {
	t.Parallel()

	scheme := newIdentityTestScheme(t)
	vaultObj := &vaultv1alpha1.Vault{
		ObjectMeta: metav1.ObjectMeta{Name: "vault-sample", Namespace: "default"},
		Spec: vaultv1alpha1.VaultSpec{
			ServiceAccountRef: &vaultv1alpha1.ServiceAccountRef{Name: "vault-owner-sa"},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, vaultObj)
	if err != nil {
		t.Fatalf("expected pending status, got error: %v", err)
	}
	if !requeue {
		t.Fatalf("expected requeue=true for missing service account")
	}
	if resolved.PendingMessage != `ServiceAccount "vault-owner-sa" not found` {
		t.Fatalf("expected not found message, got %q", resolved.PendingMessage)
	}
}

func newIdentityTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := clientgoscheme(scheme); err != nil {
		t.Fatalf("failed to add schemes: %v", err)
	}

	return scheme
}

func clientgoscheme(scheme *runtime.Scheme) error {
	if err := identityv1alpha1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := vaultv1alpha1.AddToScheme(scheme); err != nil {
		return err
	}
	return corev1.AddToScheme(scheme)
}
