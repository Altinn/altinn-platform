package controller

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/messaging"
)

const testEndpoint = "sb-messaging.servicebus.windows.net"

// createNamespace creates a namespace for one spec. envtest cannot delete a
// namespace, so the API server adds a random suffix to generateName.
func createNamespace(generateName string) string {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: generateName}}
	Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	return namespace.Name
}

// productPrefix returns the default queue name prefix of a product namespace.
func productPrefix(namespace string) string {
	return strings.TrimPrefix(namespace, "product-") + "."
}

func queueIn(namespace, name, queueName string) *messagingv1alpha1.Queue {
	return newQueue(name, func(q *messagingv1alpha1.Queue) {
		q.Namespace = namespace
		q.Spec.Name = queueName
	})
}

func removePrefix(name string) {
	var namespace corev1.Namespace
	Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, &namespace)).To(Succeed())
	patch := client.MergeFrom(namespace.DeepCopy())
	delete(namespace.Annotations, messaging.PrefixAnnotation)
	Expect(k8sClient.Patch(ctx, &namespace, patch)).To(Succeed())
}

func quotaKey(namespace string) client.ObjectKey {
	return client.ObjectKey{Namespace: namespace, Name: messaging.ResourceQuotaName}
}

func configMapKey(namespace string) client.ObjectKey {
	return client.ObjectKey{Namespace: namespace, Name: messaging.ConnectionConfigMapName}
}

func expectNoQuota(namespace string) {
	err := k8sClient.Get(ctx, quotaKey(namespace), &corev1.ResourceQuota{})
	Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected NotFound, got %v", err)
}

func expectNoConfigMap(namespace string) {
	err := k8sClient.Get(ctx, configMapKey(namespace), &corev1.ConfigMap{})
	Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected NotFound, got %v", err)
}

func queueLimit(quota *corev1.ResourceQuota) int64 {
	limit := quota.Spec.Hard[messaging.QueueCountResource]
	return limit.Value()
}

// hidingClient returns a client whose Get does not see a ConfigMap or a
// ResourceQuota for which hidden is true, as the manager cache would not.
// A hidden object leaves obj empty, as a cache miss does, so that the
// Create that follows carries no resourceVersion.
func hidingClient(hidden func(client.Object) bool) client.Client {
	direct, err := client.NewWithWatch(cfg, client.Options{Scheme: k8sClient.Scheme()})
	Expect(err).NotTo(HaveOccurred())
	return interceptor.NewClient(direct, interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object,
			opts ...client.GetOption) error {
			stored, ok := obj.DeepCopyObject().(client.Object)
			Expect(ok).To(BeTrue())
			if err := c.Get(ctx, key, stored, opts...); err != nil || !hidden(stored) {
				return c.Get(ctx, key, obj, opts...)
			}
			switch obj.(type) {
			case *corev1.ConfigMap:
				return apierrors.NewNotFound(corev1.Resource("configmaps"), key.Name)
			case *corev1.ResourceQuota:
				return apierrors.NewNotFound(corev1.Resource("resourcequotas"), key.Name)
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})
}

// labelSelectedCache is the manager cache: it holds only managed objects.
func labelSelectedCache() client.Client {
	return hidingClient(func(obj client.Object) bool { return !managed(obj) })
}

// staleCache is the manager cache before it has seen a Create.
func staleCache() client.Client {
	return hidingClient(func(client.Object) bool { return true })
}

