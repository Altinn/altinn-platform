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
	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ApiVersion Controller", func() {

	Context("When reconciling an ApiVersion resource", func() {
		const resourceName = "test-apiversion"
		const resourceNamespace = "default"

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
						Name:        utils.ToPointer("v1"),
						Content:     utils.ToPointer(`{"openapi": "3.0.0","info": {"title": "Minimal API v1","version": "1.0.0"},"paths": {}}`),
						DisplayName: "the default version",
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
				updatedApiVersion.Spec.Content = utils.ToPointer(`{"openapi": "3.0.0","info": {"title": "Minimal API v1","version": "1.0.0"},"paths": {"test": "test"}}`)
				g.Expect(k8sClient.Update(ctx, &updatedApiVersion)).To(Succeed())
			}, timeout, interval).Should(Succeed(), "apiVersion content should eventually be updated")

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedApiVersion)).To(Succeed())
				g.Expect(*updatedApiVersion.Spec.Content).To(Equal(`{"openapi": "3.0.0","info": {"title": "Minimal API v1","version": "1.0.0"},"paths": {"test": "test"}}`))
				g.Expect(fakeApim.APIMVersions).To(HaveLen(1))
				g.Expect(fakeApim.Policies).To(HaveLen(0))
				g.Expect(fakeApim.ApiDiagnostics).To(HaveLen(0))
			}, timeout, interval).Should(Succeed(), "apiVersion content should eventually be updated")

			By("adding policy and diagnostics to the apiversion if they are set")
			Eventually(func(g Gomega) {
				updatedApiVersion = getUpdatedApiVersion(ctx, typeNamespacedName)
				updatedApiVersion.Spec.Policies = &apimv1alpha1.ApiPolicySpec{
					PolicyContent: utils.ToPointer(`<inbound><base/></inbound>`),
					PolicyFormat:  utils.ToPointer(apimv1alpha1.PolicyContentFormatXML),
				}
				updatedApiVersion.Spec.Diagnostics = &apimv1alpha1.ApiDiagnosticSpec{
					LoggerName:         utils.ToPointer("custom-api-logger"),
					SamplingPercentage: utils.ToPointer(int32(10)),
				}
				g.Expect(k8sClient.Update(ctx, &updatedApiVersion)).To(Succeed())
			}, timeout, interval).Should(Succeed(), "apiVersion content should eventually be updated")

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &updatedApiVersion)).To(Succeed())
				g.Expect(fakeApim.APIMVersions).To(HaveLen(1))
				g.Expect(fakeApim.Policies).To(HaveLen(1))
				g.Expect(fakeApim.ApiDiagnostics).To(HaveLen(2))
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
	})
})

func getUpdatedApiVersion(ctx context.Context, typeNamespacedName types.NamespacedName) apimv1alpha1.ApiVersion {
	resource := apimv1alpha1.ApiVersion{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, typeNamespacedName, &resource)).To(Succeed())
	}, timeout, interval).Should(Succeed())
	return resource
}
