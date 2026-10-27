package messaging

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
)

const (
	// ConnectionConfigMapName is the name of the ConfigMap with the
	// connection details of a namespace.
	ConnectionConfigMapName = "dis-messaging"
	// ResourceQuotaName is the name of the ResourceQuota that limits the
	// Queues of a namespace.
	ResourceQuotaName = "dis-messaging"

	// ComponentLabel tells apps which object holds the connection details.
	ComponentLabel = "messaging.dis.altinn.cloud/component"
	// ComponentConnection is the ComponentLabel value of the connection
	// ConfigMap.
	ComponentConnection = "connection"

	// EndpointKey is the ConfigMap key of the Service Bus host name.
	EndpointKey = "endpoint"

	queueKeyPrefix = "queue."
)

// QueueCountResource is the ResourceQuota object count of Queues.
var QueueCountResource = corev1.ResourceName("count/queues." + messagingv1alpha1.GroupVersion.Group)

// QueueKey returns the ConfigMap key of a Queue, "queue.<metadata.name>".
// Apps know the key when they write the Queue.
func QueueKey(queue *messagingv1alpha1.Queue) string {
	return queueKeyPrefix + queue.Name
}

// ConnectedQueues returns the Queues of a namespace that get a key in its
// connection ConfigMap: the Queues with a valid name that are not being
// deleted. A Queue with a finalizer stays after its deletion starts, and
// its key must go. The Conflict slice adds the next rule here: of two
// Queues with the same spec.name, only the oldest keeps its key. The order
// of queues is kept.
func ConnectedQueues(namespace string, annotations map[string]string, queues []messagingv1alpha1.Queue) []messagingv1alpha1.Queue {
	connected := make([]messagingv1alpha1.Queue, 0, len(queues))
	for i := range queues {
		queue := &queues[i]
		if queue.DeletionTimestamp.IsZero() && ValidateName(namespace, annotations, queue.Spec.Name) == nil {
			connected = append(connected, *queue)
		}
	}

	return connected
}

// BuildConnectionConfigMap returns the connection ConfigMap of a namespace.
// endpoint is the host name of the Azure Service Bus namespace. The
// ConfigMap has one key per Queue, with the queue name that apps use on the
// data plane: spec.name with its "/", not the ARM form of AzureName. Every
// Queue must be in the namespace, and every Queue gets a key.
// ConnectedQueues selects the Queues. The ConfigMap belongs to the
// namespace, so it has no owner reference. Data is a map, so the order of
// queues does not change the result.
func BuildConnectionConfigMap(namespace, endpoint string, queues []messagingv1alpha1.Queue) (*corev1.ConfigMap, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace must not be empty")
	}
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint must not be empty")
	}

	data := make(map[string]string, len(queues)+1)
	data[EndpointKey] = endpoint
	for i := range queues {
		queue := &queues[i]
		if queue.Namespace != namespace {
			return nil, fmt.Errorf("queue %s/%s is not in namespace %s", queue.Namespace, queue.Name, namespace)
		}
		data[QueueKey(queue)] = queue.Spec.Name
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConnectionConfigMapName,
			Namespace: namespace,
			Labels: map[string]string{
				ManagedByLabel: ManagedByValue,
				ComponentLabel: ComponentConnection,
			},
		},
		Data: data,
	}, nil
}

// BuildResourceQuota returns the ResourceQuota that allows at most
// maxQueues Queues in a namespace. The API server enforces it when a Queue
// is created. The quota belongs to the namespace, so it has no owner
// reference.
func BuildResourceQuota(namespace string, maxQueues int) (*corev1.ResourceQuota, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace must not be empty")
	}
	if maxQueues < 1 {
		return nil, fmt.Errorf("max queues must be at least 1, got %d", maxQueues)
	}

	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ResourceQuotaName,
			Namespace: namespace,
			Labels: map[string]string{
				ManagedByLabel: ManagedByValue,
			},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				QueueCountResource: *resource.NewQuantity(int64(maxQueues), resource.DecimalSI),
			},
		},
	}, nil
}
