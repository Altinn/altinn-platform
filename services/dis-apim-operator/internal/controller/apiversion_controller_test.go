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
	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/azure"
	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/utils"
	testutils "github.com/Altinn/altinn-platform/services/dis-apim-operator/test/utils"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
	apimfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2/fake"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apimv1alpha1 "github.com/Altinn/altinn-platform/services/dis-apim-operator/api/v1alpha1"
)

var _ = Describe("ApiVersion Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		apiversion := &apimv1alpha1.ApiVersion{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ApiVersion")
			err := k8sClient.Get(ctx, typeNamespacedName, apiversion)
			if err != nil && errors.IsNotFound(err) {
				resource := &apimv1alpha1.ApiVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: apimv1alpha1.ApiVersionSpec{
						ApiVersionSetId: "test-api-version-set-id",
						Path:            "/v1",
						ApiVersionSubSpec: apimv1alpha1.ApiVersionSubSpec{
							DisplayName: "test-display-name",
							Content:     utils.ToPointer(`{"openapi": "3.0.0","info": {"title": "Minimal API","version": "1.0.0"},""paths": {}}`),
						},
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &apimv1alpha1.ApiVersion{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ApiVersion")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			fakeServer := testutils.GetFakeApiClient(
				&apim.APIClientCreateOrUpdateResponse{
					APIContract: apim.APIContract{
						Properties: nil,
						ID:         nil,
						Name:       nil,
						Type:       nil,
					},
				},
				nil,
				nil,
				utils.ToPointer(http.StatusNotFound),
				nil,
				nil,
			)
			transport := apimfake.NewAPIServerTransport(fakeServer)
			factoryClientOptions := &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Transport: transport,
				},
			}
			By("Reconciling the created resource")
			controllerReconciler := &ApiVersionReconciler{
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
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
