package messaging

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
)

func TestNameHolder(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	queueAt := func(namespace, name, queueName string, age time.Duration) messagingv1alpha1.Queue {
		queue := connectionQueue(namespace, name, queueName)
		queue.CreationTimestamp = metav1.NewTime(created.Add(-age))
		return queue
	}
	orders := queueAt("product-team-a", "orders", "team-a.orders", 0)
	older := queueAt("product-team-a", "older", "team-a.orders", time.Minute)
	deleting := older
	deleting.DeletionTimestamp = &metav1.Time{Time: created}

	tests := map[string]struct {
		others []messagingv1alpha1.Queue
		want   string
	}{
		"no other queue":         {want: "product-team-a/orders"},
		"queue itself in others": {others: []messagingv1alpha1.Queue{orders}, want: "product-team-a/orders"},
		"the oldest holds the name": {
			others: []messagingv1alpha1.Queue{older}, want: "product-team-a/older",
		},
		"a newer queue does not take the name": {
			others: []messagingv1alpha1.Queue{queueAt("product-team-a", "newer", "team-a.orders", -time.Minute)},
			want:   "product-team-a/orders",
		},
		"names compare without case": {
			others: []messagingv1alpha1.Queue{queueAt("product-team-a", "upper", "TEAM-A.Orders", time.Minute)},
			want:   "product-team-a/upper",
		},
		"a tie goes to the first namespace": {
			others: []messagingv1alpha1.Queue{queueAt("product-shared", "orders", "team-a.orders", 0)},
			want:   "product-shared/orders",
		},
		"a tie in one namespace goes to the first name": {
			others: []messagingv1alpha1.Queue{queueAt("product-team-a", "aaa", "team-a.orders", 0)},
			want:   "product-team-a/aaa",
		},
		"other names do not count": {
			others: []messagingv1alpha1.Queue{queueAt("product-team-a", "invoices", "team-a.invoices", time.Hour)},
			want:   "product-team-a/orders",
		},
		"a deleting queue holds the name until it is gone": {
			others: []messagingv1alpha1.Queue{deleting}, want: "product-team-a/older",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			queue := orders
			holder := NameHolder(&queue, tc.others)
			if got := holder.Namespace + "/" + holder.Name; got != tc.want {
				t.Errorf("NameHolder() = %s, want %s", got, tc.want)
			}
			if (tc.want == "product-team-a/orders") != (holder == &queue) {
				t.Errorf("NameHolder() must return queue itself exactly when queue holds the name")
			}
		})
	}
}
