package vault

import (
	"context"
	"fmt"
	"strings"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ServiceAccountClientIDAnnotation    = "azure.workload.identity/client-id"
	ServiceAccountPrincipalIDAnnotation = "dis.altinn.cloud/principal-id"
)

type IdentitySourceKind string

const (
	IdentitySourceApplicationIdentity IdentitySourceKind = "ApplicationIdentity"
	IdentitySourceServiceAccount      IdentitySourceKind = "ServiceAccount"
)

// ResolvedIdentity contains owner identity values required for provisioning.
type ResolvedIdentity struct {
	SourceKind         IdentitySourceKind
	SourceName         string
	AuthReferenceName  string
	ServiceAccountName string
	PrincipalID        string
	PendingReason      string
	PendingMessage     string
}

func (r ResolvedIdentity) IsPending() bool {
	return r.PendingReason != ""
}

func (r ResolvedIdentity) SourceDescription() string {
	if r.SourceKind == "" || r.SourceName == "" {
		return "identity source"
	}

	return fmt.Sprintf("%s %q", r.SourceKind, r.SourceName)
}

func ActiveAuthReferenceName(v *vaultv1alpha1.Vault) (string, error) {
	if v == nil {
		return "", fmt.Errorf("vault must not be nil")
	}

	switch {
	case v.Spec.ServiceAccountRef != nil:
		name := strings.TrimSpace(v.Spec.ServiceAccountRef.Name)
		if name == "" {
			return "", fmt.Errorf("serviceAccountRef.name must not be empty")
		}
		return name, nil
	case v.Spec.IdentityRef != nil:
		name := strings.TrimSpace(v.Spec.IdentityRef.Name)
		if name == "" {
			return "", fmt.Errorf("identityRef.name must not be empty")
		}
		return name, nil
	default:
		return "", fmt.Errorf("exactly one of identityRef or serviceAccountRef must be set")
	}
}

// ResolveOwnerIdentity resolves the active owner identity source for a Vault.
// The bool return indicates whether caller should requeue.
func ResolveOwnerIdentity(ctx context.Context, c client.Reader, v *vaultv1alpha1.Vault) (ResolvedIdentity, bool, error) {
	if v == nil {
		return ResolvedIdentity{}, false, fmt.Errorf("vault must not be nil")
	}

	switch {
	case v.Spec.IdentityRef != nil && v.Spec.ServiceAccountRef != nil:
		return ResolvedIdentity{
			PendingReason:  "InvalidSpec",
			PendingMessage: "exactly one of identityRef or serviceAccountRef must be set",
		}, true, nil
	case v.Spec.IdentityRef != nil:
		return resolveApplicationIdentity(ctx, c, v.Namespace, v.Spec.IdentityRef.Name)
	case v.Spec.ServiceAccountRef != nil:
		return resolveServiceAccount(ctx, c, v.Namespace, v.Spec.ServiceAccountRef.Name)
	default:
		return ResolvedIdentity{
			PendingReason:  "InvalidSpec",
			PendingMessage: "exactly one of identityRef or serviceAccountRef must be set",
		}, true, nil
	}
}

func resolveApplicationIdentity(ctx context.Context, c client.Reader, namespace, identityName string) (ResolvedIdentity, bool, error) {
	identityName = strings.TrimSpace(identityName)
	resolved := ResolvedIdentity{
		SourceKind:         IdentitySourceApplicationIdentity,
		SourceName:         identityName,
		AuthReferenceName:  identityName,
		ServiceAccountName: identityName,
	}

	var identity identityv1alpha1.ApplicationIdentity
	if err := c.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      identityName,
	}, &identity); err != nil {
		if apierrors.IsNotFound(err) {
			resolved.PendingReason = "IdentityNotReady"
			resolved.PendingMessage = fmt.Sprintf("%s not found", resolved.SourceDescription())
			return resolved, true, nil
		}
		return ResolvedIdentity{}, false, err
	}

	readyCond := meta.FindStatusCondition(identity.Status.Conditions, string(identityv1alpha1.ConditionReady))
	if readyCond == nil || readyCond.Status != metav1.ConditionTrue {
		resolved.PendingReason = "IdentityNotReady"
		resolved.PendingMessage = fmt.Sprintf("%s is not ready", resolved.SourceDescription())
		return resolved, true, nil
	}

	if identity.Status.ManagedIdentityName == nil || *identity.Status.ManagedIdentityName == "" {
		resolved.PendingReason = "IdentityNotReady"
		resolved.PendingMessage = fmt.Sprintf("%s is missing status.managedIdentityName", resolved.SourceDescription())
		return resolved, true, nil
	}
	if identity.Status.PrincipalID == nil || *identity.Status.PrincipalID == "" {
		resolved.PendingReason = "IdentityNotReady"
		resolved.PendingMessage = fmt.Sprintf("%s is missing status.principalId", resolved.SourceDescription())
		return resolved, true, nil
	}

	resolved.PrincipalID = *identity.Status.PrincipalID
	return resolved, false, nil
}

func resolveServiceAccount(ctx context.Context, c client.Reader, namespace, serviceAccountName string) (ResolvedIdentity, bool, error) {
	serviceAccountName = strings.TrimSpace(serviceAccountName)
	resolved := ResolvedIdentity{
		SourceKind:         IdentitySourceServiceAccount,
		SourceName:         serviceAccountName,
		AuthReferenceName:  serviceAccountName,
		ServiceAccountName: serviceAccountName,
	}

	var serviceAccount corev1.ServiceAccount
	if err := c.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      serviceAccountName,
	}, &serviceAccount); err != nil {
		if apierrors.IsNotFound(err) {
			resolved.PendingReason = "IdentityNotReady"
			resolved.PendingMessage = fmt.Sprintf("%s not found", resolved.SourceDescription())
			return resolved, true, nil
		}
		return ResolvedIdentity{}, false, err
	}

	clientID := strings.TrimSpace(serviceAccount.Annotations[ServiceAccountClientIDAnnotation])
	if clientID == "" {
		resolved.PendingReason = "IdentityNotReady"
		resolved.PendingMessage = fmt.Sprintf("%s is missing annotation %q", resolved.SourceDescription(), ServiceAccountClientIDAnnotation)
		return resolved, true, nil
	}

	principalID := strings.TrimSpace(serviceAccount.Annotations[ServiceAccountPrincipalIDAnnotation])
	if principalID == "" {
		resolved.PendingReason = "IdentityNotReady"
		resolved.PendingMessage = fmt.Sprintf("%s is missing annotation %q", resolved.SourceDescription(), ServiceAccountPrincipalIDAnnotation)
		return resolved, true, nil
	}

	resolved.PrincipalID = principalID
	return resolved, false, nil
}
