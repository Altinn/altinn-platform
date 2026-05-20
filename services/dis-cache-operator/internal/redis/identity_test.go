package redis

import (
	"context"
	"testing"

	redisv1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1: %v", err)
	}
	if err := identityv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add identity scheme: %v", err)
	}
	if err := redisv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add redis scheme: %v", err)
	}
	return scheme
}

func TestResolveOwnerIdentityReadyApplicationIdentity(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	principal := "p-123"
	managedName := "mi-name"

	identity := &identityv1alpha1.ApplicationIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: "app-ready", Namespace: "default"},
		Status: identityv1alpha1.ApplicationIdentityStatus{
			ManagedIdentityName: &managedName,
			PrincipalID:         &principal,
			Conditions: []metav1.Condition{{
				Type:   string(identityv1alpha1.ConditionReady),
				Status: metav1.ConditionTrue,
				Reason: "Ready",
			}},
		},
	}
	redisObj := &redisv1alpha1.Redis{
		ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default"},
		Spec: redisv1alpha1.RedisSpec{
			IdentityRef: &redisv1alpha1.ApplicationIdentityRef{Name: "app-ready"},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(identity).WithObjects(identity).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), c, redisObj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requeue {
		t.Fatalf("expected requeue=false for ready identity")
	}
	if resolved.PrincipalID != principal {
		t.Fatalf("expected principalId %q, got %q", principal, resolved.PrincipalID)
	}
	if resolved.SourceKind != IdentitySourceApplicationIdentity {
		t.Fatalf("expected source kind ApplicationIdentity, got %q", resolved.SourceKind)
	}
}

func TestResolveOwnerIdentityUnreadyApplicationIdentity(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	identity := &identityv1alpha1.ApplicationIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: "app-pending", Namespace: "default"},
	}
	redisObj := &redisv1alpha1.Redis{
		ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default"},
		Spec: redisv1alpha1.RedisSpec{
			IdentityRef: &redisv1alpha1.ApplicationIdentityRef{Name: "app-pending"},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(identity).WithObjects(identity).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), c, redisObj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !requeue {
		t.Fatalf("expected requeue=true for unready identity")
	}
	if resolved.PendingReason != identityNotReadyReason {
		t.Fatalf("expected pending reason %q, got %q", identityNotReadyReason, resolved.PendingReason)
	}
}

func TestResolveOwnerIdentityServiceAccountAnnotated(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cache-sa",
			Namespace: "default",
			Annotations: map[string]string{
				ServiceAccountClientIDAnnotation:    "c-123",
				ServiceAccountPrincipalIDAnnotation: "p-456",
			},
		},
	}
	redisObj := &redisv1alpha1.Redis{
		ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default"},
		Spec: redisv1alpha1.RedisSpec{
			ServiceAccountRef: &redisv1alpha1.ServiceAccountRef{Name: "cache-sa"},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sa).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), c, redisObj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requeue {
		t.Fatalf("expected requeue=false for annotated SA")
	}
	if resolved.PrincipalID != "p-456" {
		t.Fatalf("expected principalId p-456, got %q", resolved.PrincipalID)
	}
	if resolved.SourceKind != IdentitySourceServiceAccount {
		t.Fatalf("expected source kind ServiceAccount")
	}
}

func TestResolveOwnerIdentityServiceAccountMissingAnnotation(t *testing.T) {
	t.Parallel()

	scheme := newTestScheme(t)
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "cache-sa",
			Namespace:   "default",
			Annotations: map[string]string{ServiceAccountClientIDAnnotation: "c-123"},
		},
	}
	redisObj := &redisv1alpha1.Redis{
		ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default"},
		Spec: redisv1alpha1.RedisSpec{
			ServiceAccountRef: &redisv1alpha1.ServiceAccountRef{Name: "cache-sa"},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sa).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), c, redisObj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !requeue {
		t.Fatalf("expected requeue=true for missing principal-id annotation")
	}
	if resolved.PrincipalID != "" {
		t.Fatalf("expected empty principalId, got %q", resolved.PrincipalID)
	}
}
