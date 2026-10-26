package controller

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	servicebusv1 "github.com/Azure/azure-service-operator/v2/api/servicebus/v1api20240101"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/config"
)

const (
	testNamespace           = "default"
	productNamespace        = "product-team-a"
	testNamespaceResourceID = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-messaging" +
		"/providers/Microsoft.ServiceBus/namespaces/sb-messaging"
)

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

	It("admits a name at the length limit of a label value", func() {
		createQueue(newQueue(strings.Repeat("q", messagingv1alpha1.MaxQueueNameLength), nil))
	})

	It("rejects a name longer than a label value", func() {
		err := k8sClient.Create(ctx, newQueue(strings.Repeat("q", messagingv1alpha1.MaxQueueNameLength+1), nil))
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected an Invalid error, got %v", err)
		Expect(err.Error()).To(ContainSubstring("metadata.name must be at most 63 characters"))
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

func newReconciler() *QueueReconciler {
	return &QueueReconciler{
		Client: k8sClient,
		Scheme: k8sClient.Scheme(),
		Config: config.OperatorConfig{NamespaceResourceID: testNamespaceResourceID},
	}
}

func reconcileQueue(queue *messagingv1alpha1.Queue) *messagingv1alpha1.Queue {
	_, err := newReconciler().Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(queue)})
	Expect(err).NotTo(HaveOccurred())

	stored := &messagingv1alpha1.Queue{}
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), stored)).To(Succeed())
	return stored
}

func productQueue(name, queueName string) *messagingv1alpha1.Queue {
	return newQueue(name, func(q *messagingv1alpha1.Queue) {
		q.Namespace = productNamespace
		q.Spec.Name = queueName
	})
}

func expectCondition(queue *messagingv1alpha1.Queue, conditionType messagingv1alpha1.ConditionType,
	status metav1.ConditionStatus, reason string) {
	condition := meta.FindStatusCondition(queue.Status.Conditions, string(conditionType))
	Expect(condition).NotTo(BeNil(), "condition %s", conditionType)
	Expect(condition.Status).To(Equal(status), "condition %s", conditionType)
	Expect(condition.Reason).To(Equal(reason), "condition %s", conditionType)
	Expect(condition.ObservedGeneration).To(Equal(queue.Generation), "condition %s", conditionType)
}

var _ = Describe("Queue reconciler", func() {
	BeforeEach(func() {
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: productNamespace}}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, namespace))).To(Succeed())
	})

	It("returns cleanly for a Queue that no longer exists", func() {
		_, err := newReconciler().Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: productNamespace, Name: "missing"},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("reports a name without the namespace prefix and creates no NamespacesQueue", func() {
		queue := productQueue("foreign", "team-b.orders")
		createQueue(queue)

		stored := reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionFalse, "InvalidName")
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionFalse, "InvalidName")
		Expect(stored.Status.ObservedGeneration).To(Equal(stored.Generation))

		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), &servicebusv1.NamespacesQueue{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected NotFound, got %v", err)
	})

	It("applies a NamespacesQueue that the Queue controls and waits for ASO", func() {
		queue := productQueue("orders", "team-a.sync/orders")
		createQueue(queue)

		stored := reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionUnknown, "Reconciling")
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionUnknown, "Reconciling")

		var nsQueue servicebusv1.NamespacesQueue
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), &nsQueue)).To(Succeed())
		Expect(nsQueue.Spec.AzureName).To(Equal("team-a.sync~orders"))
		Expect(nsQueue.Spec.Owner.ARMID).To(Equal(testNamespaceResourceID))
		Expect(metav1.IsControlledBy(&nsQueue, queue)).To(BeTrue())
	})

	It("mirrors the ASO Ready condition and the resource ID of the queue", func() {
		queue := productQueue("invoices", "team-a.invoices")
		createQueue(queue)
		reconcileQueue(queue)

		var nsQueue servicebusv1.NamespacesQueue
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), &nsQueue)).To(Succeed())
		resourceID := testNamespaceResourceID + "/queues/team-a.invoices"
		nsQueue.Status.Id = &resourceID
		nsQueue.Status.Conditions = []conditions.Condition{{
			Type:               conditions.ConditionTypeReady,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: nsQueue.Generation,
			Reason:             conditions.ReasonSucceeded,
		}}
		Expect(k8sClient.Status().Update(ctx, &nsQueue)).To(Succeed())

		stored := reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionTrue, conditions.ReasonSucceeded)
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionTrue, ReasonReady)
		Expect(stored.Status.Name).To(Equal("team-a.invoices"))
		Expect(stored.Status.ResourceId).To(Equal(resourceID))
		Expect(stored.Status.ObservedGeneration).To(Equal(stored.Generation))
	})

	It("mirrors an ASO failure and replaces a reason the API server rejects", func() {
		queue := productQueue("rejected", "team-a.rejected")
		createQueue(queue)
		reconcileQueue(queue)

		var nsQueue servicebusv1.NamespacesQueue
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), &nsQueue)).To(Succeed())
		nsQueue.Status.Conditions = []conditions.Condition{{
			Type:               conditions.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			Severity:           conditions.ConditionSeverityError,
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: nsQueue.Generation,
			Reason:             "Bad-Code",
			Message:            strings.Repeat("m", 2*maxMessageLength),
		}}
		Expect(k8sClient.Status().Update(ctx, &nsQueue)).To(Succeed())

		stored := reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionFalse, ReasonQueueNotReady)
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionFalse, ReasonQueueNotReady)
		message := meta.FindStatusCondition(stored.Status.Conditions, string(messagingv1alpha1.ConditionQueueReady)).Message
		Expect(message).To(HavePrefix("Bad-Code: mmm"))
		Expect(message).To(HaveLen(maxMessageLength))
	})

	It("does not take over a NamespacesQueue the Queue does not control", func() {
		foreign := &servicebusv1.NamespacesQueue{
			ObjectMeta: metav1.ObjectMeta{Name: "foreign", Namespace: productNamespace},
			Spec: servicebusv1.NamespacesQueue_Spec{
				AzureName: "team-a.by-hand",
				Owner:     &genruntime.KnownResourceReference{ARMID: testNamespaceResourceID},
			},
		}
		Expect(k8sClient.Create(ctx, foreign)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, foreign)).To(Succeed()) })

		queue := productQueue("foreign", "team-a.foreign")
		createQueue(queue)
		_, err := newReconciler().Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(queue)})
		Expect(err).To(MatchError(ContainSubstring("not controlled by this Queue")))

		stored := &messagingv1alpha1.Queue{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), stored)).To(Succeed())
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionFalse, ReasonApplyFailed)

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(foreign), foreign)).To(Succeed())
		Expect(foreign.OwnerReferences).To(BeEmpty())
		Expect(foreign.Spec.AzureName).To(Equal("team-a.by-hand"))
	})

	It("records an apply error in the conditions", func() {
		queue := productQueue("broken", "team-a.broken")
		createQueue(queue)

		reconciler := newReconciler()
		reconciler.Config.NamespaceResourceID = ""
		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(queue)})
		Expect(err).To(HaveOccurred())

		stored := &messagingv1alpha1.Queue{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), stored)).To(Succeed())
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionFalse, ReasonApplyFailed)
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionFalse, ReasonApplyFailed)
	})
})
