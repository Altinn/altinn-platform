package controller

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Altinn/altinn-platform/services/dis-common/identityref"
	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	servicebusv1 "github.com/Azure/azure-service-operator/v2/api/servicebus/v1api20240101"
	"github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
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
	"sigs.k8s.io/controller-runtime/pkg/event"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/messaging"
)

const (
	testNamespace           = "default"
	productNamespace        = "product-team-a"
	testSubscriptionID      = "00000000-0000-0000-0000-000000000000"
	testNamespaceResourceID = "/subscriptions/" + testSubscriptionID + "/resourceGroups/rg-messaging" +
		"/providers/Microsoft.ServiceBus/namespaces/sb-messaging"
	// Role definition IDs of Azure Service Bus Data Sender and Data Receiver.
	senderRoleID   = "69a216fc-b8fb-44d8-bc22-1f3c2cd27a39"
	receiverRoleID = "4f6d3b9b-027b-4f4c-9142-0e5a2a2247e0"
	// pollerAnnotation is what ASO writes on every poll of a running Azure
	// operation. ASO keeps the constant internal.
	pollerAnnotation = "serviceoperator.azure.com/poller-resume-token"
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
		Config: config.OperatorConfig{SubscriptionID: testSubscriptionID, NamespaceResourceID: testNamespaceResourceID},
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

// createProductNamespace creates the product namespace once. envtest cannot
// delete a namespace, so every spec shares it.
func createProductNamespace() {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: productNamespace}}
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, namespace))).To(Succeed())
}

// setPrefix sets the prefix annotation of a namespace for the rest of the
// spec. Namespaces outlive the spec, because envtest cannot delete them, so
// the cleanup removes only that annotation.
func setPrefix(name, prefix string) {
	patch := func(change func(map[string]string)) {
		var namespace corev1.Namespace
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, &namespace)).To(Succeed())
		orig := namespace.DeepCopy()
		if namespace.Annotations == nil {
			namespace.Annotations = map[string]string{}
		}
		change(namespace.Annotations)
		Expect(k8sClient.Patch(ctx, &namespace, client.MergeFrom(orig))).To(Succeed())
	}
	patch(func(a map[string]string) { a[messaging.PrefixAnnotation] = prefix })
	DeferCleanup(func() {
		patch(func(a map[string]string) { delete(a, messaging.PrefixAnnotation) })
	})
}

// reportReady writes, as ASO would, Ready=True for the current generation of
// the NamespacesQueue of queue, and resourceID as its Azure ID.
func reportReady(queue *messagingv1alpha1.Queue, resourceID string) {
	var nsQueue servicebusv1.NamespacesQueue
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), &nsQueue)).To(Succeed())
	nsQueue.Status.Id = &resourceID
	nsQueue.Status.Conditions = []conditions.Condition{{
		Type:               conditions.ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: nsQueue.Generation,
		Reason:             conditions.ReasonSucceeded,
	}}
	Expect(k8sClient.Status().Update(ctx, &nsQueue)).To(Succeed())
}

