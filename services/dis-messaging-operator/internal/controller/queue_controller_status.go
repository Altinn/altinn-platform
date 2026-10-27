package controller

import (
	"context"
	"regexp"
	"strings"

	servicebusv1 "github.com/Azure/azure-service-operator/v2/api/servicebus/v1api20240101"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
)

// Reasons of the Queue conditions. messaging.ReasonInvalidName,
// messaging.ReasonRoleNotAllowed and identityref.ReasonIdentityNotReady are
// the others.
const (
	// ReasonApplyFailed means the operator could not write the
	// NamespacesQueue or a RoleAssignment.
	ReasonApplyFailed = "ApplyFailed"
	// ReasonQueueNotReady replaces an ASO reason of the queue that is not a
	// valid condition reason.
	ReasonQueueNotReady = "QueueNotReady"
	// ReasonAccessNotReady replaces an ASO reason of a role assignment that
	// is not a valid condition reason.
	ReasonAccessNotReady = "AccessNotReady"
	// ReasonWaitingForQueue means the operator grants no roles because it
	// could not apply the queue.
	ReasonWaitingForQueue = "WaitingForQueue"
	// ReasonReady is the Ready reason when every condition is True.
	ReasonReady = "Ready"

	// maxMessageLength keeps a message far below the CRD limit of a
	// condition message.
	maxMessageLength = 1024
)

// reasonPattern is what the API server accepts as a condition reason.
var reasonPattern = regexp.MustCompile(`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`)

// failed records the conditions of an apply error and returns the error, so
// the controller retries with backoff. Without it a Queue whose
// NamespacesQueue or RoleAssignment is rejected would show no condition for
// it.
func (r *QueueReconciler) failed(
	ctx context.Context,
	queue *messagingv1alpha1.Queue,
	fields func(*messagingv1alpha1.QueueStatus),
	queueReady, accessReady metav1.Condition,
	err error,
) error {
	if statusErr := r.writeStatus(ctx, queue, fields, queueReady, accessReady); statusErr != nil {
		logf.FromContext(ctx).Error(statusErr, "cannot record the apply error in the Queue status")
	}

	return err
}

// applyFailed is the condition of an apply error.
func applyFailed(conditionType messagingv1alpha1.ConditionType, generation int64, err error) metav1.Condition {
	return newCondition(conditionType, generation, metav1.ConditionFalse, ReasonApplyFailed, shortened(err.Error()))
}

// waitingForQueue is AccessReady when the operator did not get as far as the
// role assignments.
func waitingForQueue(generation int64) metav1.Condition {
	return newCondition(messagingv1alpha1.ConditionAccessReady, generation, metav1.ConditionUnknown,
		ReasonWaitingForQueue, "the operator grants the roles after it applies the queue")
}

// writeStatus sets QueueReady, AccessReady and the aggregated Ready
// condition, lets fields change the rest of the status, and patches the Queue
// when something changed. A nil fields keeps the queue name and the resource
// ID as they are.
func (r *QueueReconciler) writeStatus(
	ctx context.Context,
	queue *messagingv1alpha1.Queue,
	fields func(*messagingv1alpha1.QueueStatus),
	queueReady, accessReady metav1.Condition,
) error {
	orig := queue.DeepCopy()

	meta.SetStatusCondition(&queue.Status.Conditions, queueReady)
	meta.SetStatusCondition(&queue.Status.Conditions, accessReady)
	meta.SetStatusCondition(&queue.Status.Conditions, readyCondition(queue.Generation, queueReady, accessReady))
	if fields != nil {
		fields(&queue.Status)
	}
	queue.Status.ObservedGeneration = queue.Generation

	// The client sends an empty patch too, so skip the round trip ourselves.
	if equality.Semantic.DeepEqual(orig.Status, queue.Status) {
		return nil
	}
	// A merge patch carries no resourceVersion, so a concurrent edit of the
	// Queue does not turn into a conflict and a retry.
	return r.Status().Patch(ctx, queue, client.MergeFrom(orig))
}

