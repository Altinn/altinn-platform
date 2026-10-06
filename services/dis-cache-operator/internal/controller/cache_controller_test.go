package controller

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	policyv1alpha1 "github.com/linkerd/linkerd2/controller/gen/apis/policy/v1alpha1"
	serverv1beta3 "github.com/linkerd/linkerd2/controller/gen/apis/server/v1beta3"
	valkeyv1alpha1 "github.com/valkey-io/valkey-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"

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
		Client:  k8sClient,
		Scheme:  k8sClient.Scheme(),
		Images:  cachepkg.Images{Valkey: "registry.example/valkey:9"},
		Scraper: cachepkg.Scraper{Namespace: "observability", ServiceAccount: "collector"},
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

func mustGet(name string, obj client.Object) {
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: name}, obj)).To(Succeed())
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
		Expect(cluster.Spec.Users[1].PasswordSecret.Keys).To(Equal([]string{cachepkg.AuthSecretPasswordKey}))

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
		want := cachepkg.BuildNetworkPolicy(cache, newReconciler().Scraper).Spec.Ingress
		Expect(policy.Spec.Ingress).To(Equal(want))

		policy.Spec.Ingress = nil
		Expect(k8sClient.Update(ctx, policy)).To(Succeed())
		Expect(getNetworkPolicy(policy.Name).Spec.Ingress).To(BeEmpty())
		reconcile("cache-netpol")

		Expect(getNetworkPolicy(policy.Name).Spec.Ingress).To(Equal(want))
	})

	It("keeps a Secret that someone else created with the same name", func() {
		cache := newCache("cache-foreign-secret", nil)
		foreign := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: cachepkg.AuthSecretName(cache), Namespace: testNamespace},
			StringData: map[string]string{cachepkg.AuthSecretPasswordKey: "team-chosen"},
		}
		Expect(k8sClient.Create(ctx, foreign)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, foreign)).To(Succeed()) })
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })

		reconcile("cache-foreign-secret")

		secret := getSecret(foreign.Name)
		Expect(string(secret.Data[cachepkg.AuthSecretPasswordKey])).To(Equal("team-chosen"))
		Expect(secret.OwnerReferences).To(BeEmpty())
	})

	It("records a failed Secret create in the Ready condition", func() {
		cache := newCache("cache-secret-fails", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })

		watchClient, err := client.NewWithWatch(cfg, client.Options{Scheme: k8sClient.Scheme()})
		Expect(err).NotTo(HaveOccurred())
		failing := interceptor.NewClient(watchClient, interceptor.Funcs{
			Create: func(callCtx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if _, isSecret := obj.(*corev1.Secret); isSecret {
					return errors.New("secrets are denied")
				}
				return c.Create(callCtx, obj, opts...)
			},
		})
		reconciler := &CacheReconciler{Client: failing, Scheme: k8sClient.Scheme()}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "cache-secret-fails"},
		})
		Expect(err).To(MatchError(ContainSubstring("secrets are denied")))

		ready := readyOf(getCache("cache-secret-fails"))
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(ReasonSecretCreateFailed))
		Expect(ready.Message).To(ContainSubstring("secrets are denied"))
	})

	It("updates the ValkeyCluster when the Cache size changes", func() {
		cache := newCache("cache-resize", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		reconcile("cache-resize")
		Expect(getValkeyCluster("cache-resize").Spec.Replicas).To(Equal(int32(1)))

		stored := getCache("cache-resize")
		stored.Spec.Size = cachev1alpha1.CacheSizeLarge
		Expect(k8sClient.Update(ctx, stored)).To(Succeed())
		reconcile("cache-resize")

		cluster := getValkeyCluster("cache-resize")
		Expect(cluster.Generation).To(Equal(int64(2)))
		Expect(cluster.Spec.Replicas).To(Equal(int32(2)))
		Expect(getCache("cache-resize").Status.ObservedGeneration).To(Equal(int64(2)))
	})

	It("creates the linkerd policies owned by the Cache", func() {
		cache := newCache("cache-mesh", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })

		reconcile("cache-mesh")
		owner := getCache("cache-mesh")

		var clientServer, busServer serverv1beta3.Server
		mustGet(cachepkg.ClientServerName(cache), &clientServer)
		mustGet(cachepkg.BusServerName(cache), &busServer)
		Expect(metav1.IsControlledBy(&clientServer, owner)).To(BeTrue())
		Expect(clientServer.Spec.Port.IntValue()).To(Equal(6379))
		Expect(clientServer.Spec.ProxyProtocol).To(Equal("opaque"))
		Expect(busServer.Spec.Port.IntValue()).To(Equal(16379))
		Expect(busServer.Spec.ProxyProtocol).To(Equal("opaque"))

		var authentication policyv1alpha1.MeshTLSAuthentication
		mustGet(cachepkg.MeshAuthenticationName(cache), &authentication)
		Expect(metav1.IsControlledBy(&authentication, owner)).To(BeTrue())
		Expect(authentication.Spec.Identities).To(HaveLen(2))

		var metricsServer serverv1beta3.Server
		mustGet(cachepkg.MetricsServerName(cache), &metricsServer)
		Expect(metav1.IsControlledBy(&metricsServer, owner)).To(BeTrue())
		Expect(metricsServer.Spec.Port.IntValue()).To(Equal(9121))
		Expect(metricsServer.Spec.ProxyProtocol).To(Equal("HTTP/1"))
		var scrapers policyv1alpha1.MeshTLSAuthentication
		mustGet(cachepkg.ScraperAuthenticationName(cache), &scrapers)
		Expect(metav1.IsControlledBy(&scrapers, owner)).To(BeTrue())
		Expect(scrapers.Spec.Identities).To(ConsistOf("collector.observability.serviceaccount.identity.linkerd.cluster.local"))
		var metricsPolicy policyv1alpha1.AuthorizationPolicy
		mustGet(cachepkg.MetricsServerName(cache), &metricsPolicy)
		Expect(metav1.IsControlledBy(&metricsPolicy, owner)).To(BeTrue())
		Expect(string(metricsPolicy.Spec.TargetRef.Name)).To(Equal(cachepkg.MetricsServerName(cache)))
		Expect(string(metricsPolicy.Spec.RequiredAuthenticationRefs[0].Kind)).To(Equal("MeshTLSAuthentication"))
		Expect(string(metricsPolicy.Spec.RequiredAuthenticationRefs[0].Name)).To(Equal(cachepkg.ScraperAuthenticationName(cache)))

		for _, name := range []string{cachepkg.ClientServerName(cache), cachepkg.BusServerName(cache)} {
			var authorization policyv1alpha1.AuthorizationPolicy
			mustGet(name, &authorization)
			Expect(metav1.IsControlledBy(&authorization, owner)).To(BeTrue())
			Expect(string(authorization.Spec.TargetRef.Name)).To(Equal(name))
			Expect(authorization.Spec.RequiredAuthenticationRefs).To(HaveLen(1))
		}
	})

	It("removes an identity that someone added to the client or scraper authentication", func() {
		cache := newCache("cache-mesh-drift", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		reconcile("cache-mesh-drift")

		for _, name := range []string{cachepkg.MeshAuthenticationName(cache), cachepkg.ScraperAuthenticationName(cache)} {
			var authentication policyv1alpha1.MeshTLSAuthentication
			mustGet(name, &authentication)
			want := len(authentication.Spec.Identities)
			Expect(want).To(BeNumerically(">", 0), name)
			authentication.Spec.Identities = append(authentication.Spec.Identities, "*")
			Expect(k8sClient.Update(ctx, &authentication)).To(Succeed())
			mustGet(name, &authentication)
			Expect(authentication.Spec.Identities).To(HaveLen(want + 1))

			reconcile("cache-mesh-drift")
			mustGet(name, &authentication)
			Expect(authentication.Spec.Identities).To(HaveLen(want), name)
			Expect(authentication.Spec.Identities).NotTo(ContainElement("*"), name)
		}
	})

	It("converges: repeated reconciles stop writing the owned objects and the status", func() {
		cache := newCache("cache-twice", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })

		reconcile("cache-twice")
		reconcile("cache-twice")
		settled := getValkeyCluster("cache-twice")
		settledPolicy := getNetworkPolicy(cachepkg.NetworkPolicyName(cache))
		var settledAuthorization, againAuthorization policyv1alpha1.AuthorizationPolicy
		mustGet(cachepkg.ClientServerName(cache), &settledAuthorization)
		settledCache := getCache("cache-twice")
		reconcile("cache-twice")
		again := getValkeyCluster("cache-twice")
		Expect(again.ResourceVersion).To(Equal(settled.ResourceVersion))
		Expect(again.Generation).To(Equal(int64(1)))
		Expect(getNetworkPolicy(settledPolicy.Name).ResourceVersion).To(Equal(settledPolicy.ResourceVersion))
		mustGet(settledAuthorization.Name, &againAuthorization)
		Expect(againAuthorization.ResourceVersion).To(Equal(settledAuthorization.ResourceVersion))
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

// TestRolesGrantNoSecretReads guards the generated roles: the operator
// creates Secrets but must never be able to read them.
func TestRolesGrantNoSecretReads(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("..", "..", "config", "rbac", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no rbac files: %v", err)
	}

	found := false
	for _, file := range files {
		for _, role := range readRoles(t, file) {
			for _, rule := range role.Rules {
				if slices.Contains(rule.APIGroups, "*") || slices.Contains(rule.Resources, "*") {
					t.Errorf("%s: wildcard rule can grant Secret reads: %+v", file, rule)
				}
				if !slices.Contains(rule.Resources, "secrets") {
					continue
				}
				found = true
				if !slices.Equal(rule.APIGroups, []string{""}) {
					t.Errorf("%s: secrets apiGroups: want [\"\"], got %v", file, rule.APIGroups)
				}
				if !slices.Equal(rule.Verbs, []string{"create", "patch"}) {
					t.Errorf("%s: secrets verbs: want [create patch], got %v", file, rule.Verbs)
				}
			}
		}
	}
	if !found {
		t.Fatal("no rbac file has a rule for secrets")
	}
}

type rbacRole struct {
	Kind  string              `json:"kind"`
	Rules []rbacv1.PolicyRule `json:"rules"`
}

// readRoles returns every Role and ClusterRole document in a YAML file.
func readRoles(t *testing.T, file string) []rbacRole {
	t.Helper()

	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	var roles []rbacRole
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(raw), 4096)
	for {
		var role rbacRole
		if err := decoder.Decode(&role); err != nil {
			if errors.Is(err, io.EOF) {
				return roles
			}
			t.Fatalf("%s: %v", file, err)
		}
		if role.Kind == "Role" || role.Kind == "ClusterRole" {
			roles = append(roles, role)
		}
	}
}

func TestValkeyClusterChanged(t *testing.T) {
	t.Parallel()

	base := &valkeyv1alpha1.ValkeyCluster{ObjectMeta: metav1.ObjectMeta{Name: "app-one-cache", Generation: 1}}
	statusChanged := base.DeepCopy()
	statusChanged.Status.State = valkeyv1alpha1.ClusterStateReady
	generationChanged := base.DeepCopy()
	generationChanged.Generation = 2
	labelsChanged := base.DeepCopy()
	labelsChanged.Labels = map[string]string{"touched": "yes"}

	tests := []struct {
		name string
		new  client.Object
		want bool
	}{
		{name: "status change", new: statusChanged, want: true},
		{name: "generation change", new: generationChanged, want: true},
		{name: "labels only", new: labelsChanged, want: false},
		{name: "same object", new: base.DeepCopy(), want: false},
		{name: "other kind", new: &corev1.Secret{}, want: true},
	}
	pred := valkeyClusterChanged()
	for _, tt := range tests {
		if got := pred.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: tt.new}); got != tt.want {
			t.Errorf("%s: want %v, got %v", tt.name, tt.want, got)
		}
	}
}