var _ = Describe("Queue reconciler", func() {
	BeforeEach(createProductNamespace)

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
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionFalse, messaging.ReasonInvalidName)
		expectCondition(stored, messagingv1alpha1.ConditionAccessReady, metav1.ConditionUnknown, ReasonWaitingForQueue)
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionFalse, messaging.ReasonInvalidName)
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
		Expect(stored.Status.Name).To(Equal("team-a.sync/orders"))
		Expect(stored.Status.ResourceId).To(BeEmpty())

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
		resourceID := testNamespaceResourceID + "/queues/team-a.invoices"
		reportReady(queue, resourceID)

		stored := reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionTrue, conditions.ReasonSucceeded)
		expectCondition(stored, messagingv1alpha1.ConditionAccessReady, metav1.ConditionTrue, conditions.ReasonSucceeded)
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

	It("reports InvalidName in a namespace without a product prefix and without the annotation", func() {
		queue := newQueue("unprefixed", nil)
		createQueue(queue)

		stored := reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionFalse, messaging.ReasonInvalidName)
		message := meta.FindStatusCondition(stored.Status.Conditions, string(messagingv1alpha1.ConditionQueueReady)).Message
		Expect(message).To(ContainSubstring("has no queue name prefix"))
	})

	It("clears the name and the resource ID when a new namespace prefix makes a ready Queue invalid", func() {
		queue := productQueue("renamed", "team-a.renamed")
		createQueue(queue)
		reconcileQueue(queue)
		reportReady(queue, testNamespaceResourceID+"/queues/team-a.renamed")

		stored := reconcileQueue(queue)
		Expect(stored.Status.Name).To(Equal("team-a.renamed"))
		Expect(stored.Status.ResourceId).NotTo(BeEmpty())

		setPrefix(productNamespace, "team-b.")
		stored = reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionFalse, messaging.ReasonInvalidName)
		Expect(stored.Status.Name).To(BeEmpty())
		Expect(stored.Status.ResourceId).To(BeEmpty())
	})

	It("creates the NamespacesQueue of an invalid Queue once the namespace gets the prefix annotation", func() {
		queue := newQueue("annotated", nil)
		createQueue(queue)
		stored := reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionFalse, messaging.ReasonInvalidName)

		setPrefix(testNamespace, "team-a.")
		stored = reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionUnknown, conditions.ReasonReconciling.Name)
		Expect(stored.Status.Name).To(Equal("team-a.annotated"))
		var nsQueue servicebusv1.NamespacesQueue
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), &nsQueue)).To(Succeed())
		Expect(nsQueue.Spec.AzureName).To(Equal("team-a.annotated"))
	})

	It("waits for ASO again when the NamespacesQueue generation passes the Ready condition", func() {
		queue := productQueue("edited", "team-a.edited")
		createQueue(queue)
		reconcileQueue(queue)
		reportReady(queue, testNamespaceResourceID+"/queues/team-a.edited")
		stored := reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionTrue, conditions.ReasonSucceeded)

		stored.Spec.LockDuration = &metav1.Duration{Duration: time.Minute}
		Expect(k8sClient.Update(ctx, stored)).To(Succeed())

		stored = reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionUnknown, conditions.ReasonReconciling.Name)
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionUnknown, conditions.ReasonReconciling.Name)
		var nsQueue servicebusv1.NamespacesQueue
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), &nsQueue)).To(Succeed())
		Expect(nsQueue.Status.Conditions).To(HaveLen(1))
		Expect(nsQueue.Generation).To(BeNumerically(">", nsQueue.Status.Conditions[0].ObservedGeneration))
	})

	It("creates a NamespacesQueue without a reconcile policy for deletionPolicy Delete", func() {
		queue := productQueue("deleted", "team-a.deleted")
		queue.Spec.DeletionPolicy = messagingv1alpha1.QueueDeletionPolicyDelete
		createQueue(queue)
		reconcileQueue(queue)

		var nsQueue servicebusv1.NamespacesQueue
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), &nsQueue)).To(Succeed())
		Expect(nsQueue.Annotations).NotTo(HaveKey(annotations.ReconcilePolicy))
	})

	It("removes the reconcile policy when the deletion policy changes from Retain to Delete", func() {
		queue := productQueue("retained", "team-a.retained")
		createQueue(queue)
		stored := reconcileQueue(queue)

		var nsQueue servicebusv1.NamespacesQueue
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), &nsQueue)).To(Succeed())
		Expect(nsQueue.Annotations).To(HaveKeyWithValue(annotations.ReconcilePolicy,
			string(annotations.ReconcilePolicyDetachOnDelete)))
		// ASO writes its own annotations on the object. The apply keeps them.
		nsQueue.Annotations[pollerAnnotation] = "token"
		Expect(k8sClient.Update(ctx, &nsQueue)).To(Succeed())

		stored.Spec.DeletionPolicy = messagingv1alpha1.QueueDeletionPolicyDelete
		Expect(k8sClient.Update(ctx, stored)).To(Succeed())
		reconcileQueue(queue)

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), &nsQueue)).To(Succeed())
		Expect(nsQueue.Annotations).NotTo(HaveKey(annotations.ReconcilePolicy))
		Expect(nsQueue.Annotations).To(HaveKeyWithValue(pollerAnnotation, "token"))
	})
})

