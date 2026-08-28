// Package identityref resolves the owner identity that a DIS custom resource
// references in its spec: either an ApplicationIdentity or an annotated
// ServiceAccount in the same namespace. The DIS operators share this logic so
// their CRs behave the same way and report the same condition reasons.
package identityref

import (
	"context"
	"fmt"
	"strings"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ServiceAccountClientIDAnnotation holds the workload identity client ID on a ServiceAccount.
	ServiceAccountClientIDAnnotation = "azure.workload.identity/client-id"
	// ServiceAccountPrincipalIDAnnotation holds the Entra principal ID on a ServiceAccount.
	ServiceAccountPrincipalIDAnnotation = "dis.altinn.cloud/principal-id"

	// ReasonIdentityNotReady is the pending reason when the referenced identity is missing or not ready.
	ReasonIdentityNotReady = "IdentityNotReady"
	// ReasonInvalidSpec is the pending reason when the identity references on the spec are invalid.
	ReasonInvalidSpec = "InvalidSpec"
)

// IdentitySourceKind identifies the kind of resource that backs the owner identity.
type IdentitySourceKind string

const (
	// IdentitySourceApplicationIdentity marks an identity resolved from an ApplicationIdentity.
	IdentitySourceApplicationIdentity IdentitySourceKind = "ApplicationIdentity"
	// IdentitySourceServiceAccount marks an identity resolved from an annotated ServiceAccount.
	IdentitySourceServiceAccount IdentitySourceKind = "ServiceAccount"
)

// Source names the identity references declared on a DIS custom resource.
// Callers map their CR spec onto this struct; a nil field means the reference
// is not set on the spec.
type Source struct {
	// Namespace is the namespace of the CR and of the referenced objects.
	Namespace string
	// IdentityRefName is spec.identityRef.name, or nil when identityRef is not set.
	IdentityRefName *string
	// ServiceAccountRefName is spec.serviceAccountRef.name, or nil when serviceAccountRef is not set.
	ServiceAccountRefName *string
}

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
func ActiveAuthReferenceName(s Source) (string, error) {
	if s.IdentityRefName != nil && s.ServiceAccountRefName != nil {
		return "", fmt.Errorf("exactly one of identityRef or serviceAccountRef must be set")
	}

	switch {
	case s.ServiceAccountRefName != nil:
		name := strings.TrimSpace(*s.ServiceAccountRefName)
		if name == "" {
			return "", fmt.Errorf("serviceAccountRef.name must not be empty")
		}
		return name, nil
	case s.IdentityRefName != nil:
		name := strings.TrimSpace(*s.IdentityRefName)
		if name == "" {
			return "", fmt.Errorf("identityRef.name must not be empty")
		}
		return name, nil
	default:
		return "", fmt.Errorf("exactly one of identityRef or serviceAccountRef must be set")
	}
}

// ResolveOwnerIdentity resolves the active owner identity source.
// The bool return indicates whether the caller should requeue.
func ResolveOwnerIdentity(ctx context.Context, c client.Reader, s Source) (ResolvedIdentity, bool, error) {
	switch {
	case s.IdentityRefName != nil && s.ServiceAccountRefName != nil:
		return ResolvedIdentity{
			PendingReason:  ReasonInvalidSpec,
			PendingMessage: "exactly one of identityRef or serviceAccountRef must be set",
		}, true, nil
	case s.IdentityRefName != nil:
		return resolveApplicationIdentity(ctx, c, s.Namespace, *s.IdentityRefName)
	case s.ServiceAccountRefName != nil:
		return resolveServiceAccount(ctx, c, s.Namespace, *s.ServiceAccountRefName)
	default:
		return ResolvedIdentity{
			PendingReason:  ReasonInvalidSpec,
			PendingMessage: "exactly one of identityRef or serviceAccountRef must be set",
		}, true, nil
	}
}

func resolveApplicationIdentity(
	ctx context.Context,
	c client.Reader,
	namespace, identityName string,
) (ResolvedIdentity, bool, error) {
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
			resolved.PendingReason = ReasonIdentityNotReady
			resolved.PendingMessage = fmt.Sprintf("%s not found", resolved.SourceDescription())
			return resolved, true, nil
		}
		return ResolvedIdentity{}, false, err
	}

	readyCond := meta.FindStatusCondition(identity.Status.Conditions, string(identityv1alpha1.ConditionReady))
	if readyCond == nil || readyCond.Status != metav1.ConditionTrue {
		resolved.PendingReason = ReasonIdentityNotReady
		resolved.PendingMessage = fmt.Sprintf("%s is not ready", resolved.SourceDescription())
		return resolved, true, nil
	}

	if identity.Status.ManagedIdentityName == nil || *identity.Status.ManagedIdentityName == "" {
		resolved.PendingReason = ReasonIdentityNotReady
		resolved.PendingMessage = fmt.Sprintf("%s is missing status.managedIdentityName", resolved.SourceDescription())
		return resolved, true, nil
	}
	if identity.Status.PrincipalID == nil || *identity.Status.PrincipalID == "" {
		resolved.PendingReason = ReasonIdentityNotReady
		resolved.PendingMessage = fmt.Sprintf("%s is missing status.principalId", resolved.SourceDescription())
		return resolved, true, nil
	}

	resolved.PrincipalID = *identity.Status.PrincipalID
	return resolved, false, nil
}

func resolveServiceAccount(
	ctx context.Context,
	c client.Reader,
	namespace, serviceAccountName string,
) (ResolvedIdentity, bool, error) {
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
			resolved.PendingReason = ReasonIdentityNotReady
			resolved.PendingMessage = fmt.Sprintf("%s not found", resolved.SourceDescription())
			return resolved, true, nil
		}
		return ResolvedIdentity{}, false, err
	}

	clientID := strings.TrimSpace(serviceAccount.Annotations[ServiceAccountClientIDAnnotation])
	if clientID == "" {
		resolved.PendingReason = ReasonIdentityNotReady
		resolved.PendingMessage = fmt.Sprintf(
			"%s is missing annotation %q", resolved.SourceDescription(), ServiceAccountClientIDAnnotation)
		return resolved, true, nil
	}

	principalID := strings.TrimSpace(serviceAccount.Annotations[ServiceAccountPrincipalIDAnnotation])
	if principalID == "" {
		resolved.PendingReason = ReasonIdentityNotReady
		resolved.PendingMessage = fmt.Sprintf(
			"%s is missing annotation %q", resolved.SourceDescription(), ServiceAccountPrincipalIDAnnotation)
		return resolved, true, nil
	}

	resolved.PrincipalID = principalID
	return resolved, false, nil
}
