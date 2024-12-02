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
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

		BeforeEach(func() {
			By("creating the custom resource for the Kind Api")
			err := k8sClient.Get(ctx, typeNamespacedName, api)
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
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &apimv1alpha1.Api{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			controllerutil.RemoveFinalizer(resource, API_FINALIZER)
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			Expect(err).NotTo(HaveOccurred())
			versions := &apimv1alpha1.ApiVersionList{}
			err = k8sClient.List(ctx, versions)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Api")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			By("Cleanup the specific resource instance ApiVersion")
			for _, version := range versions.Items {
				k8sClient.Delete(ctx, &version)
			}
		})
		It("should successfully reconcile the resource and create a azure apim api versionset during first reconcile", func() {
			var createdVersionsets []apim.APIVersionSetContract
			getCounter := 0
			fakeServer := testutils.GetFakeApiVersionSetClient(
				&apim.APIVersionSetClientCreateOrUpdateResponse{
					APIVersionSetContract: apim.APIVersionSetContract{
						Properties: &apim.APIVersionSetContractProperties{},
						ID:         utils.ToPointer("fake-api-id"),
						Name:       utils.ToPointer(resourceName),
						Type:       nil,
					},
				},
				nil,
				nil,
				utils.ToPointer(http.StatusNotFound),
				nil,
				nil,
				func(input apim.APIVersionSetContract, errorResponse bool) {
					createdVersionsets = append(createdVersionsets, input)
				},
				func(input string, errorResponse bool) {
					getCounter++
				},
			)
			transport := apimfake.NewAPIVersionSetServerTransport(fakeServer)
			factoryClientOptions := &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Transport: transport,
				},
			}
			By("Reconciling the created resource")
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
			Expect(getCounter).To(Equal(1), "Expected to get the versionset once")
			Expect(createdVersionsets).To(HaveLen(1))
			Expect(createdVersionsets[0].Name).To(Equal(utils.ToPointer(fmt.Sprintf("%s-%s", resourceNamespace, resourceName))))
			Expect(createdVersionsets[0].ID).To(BeNil())
			Expect(createdVersionsets[0].Properties).NotTo(BeNil())
			Expect(createdVersionsets[0].Properties.DisplayName).To(Equal(utils.ToPointer("test-api")))
			Expect(*createdVersionsets[0].Properties.VersioningScheme).To(Equal(apim.VersioningSchemeSegment))
			Expect(createdVersionsets[0].Properties.VersionQueryName).To(BeNil())
			var apiVersions apimv1alpha1.ApiVersionList
			k8sClient.List(ctx, &apiVersions)
			Expect(apiVersions.Items).To(HaveLen(0))
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