func groupPrincipal(role messagingv1alpha1.QueueAccessRole, id string) messagingv1alpha1.QueueAccessPrincipalSpec {
	return messagingv1alpha1.QueueAccessPrincipalSpec{
		Role:  role,
		Group: &messagingv1alpha1.QueueGroupPrincipalSpec{Name: "group-" + id, PrincipalId: id},
	}
}

func servicePrincipal(role messagingv1alpha1.QueueAccessRole, id string) messagingv1alpha1.QueueAccessPrincipalSpec {
	return messagingv1alpha1.QueueAccessPrincipalSpec{
		Role:             role,
		ServicePrincipal: &messagingv1alpha1.QueueServicePrincipalSpec{Name: "sp-" + id, PrincipalId: id},
	}
}

func accessQueue(name string, principals ...messagingv1alpha1.QueueAccessPrincipalSpec) *messagingv1alpha1.Queue {
	queue := productQueue(name, "team-a."+name)
	queue.Spec.Access = &messagingv1alpha1.QueueAccessSpec{Principals: principals}
	return queue
}

// setPrincipals replaces the principals of a stored Queue.
func setPrincipals(queue *messagingv1alpha1.Queue, principals ...messagingv1alpha1.QueueAccessPrincipalSpec) {
	queue.Spec.Access.Principals = principals
	Expect(k8sClient.Update(ctx, queue)).To(Succeed())
}

// grantsOf lists the RoleAssignments with the labels of queue.
func grantsOf(queue *messagingv1alpha1.Queue) []authorizationv1.RoleAssignment {
	var grants authorizationv1.RoleAssignmentList
	Expect(k8sClient.List(ctx, &grants, client.InNamespace(queue.Namespace),
		client.MatchingLabels{messaging.QueueNameLabel: queue.Name})).To(Succeed())
	return grants.Items
}

// grantedRoles returns "<principal type>/<principal ID>/<role definition ID>"
// for each RoleAssignment of queue, and checks that the Queue controls it.
func grantedRoles(queue *messagingv1alpha1.Queue) []string {
	grants := grantsOf(queue)
	roles := make([]string, 0, len(grants))
	for _, grant := range grants {
		Expect(metav1.IsControlledBy(&grant, queue)).To(BeTrue(), "RoleAssignment %s", grant.Name)
		roleID := grant.Spec.RoleDefinitionReference.ARMID
		roles = append(roles, string(*grant.Spec.PrincipalType)+"/"+*grant.Spec.PrincipalId+"/"+
			roleID[strings.LastIndex(roleID, "/")+1:])
	}
	return roles
}

// reportGrants writes, as ASO would, condition as the Ready condition of the
// current generation of every RoleAssignment of queue.
func reportGrants(queue *messagingv1alpha1.Queue, condition conditions.Condition) {
	for _, grant := range grantsOf(queue) {
		condition.LastTransitionTime = metav1.Now()
		condition.ObservedGeneration = grant.Generation
		grant.Status.Conditions = []conditions.Condition{condition}
		Expect(k8sClient.Status().Update(ctx, &grant)).To(Succeed())
	}
}

// createIdentity creates an ApplicationIdentity that is not ready yet.
func createIdentity(namespace, name string) *identityv1alpha1.ApplicationIdentity {
	identity := &identityv1alpha1.ApplicationIdentity{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	Expect(k8sClient.Create(ctx, identity)).To(Succeed())
	DeferCleanup(func() { Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, identity))).To(Succeed()) })
	return identity
}

// reportIdentity writes, as the identity operator would, Ready=True and
// principalID in the status of identity.
func reportIdentity(identity *identityv1alpha1.ApplicationIdentity, principalID string) {
	identity.Status.ManagedIdentityName = ptr.To(identity.Name)
	identity.Status.PrincipalID = ptr.To(principalID)
	identity.Status.Conditions = []metav1.Condition{{
		Type:               string(identityv1alpha1.ConditionReady),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "Ready",
	}}
	Expect(k8sClient.Status().Update(ctx, identity)).To(Succeed())
}

func accessMessage(queue *messagingv1alpha1.Queue) string {
	return meta.FindStatusCondition(queue.Status.Conditions, string(messagingv1alpha1.ConditionAccessReady)).Message
}

