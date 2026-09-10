package controller

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	valkeyv1alpha1 "github.com/valkey-io/valkey-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"

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

func getSecret(name string) *corev1.Secret {
	var secret corev1.Secret
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: name}, &secret)).To(Succeed())

	return &secret
}

func getNetworkPolicy(name string) *netv1.NetworkPolicy {
	var policy netv1.NetworkPolicy
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: name}, &policy)).To(Succeed())

	return &policy
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

	It("admits a name at the length limit of the Valkey Service name", func() {
		cache := newCache(strings.Repeat("c", cachev1alpha1.MaxCacheNameLength), nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
	})

	It("rejects a name that makes the Valkey Service name too long", func() {
		cache := newCache(strings.Repeat("c", cachev1alpha1.MaxCacheNameLength+1), nil)
		Expect(k8sClient.Create(ctx, cache)).To(MatchError(ContainSubstring("at most 56 characters")))
	})

	It("rejects a name with a dot", func() {
		cache := newCache("cache.dotted", nil)
		Expect(k8sClient.Create(ctx, cache)).To(MatchError(ContainSubstring("must not contain a dot")))
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

	It("creates the auth Secret owned by the Cache and keeps its password afterwards", func() {
		cache := newCache("cache-secret", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })

		reconcile("cache-secret")

		secret := getSecret(cachepkg.AuthSecretName(cache))
		Expect(metav1.IsControlledBy(secret, getCache("cache-secret"))).To(BeTrue())
		Expect(string(secret.Data[cachepkg.AuthSecretUsernameKey])).To(Equal(cachepkg.AuthUsername))
		password := secret.Data[cachepkg.AuthSecretPasswordKey]
		Expect(password).NotTo(BeEmpty())

		cluster := getValkeyCluster("cache-secret")
		Expect(cluster.Spec.Users[1].PasswordSecret.Name).To(Equal(secret.Name))

		reconcile("cache-secret")
		Expect(getSecret(secret.Name).Data[cachepkg.AuthSecretPasswordKey]).To(Equal(password))
	})

	It("creates the Secret again with a new password after someone deletes it", func() {
		cache := newCache("cache-secret-gone", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		reconcile("cache-secret-gone")

		secret := getSecret(cachepkg.AuthSecretName(cache))
		Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		reconcile("cache-secret-gone")

		recreated := getSecret(secret.Name)
		Expect(recreated.Data[cachepkg.AuthSecretPasswordKey]).NotTo(BeEmpty())
		Expect(recreated.Data[cachepkg.AuthSecretPasswordKey]).NotTo(Equal(secret.Data[cachepkg.AuthSecretPasswordKey]))
	})

	It("creates the NetworkPolicy owned by the Cache and restores its rules", func() {
		cache := newCache("cache-netpol", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })

		reconcile("cache-netpol")

		policy := getNetworkPolicy(cachepkg.NetworkPolicyName(cache))
		Expect(metav1.IsControlledBy(policy, getCache("cache-netpol"))).To(BeTrue())
		want := len(cachepkg.BuildNetworkPolicy(cache).Spec.Ingress)
		Expect(policy.Spec.Ingress).To(HaveLen(want))

		policy.Spec.Ingress = nil
		Expect(k8sClient.Update(ctx, policy)).To(Succeed())
		reconcile("cache-netpol")

		Expect(getNetworkPolicy(policy.Name).Spec.Ingress).To(HaveLen(want))
	})

	It("converges: repeated reconciles stop writing the owned objects and the status", func() {
		cache := newCache("cache-twice", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })

		reconcile("cache-twice")
		reconcile("cache-twice")
		settled := getValkeyCluster("cache-twice")
		settledPolicy := getNetworkPolicy(cachepkg.NetworkPolicyName(cache))
		settledCache := getCache("cache-twice")
		reconcile("cache-twice")
		again := getValkeyCluster("cache-twice")
		Expect(again.ResourceVersion).To(Equal(settled.ResourceVersion))
		Expect(again.Generation).To(Equal(int64(1)))
		Expect(getNetworkPolicy(settledPolicy.Name).ResourceVersion).To(Equal(settledPolicy.ResourceVersion))
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
	defaulted[0].Enabled = !desired[0].Enabled
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

// TestRoleGrantsNoSecretReads guards the generated ClusterRole: the operator
// creates Secrets but must never be able to read them.
func TestRoleGrantsNoSecretReads(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "config", "rbac", "role.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var role rbacv1.ClusterRole
	if err := yaml.Unmarshal(raw, &role); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, rule := range role.Rules {
		if slices.Contains(rule.APIGroups, "*") || slices.Contains(rule.Resources, "*") {
			t.Errorf("wildcard rule grants Secret reads: %+v", rule)
		}
		if !slices.Contains(rule.Resources, "secrets") {
			continue
		}
		found = true
		if !slices.Equal(rule.APIGroups, []string{""}) {
			t.Errorf("secrets apiGroups: want [\"\"], got %v", rule.APIGroups)
		}
		if !slices.Equal(rule.Verbs, []string{"create"}) {
			t.Errorf("secrets verbs: want [create], got %v", rule.Verbs)
		}
	}
	if !found {
		t.Fatal("role.yaml has no rule for secrets")
	}
}
