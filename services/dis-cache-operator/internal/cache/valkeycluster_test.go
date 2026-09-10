package cache

import (
	"slices"
	"strings"
	"testing"

	valkeyv1alpha1 "github.com/valkey-io/valkey-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

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

func userByName(t *testing.T, users []valkeyv1alpha1.UserAclSpec, name string) valkeyv1alpha1.UserAclSpec {
	t.Helper()

	for _, user := range users {
		if user.Name == name {
			return user
		}
	}
	t.Fatalf("user %q not found in %+v", name, users)

	return valkeyv1alpha1.UserAclSpec{}
}

func TestBuildValkeyClusterDefaults(t *testing.T) {
	t.Parallel()

	cluster := BuildValkeyCluster(newTestCache(nil), Images{})

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
	if cluster.Spec.Image != "" || cluster.Spec.Exporter.Image != "" {
		t.Errorf("images: want upstream defaults when unset, got %q / %q", cluster.Spec.Image, cluster.Spec.Exporter.Image)
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

func TestBuildValkeyClusterWithoutPersistenceDisablesSnapshots(t *testing.T) {
	t.Parallel()

	cluster := BuildValkeyCluster(newTestCache(nil), Images{})

	save, ok := cluster.Spec.Config["save"]
	if !ok || save != "" {
		t.Errorf("save: want an empty value to clear the snapshot schedule, got %q (set=%v)", save, ok)
	}
	if got := cluster.Spec.Config["appendonly"]; got != "no" {
		t.Errorf("appendonly: want no, got %q", got)
	}
}

func TestBuildValkeyClusterLargeWithPersistence(t *testing.T) {
	t.Parallel()

	cluster := BuildValkeyCluster(newTestCache(func(c *cachev1alpha1.Cache) {
		c.Spec.Size = cachev1alpha1.CacheSizeLarge
		c.Spec.Persistence = true
		c.Spec.EvictionPolicy = cachev1alpha1.CacheEvictionAllKeysLRU
	}), Images{})

	if cluster.Spec.Replicas != 2 {
		t.Errorf("replicas: want 2, got %d", cluster.Spec.Replicas)
	}
	if got := cluster.Spec.Config["maxmemory-policy"]; got != "allkeys-lru" {
		t.Errorf("maxmemory-policy: want allkeys-lru, got %q", got)
	}
	if _, set := cluster.Spec.Config["save"]; set {
		t.Errorf("save: must not be cleared when persistence is on")
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

func TestBuildValkeyClusterImages(t *testing.T) {
	t.Parallel()

	cluster := BuildValkeyCluster(newTestCache(nil), Images{
		Valkey:   "registry.example/docker.io/valkey/valkey:9.0.0",
		Exporter: "registry.example/docker.io/oliver006/redis_exporter:v1.80.0",
	})

	if cluster.Spec.Image != "registry.example/docker.io/valkey/valkey:9.0.0" {
		t.Errorf("valkey image: got %q", cluster.Spec.Image)
	}
	if cluster.Spec.Exporter.Image != "registry.example/docker.io/oliver006/redis_exporter:v1.80.0" {
		t.Errorf("exporter image: got %q", cluster.Spec.Exporter.Image)
	}
}

func TestBuildValkeyClusterLocksDefaultUser(t *testing.T) {
	t.Parallel()

	cluster := BuildValkeyCluster(newTestCache(nil), Images{})
	defaultUser := userByName(t, cluster.Spec.Users, "default")

	if !defaultUser.ResetPass {
		t.Error("default user: want resetpass so no password can authenticate")
	}
	if !slices.Contains(defaultUser.Commands.Deny, aclAllCommands) {
		t.Errorf("default user: want -@all, got deny %v", defaultUser.Commands.Deny)
	}
	if defaultUser.PasswordSecret.Name != "" {
		t.Errorf("default user: must not reference a password Secret, got %q", defaultUser.PasswordSecret.Name)
	}
}

func TestBuildValkeyClusterAppUser(t *testing.T) {
	t.Parallel()

	cluster := BuildValkeyCluster(newTestCache(nil), Images{})
	appUser := userByName(t, cluster.Spec.Users, AuthUsername)

	if !appUser.Enabled {
		t.Error("app user: want enabled")
	}
	if appUser.PasswordSecret.Name != "app-one-cache-cache-auth" {
		t.Errorf("app user: want the auth Secret, got %q", appUser.PasswordSecret.Name)
	}
	if !slices.Equal(appUser.PasswordSecret.Keys, []string{AuthSecretPasswordKey}) {
		t.Errorf("app user: want password key %q, got %v", AuthSecretPasswordKey, appUser.PasswordSecret.Keys)
	}
	if !slices.Equal(appUser.Commands.Allow, []string{aclAllCommands}) {
		t.Errorf("app user: want +@all, got allow %v", appUser.Commands.Allow)
	}
	if !slices.Equal(appUser.Commands.Deny, []string{aclAdminCommands, aclDangerousCommands}) {
		t.Errorf("app user: want -@admin -@dangerous, got deny %v", appUser.Commands.Deny)
	}
	if !slices.Equal(appUser.Keys.ReadWrite, []string{"*"}) {
		t.Errorf("app user: want all keys, got %v", appUser.Keys.ReadWrite)
	}
	if !slices.Equal(appUser.Channels.Patterns, []string{"*"}) {
		t.Errorf("app user: want all channels, got %v", appUser.Channels.Patterns)
	}
}

func TestBuildValkeyClusterSecurityContext(t *testing.T) {
	t.Parallel()

	cluster := BuildValkeyCluster(newTestCache(nil), Images{})

	pod := cluster.Spec.PodSecurityContext
	if pod == nil || pod.RunAsNonRoot == nil || !*pod.RunAsNonRoot {
		t.Fatalf("pod security context: want runAsNonRoot, got %+v", pod)
	}
	for name, got := range map[string]*int64{"runAsUser": pod.RunAsUser, "runAsGroup": pod.RunAsGroup, "fsGroup": pod.FSGroup} {
		if got == nil || *got != 999 {
			t.Errorf("%s: want 999, got %v", name, got)
		}
	}
	if pod.SeccompProfile == nil || pod.SeccompProfile.Type != "RuntimeDefault" {
		t.Errorf("seccomp: want RuntimeDefault, got %+v", pod.SeccompProfile)
	}

	if len(cluster.Spec.Containers) != 2 {
		t.Fatalf("containers: want patches for server and metrics-exporter, got %d", len(cluster.Spec.Containers))
	}
	for _, container := range cluster.Spec.Containers {
		sc := container.SecurityContext
		if sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
			t.Errorf("%s: want allowPrivilegeEscalation=false", container.Name)
		}
		if sc == nil || sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
			t.Errorf("%s: want readOnlyRootFilesystem=true", container.Name)
		}
		if sc == nil || sc.Capabilities == nil || !slices.Contains(sc.Capabilities.Drop, "ALL") {
			t.Errorf("%s: want all capabilities dropped", container.Name)
		}
	}
}

func TestValkeyServiceNameUsesUpstreamPrefix(t *testing.T) {
	t.Parallel()

	if got := ValkeyServiceName(newTestCache(nil)); got != "valkey-app-one-cache" {
		t.Errorf("service name: want valkey-app-one-cache, got %q", got)
	}
}

func TestBuildValkeyClusterEmptyEvictionPolicyFallsBack(t *testing.T) {
	t.Parallel()

	cluster := BuildValkeyCluster(newTestCache(func(c *cachev1alpha1.Cache) {
		c.Spec.EvictionPolicy = ""
	}), Images{})

	if got := cluster.Spec.Config["maxmemory-policy"]; got != "noeviction" {
		t.Errorf("maxmemory-policy: want noeviction, got %q", got)
	}
}

func TestValkeyServiceNameFitsLongestAdmittedCacheName(t *testing.T) {
	t.Parallel()

	longest := newTestCache(func(c *cachev1alpha1.Cache) {
		c.Name = strings.Repeat("a", cachev1alpha1.MaxCacheNameLength)
	})
	serviceName := ValkeyServiceName(longest)
	if errs := validation.IsDNS1035Label(serviceName); len(errs) != 0 {
		t.Errorf("service name %q for the longest admitted Cache name is not a valid Service name: %v", serviceName, errs)
	}
	if got := len(serviceName); got != validation.DNS1035LabelMaxLength {
		t.Errorf("service name length: want %d so the CRD limit is not stricter than needed, got %d", validation.DNS1035LabelMaxLength, got)
	}
}
