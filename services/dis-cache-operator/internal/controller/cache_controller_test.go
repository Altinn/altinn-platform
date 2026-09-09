package controller

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	valkeyv1alpha1 "github.com/valkey-io/valkey-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
	cachepkg "github.com/Altinn/altinn-platform/services/dis-cache-operator/internal/cache"
)

const testNamespace = "default"

func newCache(name string, mutate func(*cachev1alpha1.Cache)) *cachev1alpha1.Cache {
	cache := &cachev1alpha1.Cache{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
	}
	if mutate != nil {
		mutate(cache)
	}

	return cache
}

func newReconciler() *CacheReconciler {
	return &CacheReconciler{
		Client: k8sClient,
		Scheme: k8sClient.Scheme(),
		Images: cachepkg.Images{Valkey: "registry.example/valkey:9"},
	}
}

func reconcile(name string) {
	_, err := newReconciler().Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: name},
	})
	Expect(err).NotTo(HaveOccurred())
}

func getCache(name string) *cachev1alpha1.Cache {
	var cache cachev1alpha1.Cache
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: name}, &cache)).To(Succeed())

	return &cache
}

func getValkeyCluster(name string) *valkeyv1alpha1.ValkeyCluster {
	var cluster valkeyv1alpha1.ValkeyCluster
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: name}, &cluster)).To(Succeed())

	return &cluster
}

func readyOf(cache *cachev1alpha1.Cache) *metav1.Condition {
	for i := range cache.Status.Conditions {
		if cache.Status.Conditions[i].Type == string(cachev1alpha1.ConditionReady) {
			return &cache.Status.Conditions[i]
		}
	}

	return nil
}

var _ = Describe("Cache CRD schema", func() {
	It("admits an empty-spec Cache and applies platform defaults", func() {
		cache := newCache("cache-defaults", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })

		stored := getCache("cache-defaults")
		Expect(stored.Spec.Size).To(Equal(cachev1alpha1.CacheSizeSmall))
		Expect(stored.Spec.EvictionPolicy).To(Equal(cachev1alpha1.CacheEvictionNoEviction))
		Expect(stored.Spec.Persistence).To(BeFalse())
	})

	It("rejects an unknown size value", func() {
		cache := newCache("cache-bad-size", func(c *cachev1alpha1.Cache) { c.Spec.Size = "gigantic" })
		Expect(k8sClient.Create(ctx, cache)).NotTo(Succeed())
	})

	It("rejects an unknown eviction policy", func() {
		cache := newCache("cache-bad-eviction", func(c *cachev1alpha1.Cache) { c.Spec.EvictionPolicy = "evict-everything" })
		Expect(k8sClient.Create(ctx, cache)).NotTo(Succeed())
	})
})

var _ = Describe("Cache reconciler", func() {
	It("creates the ValkeyCluster owned by the Cache and reports Provisioning", func() {
		cache := newCache("cache-create", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })

		reconcile("cache-create")

		cluster := getValkeyCluster("cache-create")
		Expect(metav1.IsControlledBy(cluster, getCache("cache-create"))).To(BeTrue())
		Expect(cluster.Spec.Shards).To(Equal(int32(1)))
		Expect(cluster.Spec.Image).To(Equal("registry.example/valkey:9"))
		Expect(cluster.Spec.Users).To(HaveLen(2))
		Expect(cluster.Spec.Users[0].Name).To(Equal("default"))
		Expect(cluster.Spec.Users[0].ResetPass).To(BeTrue())
		Expect(cluster.Spec.Users[1].Name).To(Equal(cachepkg.AuthUsername))

		ready := readyOf(getCache("cache-create"))
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(ReasonProvisioning))
	})

	It("converges: repeated reconciles stop writing the ValkeyCluster and the status", func() {
		cache := newCache("cache-twice", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })

		reconcile("cache-twice")
		reconcile("cache-twice")
		settled := getValkeyCluster("cache-twice")
		settledCache := getCache("cache-twice")
		reconcile("cache-twice")
		again := getValkeyCluster("cache-twice")
		Expect(again.ResourceVersion).To(Equal(settled.ResourceVersion))
		Expect(again.Generation).To(Equal(int64(1)))
		Expect(getCache("cache-twice").ResourceVersion).To(Equal(settledCache.ResourceVersion))
	})

	It("reports Ready with host and port when the upstream state is Ready", func() {
		cache := newCache("cache-ready", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		reconcile("cache-ready")

		cluster := getValkeyCluster("cache-ready")
		cluster.Status.State = valkeyv1alpha1.ClusterStateReady
		Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())
		reconcile("cache-ready")

		stored := getCache("cache-ready")
		ready := readyOf(stored)
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		Expect(ready.Reason).To(Equal(ReasonValkeyReady))
		Expect(stored.Status.Host).To(Equal("valkey-cache-ready.default.svc.cluster.local"))
		Expect(stored.Status.Port).To(Equal(int32(6379)))
		Expect(stored.Status.ObservedGeneration).To(Equal(stored.Generation))
	})

	It("reports the upstream failure message when the state is Failed", func() {
		cache := newCache("cache-failed", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		reconcile("cache-failed")

		cluster := getValkeyCluster("cache-failed")
		cluster.Status.State = valkeyv1alpha1.ClusterStateFailed
		cluster.Status.Message = "no nodes available"
		Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())
		reconcile("cache-failed")

		ready := readyOf(getCache("cache-failed"))
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(ReasonValkeyFailed))
		Expect(ready.Message).To(Equal("no nodes available"))
		Expect(getCache("cache-failed").Status.Host).To(BeEmpty())
	})

	It("returns cleanly for a Cache that no longer exists", func() {
		reconcile("cache-missing")
	})
})

func TestUsersMatch(t *testing.T) {
	t.Parallel()

	desired := cachepkg.BuildValkeyCluster(&cachev1alpha1.Cache{
		ObjectMeta: metav1.ObjectMeta{Name: "app-one-cache", Namespace: "team-a"},
	}, cachepkg.Images{}).Spec.Users

	if !usersMatch(desired, desired) {
		t.Fatal("identical users must match")
	}

	defaulted := make([]valkeyv1alpha1.UserAclSpec, len(desired))
	copy(defaulted, desired)
	defaulted[0].Enabled = true
	if !usersMatch(desired, defaulted) {
		t.Error("a defaulted enabled flag must not count as a mismatch")
	}

	pruned := desired[:1]
	if usersMatch(desired, pruned) {
		t.Error("a missing app user must be a mismatch")
	}

	weakened := make([]valkeyv1alpha1.UserAclSpec, len(desired))
	copy(weakened, desired)
	weakened[0].ResetPass = false
	if usersMatch(desired, weakened) {
		t.Error("an unlocked default user must be a mismatch")
	}
}