// rotationPeriod is the default period in these specs. The fixed clock moves
// past it when a spec needs the period to end.
const rotationPeriod = time.Hour

// rotationReconciler returns a reconciler with a fixed clock and the spec
// period.
func rotationReconciler(now *time.Time) *CacheReconciler {
	r := newReconciler()
	r.PreviousValidFor = rotationPeriod
	r.Now = func() time.Time { return *now }

	return r
}

func reconcileWith(r *CacheReconciler, name string) ctrl.Result {
	result, err := r.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: name},
	})
	Expect(err).NotTo(HaveOccurred())

	return result
}

func rotatedOf(cache *cachev1alpha1.Cache) *metav1.Condition {
	return meta.FindStatusCondition(cache.Status.Conditions, string(cachev1alpha1.ConditionPasswordRotated))
}

// requestRotation writes a rotation request with the given marker time.
func requestRotation(name string, at time.Time) {
	cache := getCache(name)
	if cache.Spec.PasswordRotation == nil {
		cache.Spec.PasswordRotation = &cachev1alpha1.PasswordRotationSpec{}
	}
	cache.Spec.PasswordRotation.RequestedAt = &metav1.Time{Time: at}
	Expect(k8sClient.Update(ctx, cache)).To(Succeed())
}

var _ = Describe("Cache password rotation", func() {
	It("rotates on request, keeps the previous password valid, and removes it after the period", func() {
		now := time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC)
		r := rotationReconciler(&now)
		cache := newCache("cache-rotate", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		reconcileWith(r, "cache-rotate")
		secret := getSecret(cachepkg.AuthSecretName(cache))
		first := secret.Data[cachepkg.AuthSecretPasswordKey]
		Expect(secret.Annotations[cachepkg.RotationStepAnnotation]).To(Equal(cachepkg.RotationStepIdle))

		requestRotation("cache-rotate", now)
		result := reconcileWith(r, "cache-rotate")

		rotated := getSecret(secret.Name)
		Expect(rotated.Data[cachepkg.AuthSecretPreviousPasswordKey]).To(Equal(first))
		Expect(rotated.Data[cachepkg.AuthSecretPasswordKey]).NotTo(Equal(first))
		Expect(rotated.Annotations[cachepkg.RotationStepAnnotation]).To(Equal(cachepkg.RotationStepReplaced))
		Expect(getValkeyCluster("cache-rotate").Spec.Users[1].PasswordSecret.Keys).To(Equal(
			[]string{cachepkg.AuthSecretPasswordKey, cachepkg.AuthSecretPreviousPasswordKey}))
		updated := getCache("cache-rotate")
		cond := rotatedOf(updated)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.Reason).To(Equal(ReasonPreviousPasswordValid))
		Expect(updated.Status.PasswordRotation.RequestedAt.Time).To(BeTemporally("==", now))
		Expect(updated.Status.PasswordRotation.LastRotatedAt.Time).To(BeTemporally("==", now))
		Expect(updated.Status.PasswordRotation.PreviousValidUntil.Time).To(BeTemporally("==", now.Add(time.Hour)))
		// The first requeue is short: the next reconcile writes the rotation
		// time on the ValkeyCluster, and from then on the requeue is the period.
		Expect(result.RequeueAfter).To(Equal(time.Second))
		result = reconcileWith(r, "cache-rotate")
		Expect(result.RequeueAfter).To(Equal(time.Hour))
		Expect(getValkeyCluster("cache-rotate").Annotations[cachepkg.PasswordRotatedAtAnnotation]).To(Equal("2026-09-19T08:00:00Z"))

		// A further reconcile inside the period changes nothing.
		reconcileWith(r, "cache-rotate")
		Expect(getSecret(secret.Name).Data).To(Equal(rotated.Data))

		// After the period the previous password goes away and one key is left.
		now = now.Add(time.Hour)
		result = reconcileWith(r, "cache-rotate")
		final := getSecret(secret.Name)
		Expect(final.Data).NotTo(HaveKey(cachepkg.AuthSecretPreviousPasswordKey))
		Expect(final.Data[cachepkg.AuthSecretPasswordKey]).To(Equal(rotated.Data[cachepkg.AuthSecretPasswordKey]))
		Expect(final.Annotations[cachepkg.RotationStepAnnotation]).To(Equal(cachepkg.RotationStepIdle))
		Expect(getValkeyCluster("cache-rotate").Spec.Users[1].PasswordSecret.Keys).To(Equal([]string{cachepkg.AuthSecretPasswordKey}))
		updated = getCache("cache-rotate")
		Expect(rotatedOf(updated).Status).To(Equal(metav1.ConditionTrue))
		Expect(rotatedOf(updated).Reason).To(Equal(ReasonRotated))
		Expect(updated.Status.PasswordRotation.PreviousValidUntil).To(BeNil())
		Expect(result.RequeueAfter).To(BeZero())
	})

	It("carries out a request made during the period when the period ends", func() {
		now := time.Date(2026, 9, 19, 9, 0, 0, 0, time.UTC)
		r := rotationReconciler(&now)
		cache := newCache("cache-rotate-again", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		reconcileWith(r, "cache-rotate-again")
		requestRotation("cache-rotate-again", now)
		reconcileWith(r, "cache-rotate-again")
		second := getSecret(cachepkg.AuthSecretName(cache)).Data[cachepkg.AuthSecretPasswordKey]

		// The new request waits: the password does not change inside the period.
		now = now.Add(time.Minute)
		requestRotation("cache-rotate-again", now)
		reconcileWith(r, "cache-rotate-again")
		Expect(getSecret(cachepkg.AuthSecretName(cache)).Data[cachepkg.AuthSecretPasswordKey]).To(Equal(second))

		// The period ends: the previous password is removed, then the waiting
		// request starts the next rotation.
		now = now.Add(time.Hour)
		reconcileWith(r, "cache-rotate-again")
		Expect(getSecret(cachepkg.AuthSecretName(cache)).Data).NotTo(HaveKey(cachepkg.AuthSecretPreviousPasswordKey))
		reconcileWith(r, "cache-rotate-again")
		third := getSecret(cachepkg.AuthSecretName(cache))
		Expect(third.Data[cachepkg.AuthSecretPreviousPasswordKey]).To(Equal(second))
		Expect(third.Data[cachepkg.AuthSecretPasswordKey]).NotTo(Equal(second))
	})

	It("resets the rotation when the Secret is created again during the period", func() {
		now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
		r := rotationReconciler(&now)
		cache := newCache("cache-rotate-reset", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		reconcileWith(r, "cache-rotate-reset")
		requestRotation("cache-rotate-reset", now)
		reconcileWith(r, "cache-rotate-reset")

		Expect(k8sClient.Delete(ctx, getSecret(cachepkg.AuthSecretName(cache)))).To(Succeed())
		reconcileWith(r, "cache-rotate-reset")

		recreated := getSecret(cachepkg.AuthSecretName(cache))
		Expect(recreated.Data).NotTo(HaveKey(cachepkg.AuthSecretPreviousPasswordKey))
		Expect(recreated.Annotations[cachepkg.RotationStepAnnotation]).To(Equal(cachepkg.RotationStepIdle))
		Expect(getValkeyCluster("cache-rotate-reset").Spec.Users[1].PasswordSecret.Keys).To(Equal([]string{cachepkg.AuthSecretPasswordKey}))
		updated := getCache("cache-rotate-reset")
		Expect(rotatedOf(updated).Status).To(Equal(metav1.ConditionTrue))
		Expect(updated.Status.PasswordRotation.PreviousValidUntil).To(BeNil())
	})

	It("marks a Secret from before the annotation idle and then rotates it", func() {
		now := time.Date(2026, 9, 19, 11, 0, 0, 0, time.UTC)
		r := rotationReconciler(&now)
		cache := newCache("cache-rotate-old", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		reconcileWith(r, "cache-rotate-old")
		old := getSecret(cachepkg.AuthSecretName(cache))
		old.Annotations = nil
		Expect(k8sClient.Update(ctx, old)).To(Succeed())

		requestRotation("cache-rotate-old", now)
		// One reconcile adopts the Secret, the next copies, the third replaces.
		reconcileWith(r, "cache-rotate-old")
		Expect(getSecret(old.Name).Annotations[cachepkg.RotationStepAnnotation]).To(Equal(cachepkg.RotationStepIdle))
		reconcileWith(r, "cache-rotate-old")
		Expect(getSecret(old.Name).Annotations[cachepkg.RotationStepAnnotation]).To(Equal(cachepkg.RotationStepCopied))
		reconcileWith(r, "cache-rotate-old")

		rotated := getSecret(old.Name)
		Expect(rotated.Data[cachepkg.AuthSecretPreviousPasswordKey]).To(Equal(old.Data[cachepkg.AuthSecretPasswordKey]))
		Expect(rotated.Annotations[cachepkg.RotationStepAnnotation]).To(Equal(cachepkg.RotationStepReplaced))
	})

	It("removes the previous password at once when the Cache sets a zero period", func() {
		now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
		r := rotationReconciler(&now)
		cache := newCache("cache-rotate-zero", func(c *cachev1alpha1.Cache) {
			c.Spec.PasswordRotation = &cachev1alpha1.PasswordRotationSpec{PreviousValidFor: &metav1.Duration{}}
		})
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		reconcileWith(r, "cache-rotate-zero")
		first := getSecret(cachepkg.AuthSecretName(cache)).Data[cachepkg.AuthSecretPasswordKey]

		requestRotation("cache-rotate-zero", now)
		reconcileWith(r, "cache-rotate-zero")
		reconcileWith(r, "cache-rotate-zero")

		final := getSecret(cachepkg.AuthSecretName(cache))
		Expect(final.Data).NotTo(HaveKey(cachepkg.AuthSecretPreviousPasswordKey))
		Expect(final.Data[cachepkg.AuthSecretPasswordKey]).NotTo(Equal(first))
		Expect(rotatedOf(getCache("cache-rotate-zero")).Status).To(Equal(metav1.ConditionTrue))
	})
})

var _ = Describe("Cache password rotation of a foreign Secret", func() {
	It("refuses to rotate a Secret that someone else created with the same name", func() {
		now := time.Date(2026, 9, 19, 13, 0, 0, 0, time.UTC)
		r := rotationReconciler(&now)
		cache := newCache("cache-rotate-foreign", nil)
		foreign := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: cachepkg.AuthSecretName(cache), Namespace: testNamespace},
			Data:       map[string][]byte{cachepkg.AuthSecretPasswordKey: []byte("theirs")},
		}
		Expect(k8sClient.Create(ctx, foreign)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, foreign)).To(Succeed()) })
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		reconcileWith(r, "cache-rotate-foreign")

		requestRotation("cache-rotate-foreign", now)
		_, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "cache-rotate-foreign"},
		})
		Expect(err).To(HaveOccurred())

		kept := getSecret(foreign.Name)
		Expect(kept.Data).To(Equal(map[string][]byte{cachepkg.AuthSecretPasswordKey: []byte("theirs")}))
		cond := rotatedOf(getCache("cache-rotate-foreign"))
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.Reason).To(Equal(ReasonRotationFailed))
		Expect(cond.Message).NotTo(ContainSubstring("theirs"))
		Expect(cond.Message).NotTo(ContainSubstring("dGhlaXJz"))
		Expect(getValkeyCluster("cache-rotate-foreign").Spec.Users[1].PasswordSecret.Keys).To(Equal([]string{cachepkg.AuthSecretPasswordKey}))

		// The request counts as handled: the next reconcile does not retry.
		reconcileWith(r, "cache-rotate-foreign")
		Expect(getSecret(foreign.Name).Annotations).NotTo(HaveKey(cachepkg.RotationStepAnnotation))
		Expect(rotatedOf(getCache("cache-rotate-foreign")).Reason).To(Equal(ReasonRotationFailed))
	})
})

