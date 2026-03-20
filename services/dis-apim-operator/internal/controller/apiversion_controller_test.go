/*
Copyright 2024 altinn.

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
	"context"

	apimv1alpha1 "github.com/Altinn/altinn-platform/services/dis-apim-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ApiVersion Controller", func() {

	Context("When reconciling an ApiVersion resource", func() {
		const resourceName = "test-apiversion"
		const resourceNamespace = "default-test"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		It("should successfully reconcile the ApiVersion resource", func() {
			resource := &apimv1alpha1.ApiVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: apimv1alpha1.ApiVersionSpec{
					Path: "/test-api",
					ApiVersionSubSpec: apimv1alpha1.ApiVersionSubSpec{
						Name:        ptr.To("v1"),
						Content:     ptr.To(`{"openapi": "3.0.0","info": {"title": "Minimal API v1","version": "1.0.0"},"paths": {}}`),
						DisplayName: "the default version",
					},
					Contact: &apimv1alpha1.APIContactInformation{
						Name:  ptr.To("Test Contact"),
						Email: ptr.To("test@example.com"),
						URL:   ptr.To("https://example.com/contact"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			updatedApiVersion := getUpdatedApiVersion(ctx, typeNamespacedName)
			Expect(updatedApiVersion.Spec.DisplayName).To(Equal("the default version"))
			Expect(*updatedApiVersion.Spec.Name).To(Equal("v1"))
			Expect(*updatedApiVersion.Spec.Content).To(Equal(`{"openapi": "3.0.0","info": {"title": "Minimal API v1","version": "1.0.0"},"paths": {}}`))

			By("updating the openapi content in the apiversion if it has changed")
			Eventually(func(g Gomega) {
				updatedApiVersion = getUpdatedApiVersion(ctx, typeNamespacedName)
				updatedApiVersion.Spec.Content = ptr.To(`{"openapi": "3.0.0","info": {"title": "Minimal API v1","version": "1.0.0"},"paths": {"test": "test"}}`)
				g.Expect(k8sClient.Update(ctx, &updatedApiVersion)).To(Succeed())
			}, timeout, interval).Should(Succeed(), "apiVersion content should eventually be updated")

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedApiVersion)).To(Succeed())
				g.Expect(*updatedApiVersion.Spec.Content).To(Equal(`{"openapi": "3.0.0","info": {"title": "Minimal API v1","version": "1.0.0"},"paths": {"test": "test"}}`))
				g.Expect(fakeApim.APIMVersions).To(HaveLen(1))
				g.Expect(fakeApim.Policies).To(BeEmpty())
				g.Expect(fakeApim.ApiDiagnostics).To(BeEmpty())
			}, timeout, interval).Should(Succeed(), "apiVersion content should eventually be updated")

			By("adding policy and diagnostics to the apiversion if they are set")
			Eventually(func(g Gomega) {
				updatedApiVersion = getUpdatedApiVersion(ctx, typeNamespacedName)
				updatedApiVersion.Spec.Policies = &apimv1alpha1.ApiPolicySpec{
					PolicyContent: ptr.To(`<inbound><base/></inbound>`),
					PolicyFormat:  ptr.To(apimv1alpha1.PolicyContentFormatXML),
				}
				updatedApiVersion.Spec.Diagnostics = &apimv1alpha1.ApiDiagnosticSpec{
					LoggerName:         ptr.To("custom-api-logger"),
					SamplingPercentage: ptr.To(int32(10)),
				}
				g.Expect(k8sClient.Update(ctx, &updatedApiVersion)).To(Succeed())
			}, timeout, interval).Should(Succeed(), "apiVersion content should eventually be updated")

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedApiVersion)).To(Succeed())
				g.Expect(fakeApim.APIMVersions).To(HaveLen(1))
				g.Expect(fakeApim.Policies).To(HaveLen(1))
				g.Expect(fakeApim.ApiDiagnostics).To(HaveLen(2))
			}, timeout, interval).Should(Succeed(), "apiVersion content should eventually be updated")

			By("removing policy and diagnostics to the apiversion if they are nil")
			Eventually(func(g Gomega) {
				updatedApiVersion = getUpdatedApiVersion(ctx, typeNamespacedName)
				updatedApiVersion.Spec.Policies = nil
				updatedApiVersion.Spec.Diagnostics = nil
				g.Expect(k8sClient.Update(ctx, &updatedApiVersion)).To(Succeed())
			}, timeout, interval).Should(Succeed(), "apiVersion content should eventually be updated")

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedApiVersion)).To(Succeed())
				g.Expect(fakeApim.APIMVersions).To(HaveLen(1))
				g.Expect(fakeApim.Policies).To(BeEmpty())
				g.Expect(fakeApim.ApiDiagnostics).To(BeEmpty())
			}, timeout, interval).Should(Succeed(), "apiVersion content should eventually be updated")

			By("deleting the apiversion if it has been marked for deletion")
			err := k8sClient.Get(ctx, typeNamespacedName, &updatedApiVersion)
			Expect(err).NotTo(HaveOccurred())
			Eventually(k8sClient.Delete).WithArguments(ctx, &updatedApiVersion).Should(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, &updatedApiVersion)
				g.Expect(err).To(HaveOccurred())
			}, timeout, interval).Should(Succeed(), "apiVersion should eventually be deleted")
		})

		It("should clear stale resume token when delete is attempted with create/update token", func() {
			const localName = "test-apiversion-stale-token"
			localNamespacedName := types.NamespacedName{Name: localName, Namespace: resourceNamespace}

			resource := &apimv1alpha1.ApiVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      localName,
					Namespace: resourceNamespace,
				},
				Spec: apimv1alpha1.ApiVersionSpec{
					Path: "/test-stale-token",
					ApiVersionSubSpec: apimv1alpha1.ApiVersionSubSpec{
						Name:        ptr.To("v1"),
						Content:     ptr.To(`{"openapi": "3.0.0","info": {"title": "Stale Token API","version": "1.0.0"},"paths": {}}`),
						DisplayName: "stale token test version",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("waiting for the resource to reach Succeeded state")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, localNamespacedName, resource)).To(Succeed())
				g.Expect(resource.Status.ProvisioningState).To(Equal(apimv1alpha1.ProvisioningStateSucceeded))
			}, timeout, interval).Should(Succeed())

			By("patching status with a stale resume token")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, localNamespacedName, resource)).To(Succeed())
				orig := resource.DeepCopy()
				resource.Status.ResumeToken = "stale-create-token"
				resource.Status.ProvisioningState = apimv1alpha1.ProvisioningStateUpdating
				g.Expect(k8sClient.Status().Patch(ctx, resource, client.MergeFrom(orig))).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("deleting the resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("verifying the resource is fully deleted")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, localNamespacedName, resource)
				g.Expect(err).To(HaveOccurred())
			}, timeout, interval).Should(Succeed())

			Expect(fakeApim.APIMVersions).NotTo(HaveKey(resource.GetApiVersionAzureFullName()))
		})

		It("should recover from failed LRO during create/update", func() {
			const localName = "test-apiversion-lro-create-fail"
			localNamespacedName := types.NamespacedName{Name: localName, Namespace: resourceNamespace}

			resource := &apimv1alpha1.ApiVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      localName,
					Namespace: resourceNamespace,
				},
				Spec: apimv1alpha1.ApiVersionSpec{
					Path: "/test-lro-create-fail",
					ApiVersionSubSpec: apimv1alpha1.ApiVersionSubSpec{
						Name:        ptr.To("v1"),
						Content:     ptr.To(`{"openapi": "3.0.0","info": {"title": "LRO Fail API","version": "1.0.0"},"paths": {}}`),
						DisplayName: "lro create fail test version",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("waiting for the resource to reach Succeeded state")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, localNamespacedName, resource)).To(Succeed())
				g.Expect(resource.Status.ProvisioningState).To(Equal(apimv1alpha1.ProvisioningStateSucceeded))
			}, timeout, interval).Should(Succeed())

			By("enabling LRO failure for create/update")
			fakeApim.CreateUpdateApiLROFail = true

			By("triggering a reconcile by updating the spec")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, localNamespacedName, resource)).To(Succeed())
				resource.Spec.Content = ptr.To(`{"openapi": "3.0.0","info": {"title": "LRO Fail API updated","version": "1.0.0"},"paths": {}}`)
				g.Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("verifying the resource reaches Failed state with empty resume token")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, localNamespacedName, resource)).To(Succeed())
				g.Expect(resource.Status.ProvisioningState).To(Equal(apimv1alpha1.ProvisioningStateFailed))
				g.Expect(resource.Status.ResumeToken).To(BeEmpty())
			}, timeout, interval).Should(Succeed())

			By("disabling LRO failure and triggering recovery")
			fakeApim.CreateUpdateApiLROFail = false

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, localNamespacedName, resource)).To(Succeed())
				resource.Spec.Content = ptr.To(`{"openapi": "3.0.0","info": {"title": "LRO Fail API recovered","version": "1.0.0"},"paths": {}}`)
				g.Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("verifying the resource reaches Succeeded state")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, localNamespacedName, resource)).To(Succeed())
				g.Expect(resource.Status.ProvisioningState).To(Equal(apimv1alpha1.ProvisioningStateSucceeded))
			}, timeout, interval).Should(Succeed())

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, localNamespacedName, resource)
				g.Expect(err).To(HaveOccurred())
			}, timeout, interval).Should(Succeed())
		})

		It("should recover from failed LRO during delete", func() {
			const localName = "test-apiversion-lro-delete-fail"
			localNamespacedName := types.NamespacedName{Name: localName, Namespace: resourceNamespace}

			resource := &apimv1alpha1.ApiVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      localName,
					Namespace: resourceNamespace,
				},
				Spec: apimv1alpha1.ApiVersionSpec{
					Path: "/test-lro-delete-fail",
					ApiVersionSubSpec: apimv1alpha1.ApiVersionSubSpec{
						Name:        ptr.To("v1"),
						Content:     ptr.To(`{"openapi": "3.0.0","info": {"title": "LRO Delete Fail API","version": "1.0.0"},"paths": {}}`),
						DisplayName: "lro delete fail test version",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("waiting for the resource to reach Succeeded state")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, localNamespacedName, resource)).To(Succeed())
				g.Expect(resource.Status.ProvisioningState).To(Equal(apimv1alpha1.ProvisioningStateSucceeded))
			}, timeout, interval).Should(Succeed())

			By("enabling LRO failure for delete")
			fakeApim.DeleteApiLROFail = true

			By("deleting the resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("verifying the resource reaches Failed state with empty resume token")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, localNamespacedName, resource)).To(Succeed())
				g.Expect(resource.Status.ProvisioningState).To(Equal(apimv1alpha1.ProvisioningStateFailed))
				g.Expect(resource.Status.ResumeToken).To(BeEmpty())
			}, timeout, interval).Should(Succeed())

			By("disabling LRO failure to allow retry")
			fakeApim.DeleteApiLROFail = false

			By("verifying the resource is eventually fully deleted")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, localNamespacedName, resource)
				g.Expect(err).To(HaveOccurred())
			}, timeout, interval).Should(Succeed())
		})
	})
})

func getUpdatedApiVersion(ctx context.Context, typeNamespacedName types.NamespacedName) apimv1alpha1.ApiVersion {
	resource := apimv1alpha1.ApiVersion{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, typeNamespacedName, &resource)).To(Succeed())
	}, timeout, interval).Should(Succeed())
	return resource
}