var _ = Describe("Namespace reconciler", func() {
	var (
		r        *NamespaceReconciler
		recorder *events.FakeRecorder
	)
	reconcile := func(namespace string) {
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: namespace}})
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		recorder = events.NewFakeRecorder(10)
		r = &NamespaceReconciler{
			Client:    k8sClient,
			APIReader: k8sClient,
			Config:    config.OperatorConfig{Endpoint: testEndpoint, MaxQueuesPerNamespace: 50},
			Recorder:  recorder,
		}
	})

	It("returns cleanly for a namespace that no longer exists", func() {
		reconcile("missing")
	})

	It("creates the quota in a product namespace and none in another namespace", func() {
		product, other := createNamespace("product-team-"), createNamespace("team-")
		reconcile(product)
		reconcile(other)

		var quota corev1.ResourceQuota
		Expect(k8sClient.Get(ctx, quotaKey(product), &quota)).To(Succeed())
		Expect(quota.Labels).To(HaveKeyWithValue(messaging.ManagedByLabel, messaging.ManagedByValue))
		Expect(quota.OwnerReferences).To(BeEmpty())
		Expect(queueLimit(&quota)).To(BeEquivalentTo(50))
		expectNoConfigMap(product)
		expectNoQuota(other)
	})

	It("follows a new queue limit and keeps the labels of others", func() {
		namespace := createNamespace("product-team-")
		reconcile(namespace)
		var quota corev1.ResourceQuota
		Expect(k8sClient.Get(ctx, quotaKey(namespace), &quota)).To(Succeed())
		quota.Labels["owner"] = "team-a"
		Expect(k8sClient.Update(ctx, &quota)).To(Succeed())

		r.Config.MaxQueuesPerNamespace = 10
		reconcile(namespace)
		Expect(k8sClient.Get(ctx, quotaKey(namespace), &quota)).To(Succeed())
		Expect(quota.Labels).To(HaveKeyWithValue("owner", "team-a"))
		Expect(queueLimit(&quota)).To(BeEquivalentTo(10))
	})

	It("deletes the quota when the prefix annotation goes away", func() {
		namespace := createNamespace("team-")
		setPrefix(namespace, "team-x.")
		reconcile(namespace)
		Expect(k8sClient.Get(ctx, quotaKey(namespace), &corev1.ResourceQuota{})).To(Succeed())

		removePrefix(namespace)
		reconcile(namespace)
		expectNoQuota(namespace)
	})

	DescribeTable("neither changes nor deletes a quota without the managed-by label",
		func(cached bool) {
			namespace := createNamespace("team-")
			setPrefix(namespace, "team-x.")
			foreign := &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{Name: messaging.ResourceQuotaName, Namespace: namespace},
				Spec: corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{
					messaging.QueueCountResource: resource.MustParse("5"),
				}},
			}
			Expect(k8sClient.Create(ctx, foreign)).To(Succeed())
			if cached {
				r.Client = labelSelectedCache()
			}

			reconcile(namespace)
			Expect(recorder.Events).To(Receive(HavePrefix("Warning NotManaged ResourceQuota dis-messaging exists")))
			removePrefix(namespace)
			reconcile(namespace)

			var stored corev1.ResourceQuota
			Expect(k8sClient.Get(ctx, quotaKey(namespace), &stored)).To(Succeed())
			Expect(stored.Labels).To(BeEmpty())
			Expect(queueLimit(&stored)).To(BeEquivalentTo(5))
		},
		Entry("read from the API server", false),
		Entry("hidden by the label-selected cache", true),
	)

	It("retries when the cache lags behind its own Create and reports nothing", func() {
		namespace := createNamespace("product-team-")
		reconcile(namespace)

		r.Client = staleCache()
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: namespace}})
		Expect(apierrors.IsAlreadyExists(err)).To(BeTrue(), "expected AlreadyExists, got %v", err)
		Expect(recorder.Events).NotTo(Receive())
		var quota corev1.ResourceQuota
		Expect(k8sClient.Get(ctx, quotaKey(namespace), &quota)).To(Succeed())
		Expect(quota.Labels).To(HaveKeyWithValue(messaging.ManagedByLabel, messaging.ManagedByValue))
	})

	It("writes a key for each Queue with a valid name that is not being deleted", func() {
		namespace := createNamespace("product-team-")
		prefix := productPrefix(namespace)
		createQueue(queueIn(namespace, "orders", prefix+"orders"))
		createQueue(queueIn(namespace, "invoices", prefix+"sync/invoices"))
		createQueue(queueIn(namespace, "foreign", "team-b.foreign"))
		deleting := queueIn(namespace, "deleting", prefix+"deleting")
		deleting.Finalizers = []string{"test.dis.altinn.cloud/hold"}
		Expect(k8sClient.Create(ctx, deleting)).To(Succeed())
		Expect(k8sClient.Delete(ctx, deleting)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deleting), deleting)).To(Succeed())
			deleting.Finalizers = nil
			Expect(k8sClient.Update(ctx, deleting)).To(Succeed())
		})

		reconcile(namespace)
		var configMap corev1.ConfigMap
		Expect(k8sClient.Get(ctx, configMapKey(namespace), &configMap)).To(Succeed())
		Expect(configMap.Labels).To(HaveKeyWithValue(messaging.ComponentLabel, messaging.ComponentConnection))
		Expect(configMap.OwnerReferences).To(BeEmpty())
		Expect(configMap.Data).To(Equal(map[string]string{
			messaging.EndpointKey: testEndpoint,
			"queue.orders":        prefix + "orders",
			"queue.invoices":      prefix + "sync/invoices",
		}))
	})

	It("replaces the data of the ConfigMap, clears binaryData and keeps the labels of others", func() {
		namespace := createNamespace("product-team-")
		createQueue(queueIn(namespace, "orders", productPrefix(namespace)+"orders"))
		reconcile(namespace)
		var configMap corev1.ConfigMap
		Expect(k8sClient.Get(ctx, configMapKey(namespace), &configMap)).To(Succeed())
		configMap.Labels["owner"] = "team-a"
		configMap.Data["stray"] = "yes"
		configMap.BinaryData = map[string][]byte{"blob": []byte("x")}
		Expect(k8sClient.Update(ctx, &configMap)).To(Succeed())

		reconcile(namespace)
		Expect(k8sClient.Get(ctx, configMapKey(namespace), &configMap)).To(Succeed())
		Expect(configMap.Labels).To(HaveKeyWithValue("owner", "team-a"))
		Expect(configMap.BinaryData).To(BeEmpty())
		Expect(configMap.Data).To(Equal(map[string]string{
			messaging.EndpointKey: testEndpoint,
			"queue.orders":        productPrefix(namespace) + "orders",
		}))
	})

	It("keeps the ConfigMap with only the endpoint while every Queue has an invalid name", func() {
		namespace := createNamespace("team-")
		setPrefix(namespace, "team-x.")
		createQueue(queueIn(namespace, "orders", "team-x.orders"))
		reconcile(namespace)

		setPrefix(namespace, "team-y.")
		reconcile(namespace)
		var configMap corev1.ConfigMap
		Expect(k8sClient.Get(ctx, configMapKey(namespace), &configMap)).To(Succeed())
		Expect(configMap.Data).To(Equal(map[string]string{messaging.EndpointKey: testEndpoint}))
	})

	It("deletes the ConfigMap when the last Queue is gone and keeps the quota", func() {
		namespace := createNamespace("product-team-")
		queue := queueIn(namespace, "orders", productPrefix(namespace)+"orders")
		Expect(k8sClient.Create(ctx, queue)).To(Succeed())
		reconcile(namespace)
		Expect(k8sClient.Get(ctx, configMapKey(namespace), &corev1.ConfigMap{})).To(Succeed())

		Expect(k8sClient.Delete(ctx, queue)).To(Succeed())
		reconcile(namespace)
		expectNoConfigMap(namespace)
		Expect(k8sClient.Get(ctx, quotaKey(namespace), &corev1.ResourceQuota{})).To(Succeed())
	})

	It("deletes the ConfigMap with the quota when the prefix annotation goes away", func() {
		namespace := createNamespace("team-")
		setPrefix(namespace, "team-x.")
		createQueue(queueIn(namespace, "orders", "team-x.orders"))
		reconcile(namespace)
		Expect(k8sClient.Get(ctx, configMapKey(namespace), &corev1.ConfigMap{})).To(Succeed())

		removePrefix(namespace)
		reconcile(namespace)
		expectNoConfigMap(namespace)
		expectNoQuota(namespace)
	})

	DescribeTable("neither changes nor deletes a ConfigMap without the managed-by label",
		func(cached bool) {
			namespace := createNamespace("product-team-")
			foreign := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: messaging.ConnectionConfigMapName, Namespace: namespace},
				Data:       map[string]string{"owner": "team-a"},
			}
			Expect(k8sClient.Create(ctx, foreign)).To(Succeed())
			queue := queueIn(namespace, "orders", productPrefix(namespace)+"orders")
			Expect(k8sClient.Create(ctx, queue)).To(Succeed())
			if cached {
				r.Client = labelSelectedCache()
			}

			reconcile(namespace)
			Expect(recorder.Events).To(Receive(HavePrefix("Warning NotManaged ConfigMap dis-messaging exists")))
			Expect(k8sClient.Delete(ctx, queue)).To(Succeed())
			reconcile(namespace)

			var stored corev1.ConfigMap
			Expect(k8sClient.Get(ctx, configMapKey(namespace), &stored)).To(Succeed())
			Expect(stored.Labels).To(BeEmpty())
			Expect(stored.Data).To(Equal(foreign.Data))
		},
		Entry("read from the API server", false),
		Entry("hidden by the label-selected cache", true),
	)
})

