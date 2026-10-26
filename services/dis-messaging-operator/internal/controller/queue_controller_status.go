package controller

import (
	"context"
	"regexp"
	"strings"

	servicebusv1 "github.com/Azure/azure-service-operator/v2/api/servicebus/v1api20240101"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
)

// Reasons of the Queue conditions. messaging.ReasonInvalidName is the other one.
const (
	// ReasonApplyFailed means the operator could not write the NamespacesQueue.
	ReasonApplyFailed = "ApplyFailed"
	// ReasonQueueNotReady replaces an ASO reason that is not a valid
	// condition reason.
	ReasonQueueNotReady = "QueueNotReady"
	// ReasonReady is the Ready reason when every condition is True.
	ReasonReady = "Ready"

	// maxMessageLength keeps a message far below the CRD limit of a
	// condition message.
	maxMessageLength = 1024
)

// reasonPattern is what the API server accepts as a condition reason.
var reasonPattern = regexp.MustCompile(`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`)

// failed records the apply error in the conditions and returns it, so the
// controller retries with backoff. Without it a Queue whose NamespacesQueue
// is rejected would show no condition at all.
func (r *QueueReconciler) failed(ctx context.Context, queue *messagingv1alpha1.Queue, err error) error {
	condition := newCondition(messagingv1alpha1.ConditionQueueReady, queue.Generation, metav1.ConditionFalse,
		ReasonApplyFailed, shortened(err.Error()))
	if statusErr := r.writeStatus(ctx, queue, nil, condition); statusErr != nil {
		logf.FromContext(ctx).Error(statusErr, "cannot record the apply error in the Queue status")
	}

	return err
}

// writeStatus sets QueueReady and the aggregated Ready condition. With a
// NamespacesQueue it also sets the queue name and the Azure resource ID.
func (r *QueueReconciler) writeStatus(
	ctx context.Context,
	queue *messagingv1alpha1.Queue,
	nsQueue *servicebusv1.NamespacesQueue,
	queueReady metav1.Condition,
) error {
	orig := queue.DeepCopy()

	meta.SetStatusCondition(&queue.Status.Conditions, queueReady)
	meta.SetStatusCondition(&queue.Status.Conditions, readyCondition(queue.Generation, queueReady))
	if nsQueue != nil {
		// Both come from the ASO status, so they report what Azure has and
		// stay empty until Azure has created the queue. A copy of spec.name
		// would say nothing new. The name is the ARM resource name that
		// messaging.AzureName builds, with "~" for "/"; apps keep using
		// spec.name on the data plane.
		queue.Status.Name = ptr.Deref(nsQueue.Status.Name, "")
		queue.Status.ResourceId = ptr.Deref(nsQueue.Status.Id, "")
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

// queueReadyCondition mirrors the ASO Ready condition of the NamespacesQueue.
// A Ready condition from an older generation does not count: ASO has not
// reconciled the current spec yet.
func queueReadyCondition(generation int64, nsQueue *servicebusv1.NamespacesQueue) metav1.Condition {
	for _, c := range nsQueue.Status.Conditions {
		if c.Type == conditions.ConditionTypeReady && c.ObservedGeneration == nsQueue.Generation {
			return mirrored(generation, c)
		}
	}

	return newCondition(messagingv1alpha1.ConditionQueueReady, generation, metav1.ConditionUnknown,
		conditions.ReasonReconciling.Name, "waiting for Azure Service Operator to reconcile the queue")
}

// mirrored copies an ASO condition into QueueReady. ASO uses the Azure error
// code as the reason. A code the API server does not accept as a reason would
// make the status write fail, so such a reason moves into the message.
func mirrored(generation int64, c conditions.Condition) metav1.Condition {
	reason, message := c.Reason, c.Message
	if len(reason) > maxMessageLength || !reasonPattern.MatchString(reason) {
		reason = ReasonQueueNotReady
		if c.Status == metav1.ConditionTrue {
			reason = conditions.ReasonSucceeded
		}
		message = strings.TrimPrefix(c.Reason+": "+c.Message, ": ")
	}

	return newCondition(messagingv1alpha1.ConditionQueueReady, generation, c.Status, reason, shortened(message))
}

func shortened(message string) string {
	if len(message) <= maxMessageLength {
		return message
	}

	return message[:maxMessageLength]
}

// readyCondition aggregates the conditions of a Queue. The first False
// condition wins, then the first Unknown one, and Ready takes its reason and
// message. Today the only input is QueueReady; AccessReady joins it when the
// operator grants roles.
func readyCondition(generation int64, inputs ...metav1.Condition) metav1.Condition {
	for _, status := range []metav1.ConditionStatus{metav1.ConditionFalse, metav1.ConditionUnknown} {
		for _, c := range inputs {
			if c.Status == status {
				return newCondition(messagingv1alpha1.ConditionReady, generation, status, c.Reason, c.Message)
			}
		}
	}

	return newCondition(messagingv1alpha1.ConditionReady, generation, metav1.ConditionTrue, ReasonReady,
		"the queue is ready")
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
