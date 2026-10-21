package messaging

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	servicebusv1 "github.com/Azure/azure-service-operator/v2/api/servicebus/v1api20240101"
	"github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
)

const (
	// ManagedByLabel marks objects this operator manages.
	ManagedByLabel = "messaging.dis.altinn.cloud/managed-by"
	// ManagedByValue is the value of ManagedByLabel.
	ManagedByValue = "dis-messaging-operator"
	// QueueNameLabel carries the name of the owning Queue resource.
	QueueNameLabel = "messaging.dis.altinn.cloud/queue"
)

// NamespacesQueueName returns the name of the NamespacesQueue for a Queue.
// Both live in the same namespace, so the Queue name is enough.
func NamespacesQueueName(queue *messagingv1alpha1.Queue) string {
	return queue.Name
}

// AzureName returns the ARM resource name of the Azure queue. A queue name
// can contain "/" as a path separator, but ARM does not allow "/" in a
// resource name, so ARM writes "~" instead. ASO puts the name in the ARM
// path as it is. See
// https://learn.microsoft.com/azure/service-bus-messaging/service-bus-resource-manager-overview
// The Queue CRD does not allow "~" in a name, so the mapping is unambiguous.
func AzureName(queueName string) string {
	return strings.ReplaceAll(queueName, "/", "~")
}

// BuildNamespacesQueue returns the ASO NamespacesQueue for a Queue.
// namespaceResourceID is the ARM ID of the Service Bus namespace that owns
// the queue. Fields the Queue leaves out keep the Azure defaults.
func BuildNamespacesQueue(queue *messagingv1alpha1.Queue, namespaceResourceID string) (*servicebusv1.NamespacesQueue, error) {
	if namespaceResourceID == "" {
		return nil, fmt.Errorf("namespace resource ID must not be empty")
	}
	if errs := validation.IsValidLabelValue(queue.Name); len(errs) > 0 {
		return nil, fmt.Errorf("queue name %q is not a valid label value: %s", queue.Name, strings.Join(errs, "; "))
	}

	spec := queue.Spec
	nsQueue := &servicebusv1.NamespacesQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NamespacesQueueName(queue),
			Namespace: queue.Namespace,
			Labels: map[string]string{
				ManagedByLabel: ManagedByValue,
				QueueNameLabel: queue.Name,
			},
		},
		Spec: servicebusv1.NamespacesQueue_Spec{
			AzureName:                  AzureName(spec.Name),
			Owner:                      &genruntime.KnownResourceReference{ARMID: namespaceResourceID},
			RequiresSession:            ptr.To(spec.RequiresSession),
			RequiresDuplicateDetection: ptr.To(spec.RequiresDuplicateDetection),
		},
	}

	if spec.LockDuration != nil {
		nsQueue.Spec.LockDuration = ptr.To(isoDuration(spec.LockDuration.Duration))
	}
	if spec.DefaultMessageTimeToLive != nil {
		nsQueue.Spec.DefaultMessageTimeToLive = ptr.To(isoDuration(spec.DefaultMessageTimeToLive.Duration))
	}
	if spec.MaxDeliveryCount != nil {
		nsQueue.Spec.MaxDeliveryCount = ptr.To(int(*spec.MaxDeliveryCount))
	}
	if spec.MaxSizeInMegabytes != nil {
		nsQueue.Spec.MaxSizeInMegabytes = ptr.To(int(*spec.MaxSizeInMegabytes))
	}
	if spec.DeadLetteringOnMessageExpiration != nil {
		nsQueue.Spec.DeadLetteringOnMessageExpiration = ptr.To(*spec.DeadLetteringOnMessageExpiration)
	}

	// Retain is the default, so an empty policy also keeps the Azure queue.
	if spec.DeletionPolicy != messagingv1alpha1.QueueDeletionPolicyDelete {
		nsQueue.Annotations = map[string]string{
			annotations.ReconcilePolicy: string(annotations.ReconcilePolicyDetachOnDelete),
		}
	}

	return nsQueue, nil
}

// isoDuration formats d as an ISO 8601 duration, for example PT1M or
// PT336H. Fractions of a second stay in the seconds part, for example
// PT0.5S. Neither the standard library nor the Azure modules export a
// formatter. Hours are the largest unit here. Azure also accepts days
// (P14D), and it stores durations in its own form, so the spec string
// never has to match what Azure reports.
func isoDuration(d time.Duration) string {
	if d < 0 {
		return "-" + isoDuration(-d)
	}
	if d == 0 {
		return "PT0S"
	}

	var b strings.Builder
	b.WriteString("PT")
	if h := d / time.Hour; h > 0 {
		b.WriteString(strconv.FormatInt(int64(h), 10) + "H")
	}
	if m := d % time.Hour / time.Minute; m > 0 {
		b.WriteString(strconv.FormatInt(int64(m), 10) + "M")
	}
	if s := d % time.Minute; s > 0 {
		seconds := strconv.FormatInt(int64(s/time.Second), 10)
		if ns := s % time.Second; ns > 0 {
			seconds += strings.TrimRight(fmt.Sprintf(".%09d", ns), "0")
		}
		b.WriteString(seconds + "S")
	}
	return b.String()
}
