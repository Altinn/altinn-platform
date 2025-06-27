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

	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	apimv1alpha1 "github.com/Altinn/altinn-platform/services/dis-apim-operator/api/v1alpha1"
)

var _ = Describe("Backend Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		const defaultNamespace = "default-test"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: defaultNamespace,
		}

		It("should set success status when azure resource created", func() {
			resource := &apimv1alpha1.Backend{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: defaultNamespace,
				},
				Spec: apimv1alpha1.BackendSpec{
					Title:       "test-backend",
					Description: utils.ToPointer("Test backend for the operator"),
					Url:         "https://test.example.com",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			// Fetch the updated Backend resource
			updatedBackend := &apimv1alpha1.Backend{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedBackend)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(updatedBackend.Status.ProvisioningState).To(Equal(apimv1alpha1.BackendProvisioningStateSucceeded))
				g.Expect(updatedBackend.Status.BackendID).To(Equal("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/" + updatedBackend.GetAzureResourceName()))
				g.Expect(updatedBackend.Status.ProvisioningState).To(Equal(apimv1alpha1.BackendProvisioningStateSucceeded))
				g.Expect(fakeApim.Backends).To(HaveLen(1))
			}, timeout, interval).Should(Succeed())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			By("Updating the apim Backend if it does not match the desired state")
			Eventually(func(g Gomega) {
				updatedBackend.Spec.Url = "https://updated.example.com"
				err := k8sClient.Update(ctx, updatedBackend)
				Expect(err).NotTo(HaveOccurred())
			}, timeout, interval).Should(Succeed())
			// Fetch the updated Backend resource
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedBackend)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(updatedBackend.Status.ProvisioningState).To(Equal(apimv1alpha1.BackendProvisioningStateSucceeded))
				g.Expect(updatedBackend.Status.BackendID).To(Equal("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/" + updatedBackend.GetAzureResourceName()))
				g.Expect(fakeApim.Backends).To(HaveLen(1))
				g.Expect(fakeApim.Backends).To(HaveKey(updatedBackend.GetAzureResourceName()))
				g.Expect(*fakeApim.Backends[updatedBackend.GetAzureResourceName()].Properties.URL).To(Equal(updatedBackend.Spec.Url))
			}, timeout, interval).Should(Succeed())
			By("Deleting the apim Backend and removing the finalizer when the resource is deleted")
			Eventually(k8sClient.Delete).WithArguments(ctx, updatedBackend).Should(Succeed())
			// Fetch the updated Backend resource
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedBackend)
				g.Expect(err).To(HaveOccurred())
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
				g.Expect(fakeApim.Backends).To(BeEmpty())
			}, timeout, interval).Should(Succeed())
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
