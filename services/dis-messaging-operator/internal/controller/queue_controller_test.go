package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
)

const testNamespace = "default"

func newQueue(name string, mutate func(*messagingv1alpha1.Queue)) *messagingv1alpha1.Queue {
	queue := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
		Spec: messagingv1alpha1.QueueSpec{
			Name: "team-a." + name,
		},
	}
	if mutate != nil {
		mutate(queue)
	}

	return queue
}

func createQueue(queue *messagingv1alpha1.Queue) {
	Expect(k8sClient.Create(ctx, queue)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(ctx, queue)).To(Succeed())
	})
}

func identityPrincipal(role messagingv1alpha1.QueueAccessRole, name string) messagingv1alpha1.QueueAccessPrincipalSpec {
	return messagingv1alpha1.QueueAccessPrincipalSpec{
		Role:        role,
		IdentityRef: &messagingv1alpha1.ApplicationIdentityRef{Name: name},
	}
}

var _ = Describe("Queue CRD schema", func() {
	It("admits a minimal Queue and applies the defaults", func() {
		createQueue(newQueue("defaults", nil))

		var stored messagingv1alpha1.Queue
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "defaults"}, &stored)).To(Succeed())
		Expect(stored.Spec.DeletionPolicy).To(Equal(messagingv1alpha1.QueueDeletionPolicyRetain))
		Expect(stored.Spec.RequiresSession).To(BeFalse())
		Expect(stored.Spec.RequiresDuplicateDetection).To(BeFalse())
	})

	It("admits a Queue with every field and every principal source", func() {
		createQueue(newQueue("explicit", func(q *messagingv1alpha1.Queue) {
			q.Spec.Name = "team-a.sync/requests_v2"
			q.Spec.LockDuration = &metav1.Duration{Duration: time.Minute}
			q.Spec.MaxDeliveryCount = ptr.To[int32](10)
			q.Spec.DefaultMessageTimeToLive = &metav1.Duration{Duration: 348 * time.Hour}
			q.Spec.MaxSizeInMegabytes = ptr.To[int32](5120)
			q.Spec.DeadLetteringOnMessageExpiration = ptr.To(true)
			q.Spec.RequiresSession = true
			q.Spec.RequiresDuplicateDetection = true
			q.Spec.DeletionPolicy = messagingv1alpha1.QueueDeletionPolicyDelete
			q.Spec.Access = &messagingv1alpha1.QueueAccessSpec{Principals: []messagingv1alpha1.QueueAccessPrincipalSpec{
				identityPrincipal(messagingv1alpha1.QueueAccessRoleSender, "app-one"),
				{
					Role:        messagingv1alpha1.QueueAccessRoleConsumer,
					IdentityRef: &messagingv1alpha1.ApplicationIdentityRef{Name: "app-two", Namespace: "team-b"},
				},
				{
					Role:  messagingv1alpha1.QueueAccessRoleReceiver,
					Group: &messagingv1alpha1.QueueGroupPrincipalSpec{Name: "group-one", PrincipalId: "group-id"},
				},
				{
					Role:             messagingv1alpha1.QueueAccessRoleSender,
					ServicePrincipal: &messagingv1alpha1.QueueServicePrincipalSpec{Name: "sp-one", PrincipalId: "sp-id"},
				},
			}}
		}))
	})

	It("admits a lockDuration of exactly 5m", func() {
		createQueue(newQueue("lock-max", func(q *messagingv1alpha1.Queue) {
			q.Spec.LockDuration = &metav1.Duration{Duration: 5 * time.Minute}
		}))
	})

	It("rejects a duration with a unit Kubernetes does not know", func() {
		queue := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": messagingv1alpha1.GroupVersion.String(),
			"kind":       "Queue",
			"metadata":   map[string]any{"name": "days", "namespace": testNamespace},
			"spec": map[string]any{
				"name":                     "team-a.days",
				"defaultMessageTimeToLive": "14d",
			},
		}}
		err := k8sClient.Create(ctx, queue)
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected an Invalid error, got %v", err)
		Expect(err.Error()).To(ContainSubstring("spec.defaultMessageTimeToLive"))
	})

	DescribeTable("rejects an invalid Queue",
		func(mutate func(*messagingv1alpha1.Queue), message string) {
			err := k8sClient.Create(ctx, newQueue("invalid", mutate))
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected an Invalid error, got %v", err)
			Expect(err.Error()).To(ContainSubstring(message))
		},
		Entry("a name with a space", func(q *messagingv1alpha1.Queue) {
			q.Spec.Name = "team-a.bad name"
		}, "spec.name"),
		Entry("a name that ends with a dot", func(q *messagingv1alpha1.Queue) {
			q.Spec.Name = "team-a."
		}, "spec.name"),
		Entry("a lockDuration above 5m", func(q *messagingv1alpha1.Queue) {
			q.Spec.LockDuration = &metav1.Duration{Duration: 5*time.Minute + time.Second}
		}, "lockDuration must be more than 0s and at most 5m"),
		Entry("a lockDuration of zero", func(q *messagingv1alpha1.Queue) {
			q.Spec.LockDuration = &metav1.Duration{}
		}, "lockDuration must be more than 0s and at most 5m"),
		Entry("a time to live of zero", func(q *messagingv1alpha1.Queue) {
			q.Spec.DefaultMessageTimeToLive = &metav1.Duration{}
		}, "defaultMessageTimeToLive must be more than 0s"),
		Entry("a size Azure does not offer", func(q *messagingv1alpha1.Queue) {
			q.Spec.MaxSizeInMegabytes = ptr.To[int32](1000)
		}, "spec.maxSizeInMegabytes"),
		Entry("an unknown role", func(q *messagingv1alpha1.Queue) {
			q.Spec.Access = &messagingv1alpha1.QueueAccessSpec{Principals: []messagingv1alpha1.QueueAccessPrincipalSpec{
				identityPrincipal("admin", "app-one"),
			}}
		}, "spec.access.principals[0].role"),
		Entry("a principal without a source", func(q *messagingv1alpha1.Queue) {
			q.Spec.Access = &messagingv1alpha1.QueueAccessSpec{Principals: []messagingv1alpha1.QueueAccessPrincipalSpec{
				{Role: messagingv1alpha1.QueueAccessRoleSender},
			}}
		}, "Provide exactly one principal source"),
		Entry("a principal with two sources", func(q *messagingv1alpha1.Queue) {
			principal := identityPrincipal(messagingv1alpha1.QueueAccessRoleSender, "app-one")
			principal.Group = &messagingv1alpha1.QueueGroupPrincipalSpec{Name: "group-one", PrincipalId: "group-id"}
			q.Spec.Access = &messagingv1alpha1.QueueAccessSpec{Principals: []messagingv1alpha1.QueueAccessPrincipalSpec{principal}}
		}, "Provide exactly one principal source"),
	)

	DescribeTable("rejects a change to a field Azure fixes at creation",
		func(mutate func(*messagingv1alpha1.Queue), message string) {
			queue := newQueue("immutable", nil)
			createQueue(queue)

			mutate(queue)
			err := k8sClient.Update(ctx, queue)
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected an Invalid error, got %v", err)
			Expect(err.Error()).To(ContainSubstring(message))
		},
		Entry("name", func(q *messagingv1alpha1.Queue) { q.Spec.Name = "team-a.renamed" }, "name is immutable"),
		Entry("requiresSession", func(q *messagingv1alpha1.Queue) {
			q.Spec.RequiresSession = true
		}, "requiresSession is immutable"),
		Entry("requiresDuplicateDetection", func(q *messagingv1alpha1.Queue) {
			q.Spec.RequiresDuplicateDetection = true
		}, "requiresDuplicateDetection is immutable"),
	)
})

var _ = Describe("Queue reconciler", func() {
	It("reconciles an existing Queue without error", func() {
		createQueue(newQueue("reconcile", nil))

		reconciler := &QueueReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "reconcile"},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns cleanly for a Queue that no longer exists", func() {
		reconciler := &QueueReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "missing"},
		})
		Expect(err).NotTo(HaveOccurred())
	})
})