var _ = Describe("Cache password rotation after a passing error", func() {
	It("keeps the step and completes the removal on the next reconcile", func() {
		now := time.Date(2026, 9, 19, 14, 0, 0, 0, time.UTC)
		cache := newCache("cache-rotate-retry", func(c *cachev1alpha1.Cache) {
			c.Spec.PasswordRotation = &cachev1alpha1.PasswordRotationSpec{PreviousValidFor: &metav1.Duration{}}
		})
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		good := rotationReconciler(&now)
		reconcileWith(good, "cache-rotate-retry")
		requestRotation("cache-rotate-retry", now)
		reconcileWith(good, "cache-rotate-retry")

		// The remove patch fails once, as an API server error would.
		watchClient, err := client.NewWithWatch(cfg, client.Options{Scheme: k8sClient.Scheme()})
		Expect(err).NotTo(HaveOccurred())
		failing := interceptor.NewClient(watchClient, interceptor.Funcs{
			Patch: func(callCtx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if _, isMeta := obj.(*metav1.PartialObjectMetadata); isMeta && patch.Type() == types.JSONPatchType {
					return errors.New("the API server is not available")
				}
				return c.Patch(callCtx, obj, patch, opts...)
			},
		})
		bad := rotationReconciler(&now)
		bad.Client = failing
		_, err = bad.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "cache-rotate-retry"},
		})
		Expect(err).To(HaveOccurred())
		cond := rotatedOf(getCache("cache-rotate-retry"))
		Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		Expect(cond.Reason).To(Equal(ReasonRotated))
		Expect(cond.Message).To(ContainSubstring("not available"))
		Expect(getSecret(cachepkg.AuthSecretName(cache)).Data).To(HaveKey(cachepkg.AuthSecretPreviousPasswordKey))

		// The next reconcile resumes at the same step and removes the key.
		reconcileWith(good, "cache-rotate-retry")
		Expect(getSecret(cachepkg.AuthSecretName(cache)).Data).NotTo(HaveKey(cachepkg.AuthSecretPreviousPasswordKey))
		Expect(getCache("cache-rotate-retry").Status.PasswordRotation.PreviousValidUntil).To(BeNil())
	})
})

