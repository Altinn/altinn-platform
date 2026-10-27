package controller

import (
	"context"
	"slices"
	"strings"

	"github.com/Altinn/altinn-platform/services/dis-common/identityref"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	servicebusv1 "github.com/Azure/azure-service-operator/v2/api/servicebus/v1api20240101"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/messaging"
)

// reconcileAccess applies a RoleAssignment for each role of each principal
// that the operator can grant, deletes the RoleAssignments of the Queue that
// are no longer wanted, and returns AccessReady. The grants of the other
// principals go ahead when one principal cannot be granted.
func (r *QueueReconciler) reconcileAccess(
	ctx context.Context,
	queue *messagingv1alpha1.Queue,
	nsQueue *servicebusv1.NamespacesQueue,
) (metav1.Condition, error) {
	principals, notGranted, err := r.grantablePrincipals(ctx, queue)
	if err != nil {
		return metav1.Condition{}, err
	}
	grants, err := messaging.BuildRoleAssignments(queue, nsQueue, principals, r.Config.SubscriptionID)
	if err != nil {
		return metav1.Condition{}, err
	}

	desired := make(map[string]bool, len(grants))
	for _, grant := range grants {
		if err := r.apply(ctx, queue, grant); err != nil {
			return metav1.Condition{}, err
		}
		desired[grant.Name] = true
	}
	// The grant of an identity without a principal ID has an unknown name.
	// An identity that is not ready for a moment must keep its access, so no
	// grant goes until every identity is resolved.
	if notGranted == nil || notGranted.Reason != identityref.ReasonIdentityNotReady {
		if err := r.deleteStaleGrants(ctx, queue, desired); err != nil {
			return metav1.Condition{}, err
		}
	}

	if notGranted != nil {
		return *notGranted, nil
	}
	return accessReadyCondition(queue.Generation, grants), nil
}

// grantablePrincipals returns the principals of the Queue that the operator
// can grant now, and a False AccessReady condition for the others, or nil. An
// identityRef principal can be granted when its ApplicationIdentity is ready.
// IdentityNotReady wins over RoleNotAllowed, because only IdentityNotReady
// keeps the grants of the Queue.
func (r *QueueReconciler) grantablePrincipals(
	ctx context.Context,
	queue *messagingv1alpha1.Queue,
) ([]messaging.Principal, *metav1.Condition, error) {
	split := messaging.SplitPrincipals(queue)

	principals := split.Grantable
	var notReady []string
	for _, identity := range split.Identities {
		resolved, _, err := identityref.ResolveOwnerIdentity(ctx, r.Client, identityref.Source{
			Namespace:       identity.Ref.Namespace,
			IdentityRefName: &identity.Ref.Name,
		})
		if err != nil {
			return nil, nil, err
		}
		if resolved.IsPending() {
			notReady = append(notReady, identity.Field+": "+resolved.PendingMessage)
			continue
		}
		principals = append(principals, identity.Resolved(resolved.PrincipalID))
	}
	if len(notReady)+len(split.NotAllowed) == 0 {
		return principals, nil, nil
	}

	reason := messaging.ReasonRoleNotAllowed
	if len(notReady) > 0 {
		reason = identityref.ReasonIdentityNotReady
	}
	condition := newCondition(messagingv1alpha1.ConditionAccessReady, queue.Generation, metav1.ConditionFalse, reason,
		shortened(strings.Join(slices.Concat(notReady, split.NotAllowed), "; ")))

	return principals, &condition, nil
}

// deleteStaleGrants deletes the RoleAssignments of the Queue that are not in
// desired. The list selects the labels of the operator, so it never holds a
// RoleAssignment that someone else created, and a RoleAssignment that the
// Queue does not control stays.
func (r *QueueReconciler) deleteStaleGrants(
	ctx context.Context,
	queue *messagingv1alpha1.Queue,
	desired map[string]bool,
) error {
	var grants authorizationv1.RoleAssignmentList
	if err := r.List(ctx, &grants, client.InNamespace(queue.Namespace), client.MatchingLabels{
		messaging.ManagedByLabel: messaging.ManagedByValue,
		messaging.QueueNameLabel: queue.Name,
	}); err != nil {
		return err
	}

	for i := range grants.Items {
		grant := &grants.Items[i]
		if desired[grant.Name] || !metav1.IsControlledBy(grant, queue) || !grant.DeletionTimestamp.IsZero() {
			continue
		}
		if err := r.Delete(ctx, grant); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

// accessReadyCondition mirrors the ASO Ready conditions of the grants, as
// queueReadyCondition does for the queue. The first grant that is not ready
// decides, with its name in the message. A Queue without grants has
// AccessReady=True.
func accessReadyCondition(generation int64, grants []*authorizationv1.RoleAssignment) metav1.Condition {
	inputs := make([]metav1.Condition, 0, len(grants))
	for _, grant := range grants {
		c := asoReady(messagingv1alpha1.ConditionAccessReady, ReasonAccessNotReady, generation, grant,
			"waiting for Azure Service Operator to reconcile the role assignment")
		c.Message = shortened("RoleAssignment " + grant.Name + ": " + c.Message)
		inputs = append(inputs, c)
	}
	if c := firstNotTrue(inputs); c != nil {
		return *c
	}

	return newCondition(messagingv1alpha1.ConditionAccessReady, generation, metav1.ConditionTrue,
		conditions.ReasonSucceeded, "every principal has its role on the queue")
}
