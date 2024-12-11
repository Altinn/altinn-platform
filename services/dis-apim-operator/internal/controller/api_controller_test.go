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
	"fmt"
	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/azure"
	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/utils"
	testutils "github.com/Altinn/altinn-platform/services/dis-apim-operator/test/utils"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	apimfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apimv1alpha1 "github.com/Altinn/altinn-platform/services/dis-apim-operator/api/v1alpha1"
)

var _ = Describe("Api Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		const resourceNamespace = "default"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		api := &apimv1alpha1.Api{}
		fakeApim := testutils.NewFakeAPIMClientStruct()
		BeforeEach(func() {
			By("ensuring all old resources are cleaned up")
			resource := &apimv1alpha1.Api{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && !errors.IsNotFound(err) {
				Expect(runtimeclient.IgnoreNotFound(err)).Should(Succeed())
				controllerutil.RemoveFinalizer(resource, API_FINALIZER)
				Eventually(k8sClient.Update(ctx, resource)).Should(Succeed())
				Eventually(k8sClient.Delete(ctx, resource)).Should(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, typeNamespacedName, resource)
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				})
			}
			versions := &apimv1alpha1.ApiVersionList{}
			Eventually(k8sClient.List(ctx, versions)).Should(Succeed())
			for _, version := range versions.Items {
				Eventually(k8sClient.Delete(ctx, &version)).Should(Succeed())
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Namespace: version.Namespace,
						Name:      version.Name,
					}, &version)
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				})
			}
			By("creating the custom resource for the Kind Api")
			err = k8sClient.Get(ctx, typeNamespacedName, api)
			if err != nil && errors.IsNotFound(err) {
				resource := &apimv1alpha1.Api{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: apimv1alpha1.ApiSpec{
						DisplayName: "test-api",
						Path:        "/test-api",
						Versions: []apimv1alpha1.ApiVersionSubSpec{
							{
								Name:        utils.ToPointer("v1"),
								Content:     utils.ToPointer(`{"openapi": "3.0.0","info": {"title": "Minimal API","version": "1.0.0"},""paths": {}}`),
								DisplayName: "the default version",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
			fakeApim.APIMVersionSets = make(map[string]apim.APIVersionSetContract)
		})
		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &apimv1alpha1.Api{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			controllerutil.RemoveFinalizer(resource, API_FINALIZER)
			Eventually(k8sClient.Update(ctx, resource)).Should(Succeed())
			Expect(err).NotTo(HaveOccurred())
			versions := &apimv1alpha1.ApiVersionList{}
			err = k8sClient.List(ctx, versions)
			Expect(err).NotTo(HaveOccurred())
			By("Cleanup the specific resource instance ApiVersion")
			for _, version := range versions.Items {
				Eventually(k8sClient.Delete(ctx, &version)).Should(Succeed())
			}
			By("Cleanup the specific resource instance Api")
			Eventually(k8sClient.Delete(ctx, resource)).Should(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			transport := apimfake.NewAPIVersionSetServerTransport(fakeApim.GetFakeApiVersionServer(false, false, false))
			factoryClientOptions := &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Transport: transport,
				},
			}
			By("creating the api during first reconciliation")
			controllerReconciler := &ApiReconciler{
				Client:    k8sClient,
				Scheme:    k8sClient.Scheme(),
				NewClient: testutils.NewFakeAPIMClient,
				ApimClientConfig: &azure.ApimClientConfig{
					AzureConfig: config.AzureConfig{
						SubscriptionId:  "fake-subscription-id",
						ResourceGroup:   "fake-resource-group",
						ApimServiceName: "fake-apim-service",
					},
					FactoryOptions: factoryClientOptions,
				},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeApim.APIMVersionSets).To(HaveLen(1))
			apimResourceName := resourceNamespace + "-" + resourceName
			Expect(fakeApim.APIMVersionSets[apimResourceName].Properties.DisplayName).To(Equal(utils.ToPointer("test-api")))
			Expect(fakeApim.APIMVersionSets[apimResourceName].Name).To(Equal(utils.ToPointer(fmt.Sprintf("%s-%s", resourceNamespace, resourceName))))
			Expect(fakeApim.APIMVersionSets[apimResourceName].Properties).NotTo(BeNil())
			Expect(fakeApim.APIMVersionSets[apimResourceName].Properties.DisplayName).To(Equal(utils.ToPointer("test-api")))
			Expect(*fakeApim.APIMVersionSets[apimResourceName].Properties.VersioningScheme).To(Equal(apim.VersioningSchemeSegment))
			Expect(fakeApim.APIMVersionSets[apimResourceName].Properties.VersionQueryName).To(BeNil())
			var apiVersions apimv1alpha1.ApiVersionList
			Eventually(k8sClient.List(ctx, &apiVersions)).Should(Succeed())
			Expect(apiVersions.Items).To(HaveLen(0))
			By("adding apiVersion during second reconciliation")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeApim.APIMVersionSets).To(HaveLen(1))
			Eventually(k8sClient.List(ctx, &apiVersions)).Should(Succeed())
			Expect(apiVersions.Items).To(HaveLen(1))
			Expect(apiVersions.Items[0].Spec.DisplayName).To(Equal("the default version"))
			Expect(*apiVersions.Items[0].Spec.Name).To(Equal("v1"))
			Expect(*apiVersions.Items[0].Spec.Content).To(Equal(`{"openapi": "3.0.0","info": {"title": "Minimal API","version": "1.0.0"},""paths": {}}`))

			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
