/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
)

// SizeProfile holds the platform values one spec.size maps to.
type SizeProfile struct {
	// Replicas is the number of replicas per shard (failover copies).
	Replicas int32
	// CPU and Memory are used as both request and limit.
	CPU    resource.Quantity
	Memory resource.Quantity
	// MaxMemory is the Valkey maxmemory config value. It stays below the
	// container memory limit so the eviction policy acts before the kernel
	// kills the pod.
	MaxMemory string
	// PVCSize is the volume size when persistence is on.
	PVCSize resource.Quantity
}

var sizeProfiles = map[cachev1alpha1.CacheSize]SizeProfile{
	cachev1alpha1.CacheSizeSmall: {
		Replicas:  1,
		CPU:       resource.MustParse("100m"),
		Memory:    resource.MustParse("256Mi"),
		MaxMemory: "192mb",
		PVCSize:   resource.MustParse("1Gi"),
	},
	cachev1alpha1.CacheSizeMedium: {
		Replicas:  1,
		CPU:       resource.MustParse("250m"),
		Memory:    resource.MustParse("1Gi"),
		MaxMemory: "768mb",
		PVCSize:   resource.MustParse("2Gi"),
	},
	cachev1alpha1.CacheSizeLarge: {
		Replicas:  2,
		CPU:       resource.MustParse("500m"),
		Memory:    resource.MustParse("4Gi"),
		MaxMemory: "3gb",
		PVCSize:   resource.MustParse("8Gi"),
	},
}

// ProfileFor returns the platform values for a size. An empty size returns
// the small profile: the CRD defaults spec.size to small, so an empty value
// only appears on objects that did not pass the API server.
func ProfileFor(size cachev1alpha1.CacheSize) SizeProfile {
	if profile, ok := sizeProfiles[size]; ok {
		return profile
	}

	return sizeProfiles[cachev1alpha1.CacheSizeSmall]
}

// Resources returns the container resource requirements for the profile.
// Requests equal limits, so cache pods get the Guaranteed QoS class.
func (p SizeProfile) Resources() corev1.ResourceRequirements {
	list := corev1.ResourceList{
		corev1.ResourceCPU:    p.CPU,
		corev1.ResourceMemory: p.Memory,
	}

	return corev1.ResourceRequirements{
		Requests: list,
		Limits:   list,
	}
}
