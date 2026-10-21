package messaging

import (
	"strings"
	"testing"
	"time"

	servicebusv1 "github.com/Azure/azure-service-operator/v2/api/servicebus/v1api20240101"
	"github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
)

const testNamespaceResourceID = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-messaging/providers/Microsoft.ServiceBus/namespaces/sb-messaging"

func testQueue(spec messagingv1alpha1.QueueSpec) *messagingv1alpha1.Queue {
	spec.Name = "team-a.app-one"
	return &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "app-one", Namespace: "product-team-a"},
		Spec:       spec,
	}
}

func wantNamespacesQueue(queueAnnotations map[string]string, spec servicebusv1.NamespacesQueue_Spec) *servicebusv1.NamespacesQueue {
	spec.AzureName = "team-a.app-one"
	spec.Owner = &genruntime.KnownResourceReference{ARMID: testNamespaceResourceID}
	return &servicebusv1.NamespacesQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-one",
			Namespace: "product-team-a",
			Labels: map[string]string{
				ManagedByLabel: ManagedByValue,
				QueueNameLabel: "app-one",
			},
			Annotations: queueAnnotations,
		},
		Spec: spec,
	}
}

func TestBuildNamespacesQueue(t *testing.T) {
	t.Parallel()

	detach := map[string]string{
		annotations.ReconcilePolicy: string(annotations.ReconcilePolicyDetachOnDelete),
	}

	tests := map[string]struct {
		spec messagingv1alpha1.QueueSpec
		want *servicebusv1.NamespacesQueue
	}{
		"unset fields keep the Azure defaults": {
			spec: messagingv1alpha1.QueueSpec{},
			want: wantNamespacesQueue(detach, servicebusv1.NamespacesQueue_Spec{
				RequiresSession:            ptr.To(false),
				RequiresDuplicateDetection: ptr.To(false),
			}),
		},
		"all fields": {
			spec: messagingv1alpha1.QueueSpec{
				LockDuration:                     &metav1.Duration{Duration: 30 * time.Second},
				MaxDeliveryCount:                 ptr.To[int32](10),
				DefaultMessageTimeToLive:         &metav1.Duration{Duration: 336 * time.Hour},
				MaxSizeInMegabytes:               ptr.To[int32](2048),
				DeadLetteringOnMessageExpiration: ptr.To(true),
				RequiresSession:                  true,
				RequiresDuplicateDetection:       true,
				DeletionPolicy:                   messagingv1alpha1.QueueDeletionPolicyRetain,
			},
			want: wantNamespacesQueue(detach, servicebusv1.NamespacesQueue_Spec{
				LockDuration:                     ptr.To("PT30S"),
				MaxDeliveryCount:                 ptr.To(10),
				DefaultMessageTimeToLive:         ptr.To("PT336H"),
				MaxSizeInMegabytes:               ptr.To(2048),
				DeadLetteringOnMessageExpiration: ptr.To(true),
				RequiresSession:                  ptr.To(true),
				RequiresDuplicateDetection:       ptr.To(true),
			}),
		},
		"delete policy lets ASO delete the Azure queue": {
			spec: messagingv1alpha1.QueueSpec{DeletionPolicy: messagingv1alpha1.QueueDeletionPolicyDelete},
			want: wantNamespacesQueue(nil, servicebusv1.NamespacesQueue_Spec{
				RequiresSession:            ptr.To(false),
				RequiresDuplicateDetection: ptr.To(false),
			}),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := BuildNamespacesQueue(testQueue(tt.spec), testNamespaceResourceID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("NamespacesQueue mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildNamespacesQueueErrors(t *testing.T) {
	t.Parallel()

	longName := testQueue(messagingv1alpha1.QueueSpec{})
	longName.Name = strings.Repeat("a", 64)

	tests := map[string]struct {
		queue               *messagingv1alpha1.Queue
		namespaceResourceID string
	}{
		"empty namespace resource ID": {queue: testQueue(messagingv1alpha1.QueueSpec{})},
		"name too long for a label":   {queue: longName, namespaceResourceID: testNamespaceResourceID},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got, err := BuildNamespacesQueue(tt.queue, tt.namespaceResourceID); err == nil {
				t.Fatalf("expected an error, got %v", got)
			}
		})
	}
}

func TestIsoDuration(t *testing.T) {
	t.Parallel()

	tests := map[time.Duration]string{
		0:                                  "PT0S",
		time.Nanosecond:                    "PT0.000000001S",
		100 * time.Millisecond:             "PT0.1S",
		1500 * time.Millisecond:            "PT1.5S",
		30 * time.Second:                   "PT30S",
		time.Minute:                        "PT1M",
		90 * time.Minute:                   "PT1H30M",
		time.Hour + time.Second:            "PT1H1S",
		time.Minute + 250*time.Millisecond: "PT1M0.25S",
		336 * time.Hour:                    "PT336H",
		-time.Minute:                       "-PT1M",
	}

	for d, want := range tests {
		t.Run(d.String(), func(t *testing.T) {
			t.Parallel()

			if got := isoDuration(d); got != want {
				t.Fatalf("isoDuration(%s) = %q, want %q", d, got, want)
			}
		})
	}
}