var _ = Describe("Cache password rotation after lost status writes", func() {
	It("finds the step on the Secret and completes the rotation", func() {
		now := time.Date(2026, 9, 19, 15, 0, 0, 0, time.UTC)
		cache := newCache("cache-rotate-lost", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cache)).To(Succeed()) })
		good := rotationReconciler(&now)
		reconcileWith(good, "cache-rotate-lost")
		first := getSecret(cachepkg.AuthSecretName(cache)).Data[cachepkg.AuthSecretPasswordKey]

		// Every status write of the first rotation reconcile is lost, as a
		// crash right after the Secret patches would lose them.
		watchClient, err := client.NewWithWatch(cfg, client.Options{Scheme: k8sClient.Scheme()})
		Expect(err).NotTo(HaveOccurred())
		silent := interceptor.NewClient(watchClient, interceptor.Funcs{
			SubResourcePatch: func(context.Context, client.Client, string, client.Object, client.Patch, ...client.SubResourcePatchOption) error {
				return nil
			},
		})
		requestRotation("cache-rotate-lost", now)
		amnesic := rotationReconciler(&now)
		amnesic.Client = silent
		reconcileWith(amnesic, "cache-rotate-lost")
		Expect(getSecret(cachepkg.AuthSecretName(cache)).Annotations[cachepkg.RotationStepAnnotation]).To(Equal(cachepkg.RotationStepReplaced))
		Expect(rotatedOf(getCache("cache-rotate-lost"))).To(BeNil())

		// The next reconciles see no rotation state, probe the Secret, and
		// record the rotation that already happened.
		reconcileWith(good, "cache-rotate-lost")
		reconcileWith(good, "cache-rotate-lost")
		rotated := getSecret(cachepkg.AuthSecretName(cache))
		Expect(rotated.Data[cachepkg.AuthSecretPreviousPasswordKey]).To(Equal(first))
		Expect(rotated.Data[cachepkg.AuthSecretPasswordKey]).NotTo(Equal(first))
		Expect(rotatedOf(getCache("cache-rotate-lost")).Reason).To(Equal(ReasonPreviousPasswordValid))
		Expect(getValkeyCluster("cache-rotate-lost").Spec.Users[1].PasswordSecret.Keys).To(Equal(
			[]string{cachepkg.AuthSecretPasswordKey, cachepkg.AuthSecretPreviousPasswordKey}))
	})
})
