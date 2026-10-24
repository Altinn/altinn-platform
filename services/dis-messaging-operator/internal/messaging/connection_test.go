package messaging

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
)

const testEndpoint = "sb-messaging.servicebus.windows.net"

func connectionQueue(namespace, name, queueName string) messagingv1alpha1.Queue {
	return messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       messagingv1alpha1.QueueSpec{Name: queueName},
	}
}

func wantConnectionConfigMap(data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dis-messaging",
			Namespace: "product-team-a",
			Labels: map[string]string{
				"messaging.dis.altinn.cloud/managed-by": "dis-messaging-operator",
				"messaging.dis.altinn.cloud/component":  "connection",
			},
		},
		Data: data,
	}
}

func TestBuildConnectionConfigMap(t *testing.T) {
	t.Parallel()

	appOne := connectionQueue("product-team-a", "app-one", "team-a.app-one")
	orders := connectionQueue("product-team-a", "orders", "team-a.orders/us/west")

	tests := map[string]struct {
		queues []messagingv1alpha1.Queue
		want   *corev1.ConfigMap
	}{
		"no queues": {
			want: wantConnectionConfigMap(map[string]string{"endpoint": testEndpoint}),
		},
		"one key per queue with the data-plane name": {
			queues: []messagingv1alpha1.Queue{appOne, orders},
			want: wantConnectionConfigMap(map[string]string{
				"endpoint":      testEndpoint,
				"queue.app-one": "team-a.app-one",
				"queue.orders":  "team-a.orders/us/west",
			}),
		},
		"order of the queues does not matter": {
			queues: []messagingv1alpha1.Queue{orders, appOne},
			want: wantConnectionConfigMap(map[string]string{
				"endpoint":      testEndpoint,
				"queue.app-one": "team-a.app-one",
				"queue.orders":  "team-a.orders/us/west",
			}),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := BuildConnectionConfigMap("product-team-a", testEndpoint, tt.queues)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("ConfigMap mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildConnectionConfigMapErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		namespace string
		endpoint  string
		queues    []messagingv1alpha1.Queue
	}{
		"empty namespace": {endpoint: testEndpoint},
		"empty endpoint":  {namespace: "product-team-a"},
		"queue in another namespace": {
			namespace: "product-team-a",
			endpoint:  testEndpoint,
			queues:    []messagingv1alpha1.Queue{connectionQueue("product-team-b", "app-one", "team-b.app-one")},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got, err := BuildConnectionConfigMap(tt.namespace, tt.endpoint, tt.queues); err == nil {
				t.Fatalf("expected an error, got %v", got)
			}
		})
	}
}

func TestBuildResourceQuota(t *testing.T) {
	t.Parallel()

	got, err := BuildResourceQuota("product-team-a", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dis-messaging",
			Namespace: "product-team-a",
			Labels:    map[string]string{"messaging.dis.altinn.cloud/managed-by": "dis-messaging-operator"},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				"count/queues.messaging.dis.altinn.cloud": resource.MustParse("50"),
			},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("ResourceQuota mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildResourceQuotaErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		namespace string
		maxQueues int
	}{
		"empty namespace": {maxQueues: 50},
		"zero queues":     {namespace: "product-team-a"},
		"negative queues": {namespace: "product-team-a", maxQueues: -1},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got, err := BuildResourceQuota(tt.namespace, tt.maxQueues); err == nil {
				t.Fatalf("expected an error, got %v", got)
			}
		})
	}
}