var _ = Describe("Queue access", func() {
	BeforeEach(createProductNamespace)

	ready := conditions.Condition{
		Type:   conditions.ConditionTypeReady,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonSucceeded,
	}

	It("grants a group and a service principal their roles and mirrors ASO into AccessReady", func() {
		queue := accessQueue("granted",
			groupPrincipal(messagingv1alpha1.QueueAccessRoleReceiver, "group-id"),
			servicePrincipal(messagingv1alpha1.QueueAccessRoleSender, "sp-id"))
		createQueue(queue)

		stored := reconcileQueue(queue)
		Expect(grantedRoles(queue)).To(ConsistOf("Group/group-id/"+receiverRoleID, "ServicePrincipal/sp-id/"+senderRoleID))
		expectCondition(stored, messagingv1alpha1.ConditionAccessReady, metav1.ConditionUnknown,
			conditions.ReasonReconciling.Name)

		reportReady(queue, testNamespaceResourceID+"/queues/team-a.granted")
		reportGrants(queue, ready)
		stored = reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionAccessReady, metav1.ConditionTrue, conditions.ReasonSucceeded)
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionTrue, ReasonReady)
	})

	It("gives a consumer the sender and the receiver role", func() {
		queue := accessQueue("consumed", servicePrincipal(messagingv1alpha1.QueueAccessRoleConsumer, "sp-id"))
		createQueue(queue)

		reconcileQueue(queue)
		Expect(grantedRoles(queue)).To(ConsistOf("ServicePrincipal/sp-id/"+senderRoleID,
			"ServicePrincipal/sp-id/"+receiverRoleID))
	})

	It("reports RoleNotAllowed for the owner role and grants the other principals", func() {
		queue := accessQueue("owned",
			groupPrincipal(messagingv1alpha1.QueueAccessRoleOwner, "group-id"),
			servicePrincipal(messagingv1alpha1.QueueAccessRoleSender, "sp-id"))
		createQueue(queue)

		stored := reconcileQueue(queue)
		Expect(grantedRoles(queue)).To(ConsistOf("ServicePrincipal/sp-id/" + senderRoleID))
		expectCondition(stored, messagingv1alpha1.ConditionAccessReady, metav1.ConditionFalse, messaging.ReasonRoleNotAllowed)
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionFalse, messaging.ReasonRoleNotAllowed)
		Expect(accessMessage(stored)).To(HavePrefix(`spec.access.principals[0]: role "owner"`))
	})

	It("deletes the grant of a removed principal and leaves a RoleAssignment the operator did not create", func() {
		queue := accessQueue("removed",
			groupPrincipal(messagingv1alpha1.QueueAccessRoleSender, "kept-id"),
			groupPrincipal(messagingv1alpha1.QueueAccessRoleSender, "removed-id"))
		createQueue(queue)
		foreign := &authorizationv1.RoleAssignment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "removed-by-hand",
				Namespace: productNamespace,
				Labels:    map[string]string{messaging.QueueNameLabel: queue.Name},
			},
			Spec: authorizationv1.RoleAssignment_Spec{
				Owner:       &genruntime.ArbitraryOwnerReference{ARMID: testNamespaceResourceID},
				PrincipalId: ptr.To("hand-id"),
				RoleDefinitionReference: &genruntime.WellKnownResourceReference{
					ResourceReference: genruntime.ResourceReference{ARMID: "/subscriptions/" + testSubscriptionID +
						"/providers/Microsoft.Authorization/roleDefinitions/" + senderRoleID},
				},
			},
		}
		Expect(k8sClient.Create(ctx, foreign)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, foreign)).To(Succeed()) })

		stored := reconcileQueue(queue)
		Expect(grantsOf(queue)).To(HaveLen(3))

		setPrincipals(stored, groupPrincipal(messagingv1alpha1.QueueAccessRoleSender, "kept-id"))
		reconcileQueue(queue)
		Expect(grantsOf(queue)).To(HaveLen(2))
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(foreign), foreign)).To(Succeed())
		Expect(grantsOf(queue)).To(ContainElement(HaveField("Spec.PrincipalId", HaveValue(Equal("kept-id")))))
	})

	It("reports IdentityNotReady for an identityRef and deletes no grant while it is not resolved", func() {
		queue := accessQueue("identity",
			groupPrincipal(messagingv1alpha1.QueueAccessRoleSender, "kept-id"),
			groupPrincipal(messagingv1alpha1.QueueAccessRoleSender, "removed-id"))
		createQueue(queue)
		stored := reconcileQueue(queue)

		setPrincipals(stored, groupPrincipal(messagingv1alpha1.QueueAccessRoleSender, "kept-id"),
			identityPrincipal(messagingv1alpha1.QueueAccessRoleReceiver, "app-two"))
		stored = reconcileQueue(queue)
		Expect(grantedRoles(queue)).To(ConsistOf("Group/kept-id/"+senderRoleID, "Group/removed-id/"+senderRoleID))
		expectCondition(stored, messagingv1alpha1.ConditionAccessReady, metav1.ConditionFalse, identityref.ReasonIdentityNotReady)
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionFalse, identityref.ReasonIdentityNotReady)
	})

	It("requeues while the ApplicationIdentity is missing or not ready and grants it once it is ready", func() {
		queue := accessQueue("pending-identity", identityPrincipal(messagingv1alpha1.QueueAccessRoleSender, "app-pending"))
		createQueue(queue)
		result, err := newReconciler().Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(queue)})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(identityRequeueDelay))

		stored := &messagingv1alpha1.Queue{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), stored)).To(Succeed())
		expectCondition(stored, messagingv1alpha1.ConditionAccessReady, metav1.ConditionFalse, identityref.ReasonIdentityNotReady)
		Expect(accessMessage(stored)).To(Equal(`spec.access.principals[0]: ApplicationIdentity "` + productNamespace + `/app-pending" not found`))
		Expect(grantsOf(queue)).To(BeEmpty())

		identity := createIdentity(productNamespace, "app-pending")
		stored = reconcileQueue(queue)
		Expect(accessMessage(stored)).To(Equal(`spec.access.principals[0]: ApplicationIdentity "` + productNamespace + `/app-pending" is not ready`))

		reportIdentity(identity, "pending-id")
		result, err = newReconciler().Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(queue)})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeZero())
		Expect(grantedRoles(queue)).To(ConsistOf("ServicePrincipal/pending-id/" + senderRoleID))
	})

	It("grants an ApplicationIdentity in another namespace", func() {
		reportIdentity(createIdentity(testNamespace, "app-remote"), "remote-id")
		queue := accessQueue("remote-identity", messagingv1alpha1.QueueAccessPrincipalSpec{
			Role:        messagingv1alpha1.QueueAccessRoleReceiver,
			IdentityRef: &messagingv1alpha1.ApplicationIdentityRef{Name: "app-remote", Namespace: testNamespace},
		})
		createQueue(queue)

		reconcileQueue(queue)
		Expect(grantedRoles(queue)).To(ConsistOf("ServicePrincipal/remote-id/" + receiverRoleID))
	})

	It("moves the grant to the new principal ID of an ApplicationIdentity", func() {
		identity := createIdentity(productNamespace, "app-moved")
		reportIdentity(identity, "old-id")
		queue := accessQueue("moved-identity", identityPrincipal(messagingv1alpha1.QueueAccessRoleSender, "app-moved"))
		createQueue(queue)
		reconcileQueue(queue)
		Expect(grantedRoles(queue)).To(ConsistOf("ServicePrincipal/old-id/" + senderRoleID))

		reportIdentity(identity, "new-id")
		reconcileQueue(queue)
		Expect(grantedRoles(queue)).To(ConsistOf("ServicePrincipal/new-id/" + senderRoleID))
	})

	It("keeps the grant and reports IdentityNotReady when the ApplicationIdentity is deleted", func() {
		identity := createIdentity(productNamespace, "app-deleted")
		reportIdentity(identity, "deleted-id")
		queue := accessQueue("deleted-identity", identityPrincipal(messagingv1alpha1.QueueAccessRoleSender, "app-deleted"))
		createQueue(queue)
		reconcileQueue(queue)

		Expect(k8sClient.Delete(ctx, identity)).To(Succeed())
		stored := reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionAccessReady, metav1.ConditionFalse, identityref.ReasonIdentityNotReady)
		Expect(grantedRoles(queue)).To(ConsistOf("ServicePrincipal/deleted-id/" + senderRoleID))
	})

	It("mirrors an ASO failure of a grant into AccessReady and Ready", func() {
		queue := accessQueue("refused", groupPrincipal(messagingv1alpha1.QueueAccessRoleSender, "group-id"))
		createQueue(queue)
		reconcileQueue(queue)
		reportReady(queue, testNamespaceResourceID+"/queues/team-a.refused")
		reportGrants(queue, conditions.Condition{
			Type:     conditions.ConditionTypeReady,
			Status:   metav1.ConditionFalse,
			Severity: conditions.ConditionSeverityError,
			Reason:   "Bad-Code",
			Message:  "principal not found",
		})

		stored := reconcileQueue(queue)
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionTrue, conditions.ReasonSucceeded)
		expectCondition(stored, messagingv1alpha1.ConditionAccessReady, metav1.ConditionFalse, ReasonAccessNotReady)
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionFalse, ReasonAccessNotReady)
		Expect(accessMessage(stored)).To(Equal("RoleAssignment " + grantsOf(queue)[0].Name + ": Bad-Code: principal not found"))
	})

	It("records a RoleAssignment apply error in AccessReady", func() {
		queue := accessQueue("unapplied", groupPrincipal(messagingv1alpha1.QueueAccessRoleSender, "group-id"))
		createQueue(queue)

		reconciler := newReconciler()
		reconciler.Config.SubscriptionID = ""
		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(queue)})
		Expect(err).To(HaveOccurred())

		stored := &messagingv1alpha1.Queue{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(queue), stored)).To(Succeed())
		expectCondition(stored, messagingv1alpha1.ConditionQueueReady, metav1.ConditionUnknown,
			conditions.ReasonReconciling.Name)
		expectCondition(stored, messagingv1alpha1.ConditionAccessReady, metav1.ConditionFalse, ReasonApplyFailed)
		expectCondition(stored, messagingv1alpha1.ConditionReady, metav1.ConditionFalse, ReasonApplyFailed)
	})
})

