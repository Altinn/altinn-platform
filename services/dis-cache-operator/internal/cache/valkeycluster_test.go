package cache

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
)

func newTestCache(mutate func(*cachev1alpha1.Cache)) *cachev1alpha1.Cache {
	testCache := &cachev1alpha1.Cache{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-one-cache",
			Namespace: "team-a",
		},
		Spec: cachev1alpha1.CacheSpec{
			Size:           cachev1alpha1.CacheSizeSmall,
			EvictionPolicy: cachev1alpha1.CacheEvictionNoEviction,
		},
	}
	if mutate != nil {
		mutate(testCache)
	}

	return testCache
}

func TestBuildValkeyClusterDefaults(t *testing.T) {
	t.Parallel()

	cluster := BuildValkeyCluster(newTestCache(nil))

	if cluster.Name != "app-one-cache" || cluster.Namespace != "team-a" {
		t.Fatalf("unexpected name/namespace: %s/%s", cluster.Namespace, cluster.Name)
	}
	if cluster.Labels[ManagedByLabel] != ManagedByValue {
		t.Errorf("missing managed-by label, got %v", cluster.Labels)
	}
	if cluster.Labels[CacheNameLabel] != "app-one-cache" {
		t.Errorf("missing cache name label, got %v", cluster.Labels)
	}
	if cluster.Spec.Shards != 1 {
		t.Errorf("shards: want 1, got %d", cluster.Spec.Shards)
	}
	if cluster.Spec.Replicas != 1 {
		t.Errorf("replicas: want 1, got %d", cluster.Spec.Replicas)
	}
	if want := ProfileFor(cachev1alpha1.CacheSizeSmall).MaxMemory; cluster.Spec.Config["maxmemory"] != want {
		t.Errorf("maxmemory: want %s, got %q", want, cluster.Spec.Config["maxmemory"])
	}
	if got := cluster.Spec.Config["maxmemory-policy"]; got != "noeviction" {
		t.Errorf("maxmemory-policy: want noeviction, got %q", got)
	}
	if cluster.Spec.Persistence != nil {
		t.Errorf("persistence: want nil, got %+v", cluster.Spec.Persistence)
	}

	memory := cluster.Spec.Resources.Limits.Memory()
	if memory == nil || memory.String() != "256Mi" {
		t.Errorf("memory limit: want 256Mi, got %v", memory)
	}
}

func TestBuildValkeyClusterLargeWithPersistence(t *testing.T) {
	t.Parallel()

	cluster := BuildValkeyCluster(newTestCache(func(c *cachev1alpha1.Cache) {
		c.Spec.Size = cachev1alpha1.CacheSizeLarge
		c.Spec.Persistence = true
		c.Spec.EvictionPolicy = cachev1alpha1.CacheEvictionAllKeysLRU
	}))

	if cluster.Spec.Replicas != 2 {
		t.Errorf("replicas: want 2, got %d", cluster.Spec.Replicas)
	}
	if got := cluster.Spec.Config["maxmemory-policy"]; got != "allkeys-lru" {
		t.Errorf("maxmemory-policy: want allkeys-lru, got %q", got)
	}
	if cluster.Spec.Persistence == nil {
		t.Fatal("persistence: want set, got nil")
	}
	if got := cluster.Spec.Persistence.Size.String(); got != "8Gi" {
		t.Errorf("pvc size: want 8Gi, got %s", got)
	}
	if string(cluster.Spec.Persistence.ReclaimPolicy) != "Delete" {
		t.Errorf("reclaim policy: want Delete, got %s", cluster.Spec.Persistence.ReclaimPolicy)
	}
}

func TestBuildValkeyClusterEmptyEvictionPolicyFallsBack(t *testing.T) {
	t.Parallel()

	cluster := BuildValkeyCluster(newTestCache(func(c *cachev1alpha1.Cache) {
		c.Spec.EvictionPolicy = ""
	}))

	if got := cluster.Spec.Config["maxmemory-policy"]; got != "noeviction" {
		t.Errorf("maxmemory-policy: want noeviction, got %q", got)
	}
}
