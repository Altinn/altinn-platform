package redis

import (
	"context"
	"fmt"
	"strings"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	redisv1alpha1 "github.com/Altinn/altinn-platform/services/dis-redis-operator/api/v1alpha1"
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
	identityNotReadyReason              = "IdentityNotReady"
)

// IdentitySourceKind identifies the kind of resource that backs the owner identity.
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

// IsPending reports whether the identity resolution is still waiting on dependencies.
func (r ResolvedIdentity) IsPending() bool {
	return r.PendingReason != ""
}

// SourceDescription returns a human-readable description of the identity source.
func (r ResolvedIdentity) SourceDescription() string {
	if r.SourceKind == "" || r.SourceName == "" {
		return "identity source"
	}
	return fmt.Sprintf("%s %q", r.SourceKind, r.SourceName)
}

// ActiveAuthReferenceName returns the name of the active identity reference.
func ActiveAuthReferenceName(r *redisv1alpha1.Redis) (string, error) {
	if r == nil {
		return "", fmt.Errorf("redis must not be nil")
	}

	if r.Spec.IdentityRef != nil && r.Spec.ServiceAccountRef != nil {
		return "", fmt.Errorf("exactly one of identityRef or serviceAccountRef must be set")
	}

	switch {
	case r.Spec.ServiceAccountRef != nil:
		name := strings.TrimSpace(r.Spec.ServiceAccountRef.Name)
		if name == "" {
			return "", fmt.Errorf("serviceAccountRef.name must not be empty")
		}
		return name, nil
	case r.Spec.IdentityRef != nil:
		name := strings.TrimSpace(r.Spec.IdentityRef.Name)
		if name == "" {
			return "", fmt.Errorf("identityRef.name must not be empty")
		}
		return name, nil
	default:
		return "", fmt.Errorf("exactly one of identityRef or serviceAccountRef must be set")
	}
}

// ResolveOwnerIdentity resolves the active owner identity source for a Redis CR.
// The bool return indicates whether the caller should requeue.
func ResolveOwnerIdentity(ctx context.Context, c client.Reader, r *redisv1alpha1.Redis) (ResolvedIdentity, bool, error) {
	if r == nil {
		return ResolvedIdentity{}, false, fmt.Errorf("redis must not be nil")
	}

	switch {
	case r.Spec.IdentityRef != nil && r.Spec.ServiceAccountRef != nil:
		return ResolvedIdentity{
			PendingReason:  "InvalidSpec",
			PendingMessage: "exactly one of identityRef or serviceAccountRef must be set",
		}, true, nil
	case r.Spec.IdentityRef != nil:
		return resolveApplicationIdentity(ctx, c, r.Namespace, r.Spec.IdentityRef.Name)
	case r.Spec.ServiceAccountRef != nil:
		return resolveServiceAccount(ctx, c, r.Namespace, r.Spec.ServiceAccountRef.Name)
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
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: identityName}, &identity); err != nil {
		if apierrors.IsNotFound(err) {
			resolved.PendingReason = identityNotReadyReason
			resolved.PendingMessage = fmt.Sprintf("%s not found", resolved.SourceDescription())
			return resolved, true, nil
		}
		return ResolvedIdentity{}, false, err
	}

	readyCond := meta.FindStatusCondition(identity.Status.Conditions, string(identityv1alpha1.ConditionReady))
	if readyCond == nil || readyCond.Status != metav1.ConditionTrue {
		resolved.PendingReason = identityNotReadyReason
		resolved.PendingMessage = fmt.Sprintf("%s is not ready", resolved.SourceDescription())
		return resolved, true, nil
	}

	if identity.Status.ManagedIdentityName == nil || *identity.Status.ManagedIdentityName == "" {
		resolved.PendingReason = identityNotReadyReason
		resolved.PendingMessage = fmt.Sprintf("%s is missing status.managedIdentityName", resolved.SourceDescription())
		return resolved, true, nil
	}
	if identity.Status.PrincipalID == nil || *identity.Status.PrincipalID == "" {
		resolved.PendingReason = identityNotReadyReason
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

	var sa corev1.ServiceAccount
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: serviceAccountName}, &sa); err != nil {
		if apierrors.IsNotFound(err) {
			resolved.PendingReason = identityNotReadyReason
			resolved.PendingMessage = fmt.Sprintf("%s not found", resolved.SourceDescription())
			return resolved, true, nil
		}
		return ResolvedIdentity{}, false, err
	}

	clientID := strings.TrimSpace(sa.Annotations[ServiceAccountClientIDAnnotation])
	if clientID == "" {
		resolved.PendingReason = identityNotReadyReason
		resolved.PendingMessage = fmt.Sprintf("%s is missing annotation %q", resolved.SourceDescription(), ServiceAccountClientIDAnnotation)
		return resolved, true, nil
	}

	principalID := strings.TrimSpace(sa.Annotations[ServiceAccountPrincipalIDAnnotation])
	if principalID == "" {
		resolved.PendingReason = identityNotReadyReason
		resolved.PendingMessage = fmt.Sprintf("%s is missing annotation %q", resolved.SourceDescription(), ServiceAccountPrincipalIDAnnotation)
		return resolved, true, nil
	}

	resolved.PrincipalID = principalID
	return resolved, false, nil
}
