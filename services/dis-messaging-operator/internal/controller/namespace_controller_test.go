package controller

import (
	"context"

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

	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/messaging"
)

// createNamespace creates a namespace for one spec. envtest cannot delete a
// namespace, so the API server adds a random suffix to generateName.
func createNamespace(generateName string) string {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: generateName}}
	Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	return namespace.Name
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

func expectNoQuota(namespace string) {
	err := k8sClient.Get(ctx, quotaKey(namespace), &corev1.ResourceQuota{})
	Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected NotFound, got %v", err)
}

func queueLimit(quota *corev1.ResourceQuota) int64 {
	limit := quota.Spec.Hard[messaging.QueueCountResource]
	return limit.Value()
}

// cachedQuotas returns a client that, like the label-selected cache of the
// manager, does not see a ResourceQuota without the managed-by label.
func cachedQuotas() client.Client {
	direct, err := client.NewWithWatch(cfg, client.Options{Scheme: k8sClient.Scheme()})
	Expect(err).NotTo(HaveOccurred())
	return interceptor.NewClient(direct, interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object,
			opts ...client.GetOption) error {
			_, isQuota := obj.(*corev1.ResourceQuota)
			if err := c.Get(ctx, key, obj, opts...); err != nil || !isQuota || managed(obj) {
				return err
			}
			return apierrors.NewNotFound(corev1.Resource("resourcequotas"), key.Name)
		},
	})
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
			Client:   k8sClient,
			Config:   config.OperatorConfig{MaxQueuesPerNamespace: 50},
			Recorder: recorder,
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
				r.Client = cachedQuotas()
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
})
