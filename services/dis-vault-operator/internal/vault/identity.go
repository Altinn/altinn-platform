package vault

import (
	"context"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolvedIdentity contains owner identity values required for provisioning.
type ResolvedIdentity struct {
	Name        string
	PrincipalID string
}

// ResolveOwnerIdentity resolves an owner ApplicationIdentity for a Vault.
// The bool return indicates whether caller should requeue.
func ResolveOwnerIdentity(ctx context.Context, c client.Reader, namespace, identityName string) (ResolvedIdentity, bool, error) {
	var identity identityv1alpha1.ApplicationIdentity
	if err := c.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      identityName,
	}, &identity); err != nil {
		if apierrors.IsNotFound(err) {
			return ResolvedIdentity{}, true, nil
		}
		return ResolvedIdentity{}, false, err
	}

	readyCond := meta.FindStatusCondition(identity.Status.Conditions, string(identityv1alpha1.ConditionReady))
	if readyCond == nil || readyCond.Status != metav1.ConditionTrue {
		return ResolvedIdentity{}, true, nil
	}

	if identity.Status.ManagedIdentityName == nil || *identity.Status.ManagedIdentityName == "" {
		return ResolvedIdentity{}, true, nil
	}
	if identity.Status.PrincipalID == nil || *identity.Status.PrincipalID == "" {
		return ResolvedIdentity{}, true, nil
	}

	return ResolvedIdentity{
		Name:        *identity.Status.ManagedIdentityName,
		PrincipalID: *identity.Status.PrincipalID,
	}, false, nil
}
