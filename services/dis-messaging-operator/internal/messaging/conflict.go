package messaging

import (
	"cmp"
	"strings"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
)

// ReasonConflict is the QueueReady reason of a Queue whose queue name
// another Queue holds.
const ReasonConflict = "Conflict"

// NameKey returns the key under which two queue names are one Azure queue.
// Azure compares resource names without case, so "team.Orders" and
// "team.orders" have the same key. The Queue CRD does not allow "~", so
// the "/" that AzureName writes as "~" needs no mapping.
func NameKey(queueName string) string {
	return strings.ToLower(queueName)
}

// NameHolder returns the Queue that holds the queue name of queue: of queue
// and the Queues in others with the same NameKey, the oldest one. Of Queues
// created in the same second, the first by namespace and then by name wins.
// others can also hold queue itself and Queues with other names. NameHolder
// returns queue itself when queue holds the name. A Queue that is being
// deleted holds the name until it is gone, because ASO can still be
// deleting the Azure queue.
func NameHolder(queue *messagingv1alpha1.Queue, others []messagingv1alpha1.Queue) *messagingv1alpha1.Queue {
	holder := queue
	for i := range others {
		other := &others[i]
		if NameKey(other.Spec.Name) == NameKey(queue.Spec.Name) && cmp.Or(
			other.CreationTimestamp.Compare(holder.CreationTimestamp.Time),
			cmp.Compare(other.Namespace, holder.Namespace),
			cmp.Compare(other.Name, holder.Name),
		) < 0 {
			holder = other
		}
	}

	return holder
}
