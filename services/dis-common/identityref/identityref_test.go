package identityref

import (
	"context"
	"testing"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNamespace    = "team-a"
	testIdentityName = "app-one-identity"
	testSAName       = "app-one-sa"
	testPrincipalID  = "principal-123"
)

func strPtr(s string) *string {
	return &s
}

func TestActiveAuthReferenceName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		source  Source
		want    string
		wantErr bool
	}{
		{
			name:   "identityRef set",
			source: Source{IdentityRefName: strPtr(testIdentityName)},
			want:   testIdentityName,
		},
		{
			name:   "serviceAccountRef set",
			source: Source{ServiceAccountRefName: strPtr(testSAName)},
			want:   testSAName,
		},
		{
			name:    "both set",
			source:  Source{IdentityRefName: strPtr(testIdentityName), ServiceAccountRefName: strPtr(testSAName)},
			wantErr: true,
		},
		{
			name:    "none set",
			source:  Source{},
			wantErr: true,
		},
		{
			name:    "identityRef empty name",
			source:  Source{IdentityRefName: strPtr("  ")},
			wantErr: true,
		},
		{
			name:    "serviceAccountRef empty name",
			source:  Source{ServiceAccountRefName: strPtr("")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ActiveAuthReferenceName(tt.source)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got name %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected name %q, got error: %v", tt.want, err)
			}
			if got != tt.want {
				t.Fatalf("expected name %q, got %q", tt.want, got)
			}
		})
	}
}

func TestResolveOwnerIdentityForReadyApplicationIdentity(t *testing.T) {
	t.Parallel()

	scheme := newIdentityTestScheme(t)
	readyPrincipalID := testPrincipalID
	readyName := "managed-identity-name"

	readyIdentity := &identityv1alpha1.ApplicationIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: testIdentityName, Namespace: testNamespace},
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

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(readyIdentity).
		WithObjects(readyIdentity).
		Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, Source{
		Namespace:       testNamespace,
		IdentityRefName: strPtr(testIdentityName),
	})
	if err != nil {
		t.Fatalf("expected identity resolver to succeed, got error: %v", err)
	}
	if requeue {
		t.Fatalf("expected requeue=false for ready identity")
	}
	if resolved.PrincipalID != testPrincipalID {
		t.Fatalf("expected principalId %q, got %q", testPrincipalID, resolved.PrincipalID)
	}
	if resolved.SourceKind != IdentitySourceApplicationIdentity {
		t.Fatalf("expected source kind %q, got %q", IdentitySourceApplicationIdentity, resolved.SourceKind)
	}
	if resolved.AuthReferenceName != testIdentityName || resolved.ServiceAccountName != testIdentityName {
		t.Fatalf("expected auth reference and service account names to match %q, got %#v", testIdentityName, resolved)
	}
}

func TestResolveOwnerIdentityForUnreadyApplicationIdentity(t *testing.T) {
	t.Parallel()

	scheme := newIdentityTestScheme(t)
	identity := &identityv1alpha1.ApplicationIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: "app-pending", Namespace: testNamespace},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(identity).WithObjects(identity).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, Source{
		Namespace:       testNamespace,
		IdentityRefName: strPtr("app-pending"),
	})
	if err != nil {
		t.Fatalf("expected identity resolver to return pending status, got error: %v", err)
	}
	if !requeue {
		t.Fatalf("expected requeue=true for unready identity")
	}
	if resolved.PendingReason != ReasonIdentityNotReady {
		t.Fatalf("expected pending reason %s, got %q", ReasonIdentityNotReady, resolved.PendingReason)
	}
}

func TestResolveOwnerIdentityForMissingApplicationIdentity(t *testing.T) {
	t.Parallel()

	scheme := newIdentityTestScheme(t)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, Source{
		Namespace:       testNamespace,
		IdentityRefName: strPtr(testIdentityName),
	})
	if err != nil {
		t.Fatalf("expected pending status, got error: %v", err)
	}
	if !requeue {
		t.Fatalf("expected requeue=true for missing identity")
	}
	if resolved.PendingMessage != `ApplicationIdentity "app-one-identity" not found` {
		t.Fatalf("expected not found message, got %q", resolved.PendingMessage)
	}
}

