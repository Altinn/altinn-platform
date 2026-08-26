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

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
)

const testNamespace = "default"

func newCache(name string, mutate func(*cachev1alpha1.Cache)) *cachev1alpha1.Cache {
	cache := &cachev1alpha1.Cache{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
	}
	if mutate != nil {
		mutate(cache)
	}

	return cache
}

var _ = Describe("Cache CRD schema", func() {
	It("admits an empty-spec Cache and applies platform defaults", func() {
		cache := newCache("cache-defaults", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, cache)).To(Succeed())
		})

		var stored cachev1alpha1.Cache
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "cache-defaults"}, &stored)).To(Succeed())
		Expect(stored.Spec.Size).To(Equal(cachev1alpha1.CacheSizeSmall))
		Expect(stored.Spec.EvictionPolicy).To(Equal(cachev1alpha1.CacheEvictionNoEviction))
		Expect(stored.Spec.Persistence).To(BeFalse())
	})

	It("admits a Cache with explicit values", func() {
		cache := newCache("cache-explicit", func(c *cachev1alpha1.Cache) {
			c.Spec.Size = cachev1alpha1.CacheSizeLarge
			c.Spec.Persistence = true
			c.Spec.EvictionPolicy = cachev1alpha1.CacheEvictionAllKeysLRU
		})
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, cache)).To(Succeed())
		})
	})

	It("rejects an unknown size value", func() {
		cache := newCache("cache-bad-size", func(c *cachev1alpha1.Cache) {
			c.Spec.Size = "gigantic"
		})
		err := k8sClient.Create(ctx, cache)
		Expect(err).To(HaveOccurred())
	})

	It("rejects an unknown eviction policy", func() {
		cache := newCache("cache-bad-eviction", func(c *cachev1alpha1.Cache) {
			c.Spec.EvictionPolicy = "evict-everything"
		})
		err := k8sClient.Create(ctx, cache)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Cache reconciler", func() {
	It("reconciles an existing Cache without error", func() {
		cache := newCache("cache-reconcile", nil)
		Expect(k8sClient.Create(ctx, cache)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, cache)).To(Succeed())
		})

		reconciler := &CacheReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		result, err := reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "cache-reconcile"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))
	})

	It("returns cleanly for a Cache that no longer exists", func() {
		reconciler := &CacheReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		result, err := reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "cache-missing"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))
	})
})
