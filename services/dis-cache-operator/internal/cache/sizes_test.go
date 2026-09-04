package cache

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
)

func TestProfileFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		size          cachev1alpha1.CacheSize
		wantReplicas  int32
		wantCPU       string
		wantMemory    string
		wantMaxMemory string
		wantPVCSize   string
	}{
		{
			name:          "small",
			size:          cachev1alpha1.CacheSizeSmall,
			wantReplicas:  1,
			wantCPU:       "100m",
			wantMemory:    "256Mi",
			wantMaxMemory: "192mb",
			wantPVCSize:   "1Gi",
		},
		{
			name:          "medium",
			size:          cachev1alpha1.CacheSizeMedium,
			wantReplicas:  1,
			wantCPU:       "250m",
			wantMemory:    "1Gi",
			wantMaxMemory: "768mb",
			wantPVCSize:   "2Gi",
		},
		{
			name:          "large",
			size:          cachev1alpha1.CacheSizeLarge,
			wantReplicas:  2,
			wantCPU:       "500m",
			wantMemory:    "4Gi",
			wantMaxMemory: "3gb",
			wantPVCSize:   "8Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			profile := ProfileFor(tt.size)
			if profile.Replicas != tt.wantReplicas {
				t.Errorf("replicas: want %d, got %d", tt.wantReplicas, profile.Replicas)
			}
			if profile.CPU.String() != tt.wantCPU {
				t.Errorf("cpu: want %s, got %s", tt.wantCPU, profile.CPU.String())
			}
			if profile.Memory.String() != tt.wantMemory {
				t.Errorf("memory: want %s, got %s", tt.wantMemory, profile.Memory.String())
			}
			if profile.MaxMemory != tt.wantMaxMemory {
				t.Errorf("maxmemory: want %s, got %s", tt.wantMaxMemory, profile.MaxMemory)
			}
			if profile.PVCSize.String() != tt.wantPVCSize {
				t.Errorf("pvc size: want %s, got %s", tt.wantPVCSize, profile.PVCSize.String())
			}
		})
	}
}

func TestProfileForUnknownSizeFallsBackToSmall(t *testing.T) {
	t.Parallel()

	small := ProfileFor(cachev1alpha1.CacheSizeSmall)

	for _, size := range []cachev1alpha1.CacheSize{"", "gigantic"} {
		if !reflect.DeepEqual(ProfileFor(size), small) {
			t.Errorf("ProfileFor(%q): want the small profile", size)
		}
	}
}

func TestSizeProfileResources(t *testing.T) {
	t.Parallel()

	resources := ProfileFor(cachev1alpha1.CacheSizeSmall).Resources()

	for _, name := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
		request, ok := resources.Requests[name]
		if !ok {
			t.Fatalf("missing %s request", name)
		}
		limit, ok := resources.Limits[name]
		if !ok {
			t.Fatalf("missing %s limit", name)
		}
		if request.Cmp(limit) != 0 {
			t.Errorf("%s: request %s does not equal limit %s", name, request.String(), limit.String())
		}
	}
}
