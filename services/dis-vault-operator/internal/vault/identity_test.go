package vault

import (
	"context"
	"testing"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveOwnerIdentity(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := identityv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add identity scheme: %v", err)
	}

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
	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(readyIdentity).WithObjects(readyIdentity).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, "default", "app-ready")
	if err != nil {
		t.Fatalf("TODO: expected identity resolver to succeed, got error: %v", err)
	}
	if requeue {
		t.Fatalf("TODO: expected requeue=false for ready identity")
	}
	if resolved.PrincipalID != readyPrincipalID {
		t.Fatalf("TODO: expected principalId %q, got %q", readyPrincipalID, resolved.PrincipalID)
	}
	if resolved.Name != readyName {
		t.Fatalf("TODO: expected identity name %q, got %q", readyName, resolved.Name)
	}
}
