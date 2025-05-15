/*
Copyright 2025 Altinn.

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	managedidentity "github.com/Azure/azure-service-operator/v2/api/managedidentity/v1api20230131"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	applicationv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-identity-operator/internal/utils"
)

var _ = Describe("ApplicationIdentity Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		applicationidentity := &applicationv1alpha1.ApplicationIdentity{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ApplicationIdentity")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, applicationidentity)
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
				resource := &applicationv1alpha1.ApplicationIdentity{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				g.Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &applicationv1alpha1.ApplicationIdentity{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ApplicationIdentity")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				uaIdentity := &applicationv1alpha1.ApplicationIdentity{}
				g.Expect(errors.IsNotFound(k8sClient.Get(ctx, typeNamespacedName, resource))).To(BeTrue())
				g.Expect(errors.IsNotFound(k8sClient.Get(ctx, typeNamespacedName, uaIdentity))).To(BeTrue())
			}, timeout, interval).ShouldNot(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			appIdentity := &applicationv1alpha1.ApplicationIdentity{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, appIdentity)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(appIdentity.Status.Conditions).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
		It("should create UserAssignedIdentity object", func() {
			appIdentity := &applicationv1alpha1.ApplicationIdentity{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, appIdentity)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(appIdentity.Status.Conditions).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())
			uaID := &managedidentity.UserAssignedIdentity{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, uaID)
				g.Expect(err).NotTo(HaveOccurred())
			}, timeout, interval).Should(Succeed())
		})
		It("should update ApplicationIdentity status and create Creds when UAID is updated", func() {
			appIdentity := &applicationv1alpha1.ApplicationIdentity{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, appIdentity)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(appIdentity.Status.Conditions).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())
			uaID := &managedidentity.UserAssignedIdentity{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, uaID)
				g.Expect(err).NotTo(HaveOccurred())
			}, timeout, interval).Should(Succeed())
			// Update the UAID status
			uaID.Status.Conditions = []conditions.Condition{
				{
					Type:               "Ready",
					Status:             "True",
					Reason:             "Succeeded",
					ObservedGeneration: uaID.Generation,
					LastTransitionTime: metav1.Now(),
				},
			}
			uaID.Status.ClientId = utils.ToPointer("325e4fc8-5e58-4942-be61-11b8ee679ff2")
			uaID.Status.PrincipalId = utils.ToPointer("3fb69913-169d-4c23-8ab7-39278f71d314")
			Eventually(func(g Gomega) {
				err := k8sClient.Status().Update(ctx, uaID)
				g.Expect(err).NotTo(HaveOccurred())
			}, timeout, interval).Should(Succeed())
			// Verify that the ApplicationIdentity status is updated
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, appIdentity)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(appIdentity.Status.PrincipalID).To(Equal(uaID.Status.PrincipalId))
				g.Expect(appIdentity.Status.ClientID).To(Equal(uaID.Status.ClientId))
			}, timeout, interval).Should(Succeed())
			// Verify that the FederatedIdentityCredential is created
			federatedCredential := &managedidentity.FederatedIdentityCredential{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, federatedCredential)
				g.Expect(err).NotTo(HaveOccurred())
			}, timeout, interval).Should(Succeed())
		})
		It("should update ApplicationIdentity status and create ServiceAccount when Creds is updated", func() {
			appIdentity := &applicationv1alpha1.ApplicationIdentity{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, appIdentity)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(appIdentity.Status.Conditions).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())
			uaID := &managedidentity.UserAssignedIdentity{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, uaID)
				g.Expect(err).NotTo(HaveOccurred())
			}, timeout, interval).Should(Succeed())
			// Update the UAID status
			uaID.Status.Conditions = []conditions.Condition{
				{
					Type:               "Ready",
					Status:             "True",
					Reason:             "Succeeded",
					ObservedGeneration: uaID.Generation,
					LastTransitionTime: metav1.Now(),
				},
			}
			uaID.Status.ClientId = utils.ToPointer("325e4fc8-5e58-4942-be61-11b8ee679ff2")
			uaID.Status.PrincipalId = utils.ToPointer("3fb69913-169d-4c23-8ab7-39278f71d314")
			Eventually(func(g Gomega) {
				err := k8sClient.Status().Update(ctx, uaID)
				g.Expect(err).NotTo(HaveOccurred())
			}, timeout, interval).Should(Succeed())
			// Verify that the FederatedIdentityCredential is created
			federatedCredential := &managedidentity.FederatedIdentityCredential{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, federatedCredential)
				g.Expect(err).NotTo(HaveOccurred())
			}, timeout, interval).Should(Succeed())
			// Update the FederatedIdentityCredential status
			federatedCredential.Status.Conditions = []conditions.Condition{
				{
					Type:               "Ready",
					Status:             "True",
					Reason:             "Succeeded",
					ObservedGeneration: federatedCredential.Generation,
					LastTransitionTime: metav1.Now(),
				},
			}
			federatedCredential.Status.Audiences = appIdentity.Spec.AzureAudiences
			Eventually(func(g Gomega) {
				err := k8sClient.Status().Update(ctx, federatedCredential)
				g.Expect(err).NotTo(HaveOccurred())
			}, timeout, interval).Should(Succeed())
			// Verify that the ApplicationIdentity status is updated
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, appIdentity)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(appIdentity.Status.AzureAudiences).To(Equal(federatedCredential.Status.Audiences))
			}, timeout, interval).Should(Succeed())
			// Verify that the ServiceAccount is created
			serviceAccount := &corev1.ServiceAccount{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, typeNamespacedName, serviceAccount)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(serviceAccount.Annotations).NotTo(BeEmpty())
				g.Expect(serviceAccount.Annotations["serviceaccount.azure.com/azure-identity"]).To(Equal(*appIdentity.Status.ClientID))
			}, timeout, interval).Should(Succeed())
		})
	})
})