var _ = Describe("Namespace watches", func() {
	It("maps a quota to its namespace", func() {
		quota := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: "quota", Namespace: productNamespace}}
		Expect(namespaceOf(ctx, quota)).To(ConsistOf(ctrl.Request{NamespacedName: client.ObjectKey{Name: productNamespace}}))
	})

	It("passes a new namespace and a prefix change and drops a deleted namespace", func() {
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: productNamespace}}
		prefixed := namespace.DeepCopy()
		prefixed.Annotations = map[string]string{messaging.PrefixAnnotation: "team-a."}
		Expect(namespaceChanged().Create(event.CreateEvent{Object: namespace})).To(BeTrue())
		Expect(namespaceChanged().Delete(event.DeleteEvent{Object: namespace})).To(BeFalse())
		Expect(namespaceChanged().Update(event.UpdateEvent{ObjectOld: namespace, ObjectNew: prefixed})).To(BeTrue())
		Expect(namespaceChanged().Update(event.UpdateEvent{ObjectOld: namespace, ObjectNew: namespace})).To(BeFalse())
	})

	It("passes a new and a deleted Queue", func() {
		queue := newQueue("orders", nil)
		Expect(queueChanged().Create(event.CreateEvent{Object: queue})).To(BeTrue())
		Expect(queueChanged().Delete(event.DeleteEvent{Object: queue})).To(BeTrue())
	})

	DescribeTable("passes a Queue update only when the spec changed or the deletion started",
		func(mutate func(*messagingv1alpha1.Queue), want bool) {
			old := newQueue("orders", func(q *messagingv1alpha1.Queue) { q.Generation = 1 })
			updated := old.DeepCopy()
			mutate(updated)
			Expect(queueChanged().Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated})).To(Equal(want))
		},
		Entry("spec", func(q *messagingv1alpha1.Queue) { q.Generation = 2 }, true),
		Entry("deletion started", func(q *messagingv1alpha1.Queue) { q.DeletionTimestamp = &metav1.Time{Time: time.Now()} }, true),
		Entry("status only", func(q *messagingv1alpha1.Queue) { q.Status.Name = "team-a.orders" }, false),
	)
})