// azureQueue records the queue name and the Azure resource ID of the applied
// NamespacesQueue. The name is the data-plane name, as apps use it, with
// "/". Azure stores the queue under the ARM name that messaging.AzureName
// builds, with "~" for "/", and that form is in the resource ID.
func azureQueue(name string, nsQueue *servicebusv1.NamespacesQueue) func(*messagingv1alpha1.QueueStatus) {
	return func(status *messagingv1alpha1.QueueStatus) {
		status.Name = name
		status.ResourceId = ptr.Deref(nsQueue.Status.Id, "")
	}
}

// noQueue clears the queue name and the Azure resource ID. A new namespace
// prefix can make a ready Queue invalid, and then both would be stale.
func noQueue(status *messagingv1alpha1.QueueStatus) {
	status.Name, status.ResourceId = "", ""
}

// queueReadyCondition mirrors the ASO Ready condition of the NamespacesQueue.
func queueReadyCondition(generation int64, nsQueue *servicebusv1.NamespacesQueue) metav1.Condition {
	return asoReady(messagingv1alpha1.ConditionQueueReady, ReasonQueueNotReady, generation, nsQueue,
		"waiting for Azure Service Operator to reconcile the queue")
}

// asoReady mirrors the ASO Ready condition of obj as conditionType. A Ready
// condition from an older generation does not count: ASO has not reconciled
// the current spec yet.
func asoReady(
	conditionType messagingv1alpha1.ConditionType,
	notReadyReason string,
	generation int64,
	obj genruntime.MetaObject,
	waiting string,
) metav1.Condition {
	for _, c := range obj.GetConditions() {
		if c.Type == conditions.ConditionTypeReady && c.ObservedGeneration == obj.GetGeneration() {
			return mirrored(conditionType, notReadyReason, generation, c)
		}
	}

	return newCondition(conditionType, generation, metav1.ConditionUnknown, conditions.ReasonReconciling.Name, waiting)
}

// mirrored copies an ASO condition into conditionType. ASO uses the Azure
// error code as the reason. A code the API server does not accept as a reason
// would make the status write fail, so such a reason moves into the message
// and notReadyReason replaces it.
func mirrored(
	conditionType messagingv1alpha1.ConditionType,
	notReadyReason string,
	generation int64,
	c conditions.Condition,
) metav1.Condition {
	reason, message := c.Reason, c.Message
	if len(reason) > maxMessageLength || !reasonPattern.MatchString(reason) {
		reason = notReadyReason
		if c.Status == metav1.ConditionTrue {
			reason = conditions.ReasonSucceeded
		}
		message = strings.TrimPrefix(c.Reason+": "+c.Message, ": ")
	}

	return newCondition(conditionType, generation, c.Status, reason, shortened(message))
}

func shortened(message string) string {
	if len(message) <= maxMessageLength {
		return message
	}

	return message[:maxMessageLength]
}

// readyCondition aggregates the conditions of a Queue. Ready takes the reason
// and message of the first condition that is not True.
func readyCondition(generation int64, inputs ...metav1.Condition) metav1.Condition {
	if c := firstNotTrue(inputs); c != nil {
		return newCondition(messagingv1alpha1.ConditionReady, generation, c.Status, c.Reason, c.Message)
	}

	return newCondition(messagingv1alpha1.ConditionReady, generation, metav1.ConditionTrue, ReasonReady,
		"the queue is ready")
}

// firstNotTrue returns the first False condition, then the first Unknown
// one, or nil when every condition is True.
func firstNotTrue(inputs []metav1.Condition) *metav1.Condition {
	for _, status := range []metav1.ConditionStatus{metav1.ConditionFalse, metav1.ConditionUnknown} {
		for i := range inputs {
			if inputs[i].Status == status {
				return &inputs[i]
			}
		}
	}

	return nil
}

func newCondition(
	conditionType messagingv1alpha1.ConditionType,
	generation int64,
	status metav1.ConditionStatus,
	reason, message string,
) metav1.Condition {
	return metav1.Condition{
		Type:               string(conditionType),
		Status:             status,
		ObservedGeneration: generation,
		Reason:             reason,
		Message:            message,
	}
}