var _ = Describe("Queue watches", func() {
	BeforeEach(createProductNamespace)

	It("maps a namespace to the Queues in it", func() {
		first, second := productQueue("first", "team-a.first"), productQueue("second", "team-a.second")
		createQueue(first)
		createQueue(second)
		createQueue(newQueue("elsewhere", nil))

		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: productNamespace}}
		Expect(newReconciler().namespaceToQueues(ctx, namespace)).To(ConsistOf(
			ctrl.Request{NamespacedName: client.ObjectKeyFromObject(first)},
			ctrl.Request{NamespacedName: client.ObjectKeyFromObject(second)},
		))
	})

	It("maps an ApplicationIdentity to the Queues that can grant it a role, in any namespace", func() {
		local := accessQueue("mapped-local", identityPrincipal(messagingv1alpha1.QueueAccessRoleSender, "app-mapped"))
		remote := newQueue("mapped-remote", func(q *messagingv1alpha1.Queue) {
			q.Spec.Access = &messagingv1alpha1.QueueAccessSpec{Principals: []messagingv1alpha1.QueueAccessPrincipalSpec{{
				Role:        messagingv1alpha1.QueueAccessRoleReceiver,
				IdentityRef: &messagingv1alpha1.ApplicationIdentityRef{Name: "app-mapped", Namespace: productNamespace},
			}}}
		})
		other := newQueue("mapped-other", func(q *messagingv1alpha1.Queue) {
			q.Spec.Access = &messagingv1alpha1.QueueAccessSpec{Principals: []messagingv1alpha1.QueueAccessPrincipalSpec{
				identityPrincipal(messagingv1alpha1.QueueAccessRoleSender, "app-mapped"),
			}}
		})
		createQueue(local)
		createQueue(remote)
		createQueue(other)

		reconciler := &QueueReconciler{Client: cachedClient}
		identity := &identityv1alpha1.ApplicationIdentity{
			ObjectMeta: metav1.ObjectMeta{Name: "app-mapped", Namespace: productNamespace},
		}
		// The cache learns about the Queues through its watch.
		Eventually(func() []ctrl.Request { return reconciler.identityToQueues(ctx, identity) }).
			WithTimeout(10 * time.Second).Should(ConsistOf(
			ctrl.Request{NamespacedName: client.ObjectKeyFromObject(local)},
			ctrl.Request{NamespacedName: client.ObjectKeyFromObject(remote)},
		))
	})

	It("drops a new and a deleted Namespace", func() {
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: productNamespace}}
		Expect(prefixAnnotationChanged().Create(event.CreateEvent{Object: namespace})).To(BeFalse())
		Expect(prefixAnnotationChanged().Delete(event.DeleteEvent{Object: namespace})).To(BeFalse())
	})

	DescribeTable("passes a Namespace update only when the prefix annotation changed",
		func(before, after map[string]string, want bool) {
			old := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: productNamespace, Annotations: before}}
			updated := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: productNamespace, Annotations: after}}
			Expect(prefixAnnotationChanged().Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated})).To(Equal(want))
		},
		Entry("prefix set", nil, map[string]string{messaging.PrefixAnnotation: "team-a."}, true),
		Entry("prefix changed", map[string]string{messaging.PrefixAnnotation: "team-a."},
			map[string]string{messaging.PrefixAnnotation: "team-b."}, true),
		Entry("prefix removed", map[string]string{messaging.PrefixAnnotation: "team-a."}, nil, true),
		Entry("another annotation changed", map[string]string{messaging.PrefixAnnotation: "team-a."},
			map[string]string{messaging.PrefixAnnotation: "team-a.", "touched": "yes"}, false),
		Entry("no prefix before or after", nil, map[string]string{"touched": "yes"}, false),
	)

	DescribeTable("passes an ASO resource update only when the generation, the controller, the reconcile policy or the status changed",
		func(mutate func(*servicebusv1.NamespacesQueue), want bool) {
			old := &servicebusv1.NamespacesQueue{ObjectMeta: metav1.ObjectMeta{
				Name:        "orders",
				Generation:  1,
				Annotations: map[string]string{annotations.ReconcilePolicy: string(annotations.ReconcilePolicyDetachOnDelete)},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: messagingv1alpha1.GroupVersion.String(),
					Kind:       "Queue",
					Name:       "orders",
					UID:        "queue-uid",
					Controller: ptr.To(true),
				}},
			}}
			updated := old.DeepCopy()
			mutate(updated)
			Expect(asoResourceChanged().Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated})).To(Equal(want))
		},
		Entry("generation", func(q *servicebusv1.NamespacesQueue) { q.Generation = 2 }, true),
		Entry("status", func(q *servicebusv1.NamespacesQueue) { q.Status.Id = ptr.To("id") }, true),
		Entry("reconcile policy removed", func(q *servicebusv1.NamespacesQueue) {
			delete(q.Annotations, annotations.ReconcilePolicy)
		}, true),
		Entry("controller reference removed", func(q *servicebusv1.NamespacesQueue) { q.OwnerReferences = nil }, true),
		Entry("poller annotation only", func(q *servicebusv1.NamespacesQueue) {
			q.Annotations[pollerAnnotation] = "token"
		}, false),
		Entry("same object", func(*servicebusv1.NamespacesQueue) {}, false),
	)

	It("drops a RoleAssignment update that only adds the poller annotation", func() {
		old := &authorizationv1.RoleAssignment{ObjectMeta: metav1.ObjectMeta{Name: "grant", Generation: 1}}
		updated := old.DeepCopy()
		updated.Annotations = map[string]string{pollerAnnotation: "token"}
		Expect(asoResourceChanged().Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated})).To(BeFalse())
		updated.Status.Scope = ptr.To("scope")
		Expect(asoResourceChanged().Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated})).To(BeTrue())
	})

	It("passes an update of another kind through the ASO resource predicate", func() {
		secret := &corev1.Secret{}
		Expect(asoResourceChanged().Update(event.UpdateEvent{ObjectOld: secret, ObjectNew: secret})).To(BeTrue())
	})
})
