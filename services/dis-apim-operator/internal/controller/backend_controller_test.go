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
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
	apimfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2/fake"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apimv1alpha1 "github.com/Altinn/altinn-platform/services/dis-apim-operator/api/v1alpha1"
)

var _ = Describe("Backend Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		backend := &apimv1alpha1.Backend{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Backend")
			err := k8sClient.Get(ctx, typeNamespacedName, backend)
			if err != nil && errors.IsNotFound(err) {
				resource := &apimv1alpha1.Backend{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: apimv1alpha1.BackendSpec{
						Title:       "test-backend",
						Description: utils.ToPointer("Test backend for the operator"),
						Url:         "https://test.example.com",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &apimv1alpha1.Backend{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Backend")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			fakeServer := &apimfake.BackendServer{
				CreateOrUpdate: func(
					ctx context.Context,
					resourceGroupName string,
					serviceName string,
					backendID string,
					parameters apim.BackendContract,
					options *apim.BackendClientCreateOrUpdateOptions,
				) (resp azfake.Responder[apim.BackendClientCreateOrUpdateResponse], errResp azfake.ErrorResponder) {

					response := apim.BackendClientCreateOrUpdateResponse{
						BackendContract: apim.BackendContract{
							Properties: parameters.Properties,
							ID:         utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/fake-apim-backend"),
							Name:       utils.ToPointer("fake-apim-backend"),
							Type:       utils.ToPointer("Microsoft.ApiManagement/service/backends"),
						},
					}

					responder := azfake.Responder[apim.BackendClientCreateOrUpdateResponse]{}

					responder.SetResponse(http.StatusOK, response, nil)

					return responder, azfake.ErrorResponder{}
				},
				Delete: func(
					ctx context.Context,
					resourceGroupName string,
					serviceName string,
					backendID string,
					ifMatch string,
					options *apim.BackendClientDeleteOptions,
				) (resp azfake.Responder[apim.BackendClientDeleteResponse], errResp azfake.ErrorResponder) {
					response := apim.BackendClientDeleteResponse{}
					responder := azfake.Responder[apim.BackendClientDeleteResponse]{}

					responder.SetResponse(http.StatusOK, response, nil)

					return responder, azfake.ErrorResponder{}
				},
				Get: func(
					ctx context.Context,
					resourceGroupName string,
					serviceName string,
					backendID string,
					options *apim.BackendClientGetOptions,
				) (resp azfake.Responder[apim.BackendClientGetResponse], errResp azfake.ErrorResponder) {
					response := apim.BackendClientGetResponse{
						BackendContract: apim.BackendContract{
							Properties: &apim.BackendContractProperties{
								Protocol:    utils.ToPointer(apim.BackendProtocolHTTP),
								URL:         utils.ToPointer("https://test.example.com"),
								Description: utils.ToPointer("Test backend for the operator"),
								TLS: &apim.BackendTLSProperties{
									ValidateCertificateChain: utils.ToPointer(true),
									ValidateCertificateName:  utils.ToPointer(true),
								},
								Title: utils.ToPointer("test-backend"),
							},
						},
						ETag: utils.ToPointer("33a64df551425fcc55e4d42a148795d9f25f89d5"),
					}
					responder := azfake.Responder[apim.BackendClientGetResponse]{}

					responder.SetResponse(http.StatusOK, response, nil)
					errResponder := azfake.ErrorResponder{}
					errResponder.SetResponseError(http.StatusNotFound, "Not Found")
					return responder, errResponder
				},
				GetEntityTag:          nil,
				NewListByServicePager: nil,
				Reconnect:             nil,
				Update:                nil,
			}
			transport := apimfake.NewBackendServerTransport(fakeServer)
			factoryClientOptions := &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Transport: transport,
				},
			}
			By("Reconciling the created resource")
			controllerReconciler := &BackendReconciler{
				Client:    k8sClient,
				Scheme:    k8sClient.Scheme(),
				NewClient: azure.NewAPIMClient,
				ApimClientConfig: &azure.ApimClientConfig{
					AzureConfig: config.AzureConfig{
						SubscriptionId:  "fake-subscription-id",
						ResourceGroup:   "fake-resource-group",
						ApimServiceName: "fake-apim-service",
					},
					FactoryOptions: factoryClientOptions,
				},
			}

			rsp, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(rsp).To(Equal(reconcile.Result{RequeueAfter: 1 * time.Minute}))
			// Fetch the updated Backend resource
			updatedBackend := &apimv1alpha1.Backend{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedBackend)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedBackend.Status.ProvisioningState).To(Equal(apimv1alpha1.BackendProvisioningStateSucceeded))
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