func TestResolveOwnerIdentityForServiceAccount(t *testing.T) {
	t.Parallel()

	scheme := newIdentityTestScheme(t)
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSAName,
			Namespace: testNamespace,
			Annotations: map[string]string{
				ServiceAccountClientIDAnnotation:    "client-123",
				ServiceAccountPrincipalIDAnnotation: testPrincipalID,
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(serviceAccount).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, Source{
		Namespace:             testNamespace,
		ServiceAccountRefName: strPtr(testSAName),
	})
	if err != nil {
		t.Fatalf("expected service account resolver to succeed, got error: %v", err)
	}
	if requeue {
		t.Fatalf("expected requeue=false for annotated service account")
	}
	if resolved.PrincipalID != testPrincipalID {
		t.Fatalf("expected principalId %q, got %q", testPrincipalID, resolved.PrincipalID)
	}
	if resolved.SourceKind != IdentitySourceServiceAccount {
		t.Fatalf("expected source kind %q, got %q", IdentitySourceServiceAccount, resolved.SourceKind)
	}
	if resolved.ServiceAccountName != testSAName || resolved.AuthReferenceName != testSAName {
		t.Fatalf("expected auth reference and service account names to match %q, got %#v", testSAName, resolved)
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
			annotations: map[string]string{ServiceAccountPrincipalIDAnnotation: testPrincipalID},
			wantMessage: `ServiceAccount "app-one-sa" is missing annotation "azure.workload.identity/client-id"`,
		},
		{
			name:        "missing principal id",
			annotations: map[string]string{ServiceAccountClientIDAnnotation: "client-123"},
			wantMessage: `ServiceAccount "app-one-sa" is missing annotation "dis.altinn.cloud/principal-id"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scheme := newIdentityTestScheme(t)
			serviceAccount := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testSAName,
					Namespace:   testNamespace,
					Annotations: tt.annotations,
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(serviceAccount).Build()

			resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, Source{
				Namespace:             testNamespace,
				ServiceAccountRefName: strPtr(testSAName),
			})
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
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, Source{
		Namespace:             testNamespace,
		ServiceAccountRefName: strPtr(testSAName),
	})
	if err != nil {
		t.Fatalf("expected pending status, got error: %v", err)
	}
	if !requeue {
		t.Fatalf("expected requeue=true for missing service account")
	}
	if resolved.PendingMessage != `ServiceAccount "app-one-sa" not found` {
		t.Fatalf("expected not found message, got %q", resolved.PendingMessage)
	}
}

func TestResolveOwnerIdentityForInvalidSpec(t *testing.T) {
	t.Parallel()

	scheme := newIdentityTestScheme(t)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	tests := []struct {
		name   string
		source Source
	}{
		{
			name: "both refs set",
			source: Source{
				Namespace:             testNamespace,
				IdentityRefName:       strPtr(testIdentityName),
				ServiceAccountRefName: strPtr(testSAName),
			},
		},
		{
			name:   "no refs set",
			source: Source{Namespace: testNamespace},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resolved, requeue, err := ResolveOwnerIdentity(context.Background(), client, tt.source)
			if err != nil {
				t.Fatalf("expected pending status, got error: %v", err)
			}
			if !requeue {
				t.Fatalf("expected requeue=true for invalid spec")
			}
			if resolved.PendingReason != ReasonInvalidSpec {
				t.Fatalf("expected pending reason %s, got %q", ReasonInvalidSpec, resolved.PendingReason)
			}
		})
	}
}

func newIdentityTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := identityv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add identity scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add core scheme: %v", err)
	}

	return scheme
}
