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
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	runctimeclient "sigs.k8s.io/controller-runtime/pkg/client"
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
			Expect(runctimeclient.IgnoreNotFound(err)).NotTo(HaveOccurred())

			if err == nil {
				By("Removing finalizer")
				resource.SetFinalizers([]string{})
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())
				if resource.DeletionTimestamp == nil {
					By("Cleanup the specific resource instance Backend")
					Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
				}
			}
		})
		It("should set success status when azure resource created", func() {
			fakeServer := testutils.GetFakeBackendServer(
				apim.BackendClientGetResponse{},
				utils.ToPointer(http.StatusNotFound),
				apim.BackendClientCreateOrUpdateResponse{
					BackendContract: apim.BackendContract{
						ID:   utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/fake-apim-backend"),
						Name: utils.ToPointer("fake-apim-backend"),
						Type: utils.ToPointer("Microsoft.ApiManagement/service/backends"),
					},
				},
				nil,
				apim.BackendClientDeleteResponse{},
				utils.ToPointer(http.StatusOK),
			)
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
			Expect(updatedBackend.Status.BackendID).To(Equal("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/fake-apim-backend"))
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
		It("should set failed status when azure resource failed to create and requeue", func() {
			fakeServer := testutils.GetFakeBackendServer(apim.BackendClientGetResponse{}, utils.ToPointer(http.StatusNotFound), apim.BackendClientCreateOrUpdateResponse{}, utils.ToPointer(http.StatusInternalServerError), apim.BackendClientDeleteResponse{}, utils.ToPointer(http.StatusOK))
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
			Expect(err).To(HaveOccurred())
			Expect(rsp).To(Equal(reconcile.Result{}))
			// Fetch the updated Backend resource
			updatedBackend := &apimv1alpha1.Backend{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedBackend)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedBackend.Status.ProvisioningState).To(Equal(apimv1alpha1.BackendProvisioningStateFailed))
			Expect(updatedBackend.Status.LastProvisioningError).NotTo(BeEmpty())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
		It("Should set status to succeeded if get returns backend", func() {
			fakeServer := testutils.GetFakeBackendServer(
				apim.BackendClientGetResponse{
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
						ID: utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/fake-apim-backend"),
					},
				},
				nil,
				apim.BackendClientCreateOrUpdateResponse{},
				utils.ToPointer(http.StatusInternalServerError),
				apim.BackendClientDeleteResponse{},
				utils.ToPointer(http.StatusOK),
			)
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
			Expect(updatedBackend.Status.LastProvisioningError).To(BeEmpty())
			Expect(updatedBackend.Status.BackendID).To(Equal("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/fake-apim-backend"))
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
		It("Should update azure resource if actual state does not match desired", func() {
			fakeServer := testutils.GetFakeBackendServer(
				apim.BackendClientGetResponse{
					BackendContract: apim.BackendContract{
						Properties: &apim.BackendContractProperties{
							Protocol:    utils.ToPointer(apim.BackendProtocolHTTP),
							URL:         utils.ToPointer("https://example.com"),
							Description: utils.ToPointer("Test backend for the operator"),
							TLS: &apim.BackendTLSProperties{
								ValidateCertificateChain: utils.ToPointer(true),
								ValidateCertificateName:  utils.ToPointer(true),
							},
							Title: utils.ToPointer("test-backend"),
						},
						ID: utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/fake-apim-backend"),
					},
				},
				nil,
				apim.BackendClientCreateOrUpdateResponse{
					BackendContract: apim.BackendContract{
						Properties: &apim.BackendContractProperties{
							Protocol:    utils.ToPointer(apim.BackendProtocolHTTP),
							URL:         utils.ToPointer("https://example.com"),
							Description: utils.ToPointer("Test backend for the operator"),
							TLS: &apim.BackendTLSProperties{
								ValidateCertificateChain: utils.ToPointer(true),
								ValidateCertificateName:  utils.ToPointer(true),
							},
							Title: utils.ToPointer("test-backend"),
						},
						ID: utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/fake-apim-backend-updated"),
					},
				},
				nil,
				apim.BackendClientDeleteResponse{},
				nil,
			)
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
			Expect(updatedBackend.Status.LastProvisioningError).To(BeEmpty())
			Expect(updatedBackend.Status.BackendID).To(Equal("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/fake-apim-backend-updated"))
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
		It("Should delete azure resources and remove finalizer on deletion", func() {
			beforeTest := &apimv1alpha1.Backend{}
			err := k8sClient.Get(ctx, typeNamespacedName, beforeTest)
			Expect(err).NotTo(HaveOccurred())
			beforeTest.SetFinalizers([]string{BACKEND_FINALIZER})
			Expect(k8sClient.Update(ctx, beforeTest)).To(Succeed())
			Expect(k8sClient.Delete(ctx, beforeTest)).To(Succeed())
			fakeServer := testutils.GetFakeBackendServer(
				apim.BackendClientGetResponse{
					ETag: utils.ToPointer("fake-etag"),
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
						ID: utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/fake-apim-backend"),
					},
				},
				nil,
				apim.BackendClientCreateOrUpdateResponse{},
				utils.ToPointer(http.StatusInternalServerError),
				apim.BackendClientDeleteResponse{},
				nil,
			)
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
			Expect(rsp).To(Equal(reconcile.Result{}))
			// Fetch the updated Backend resource
			updatedBackend := &apimv1alpha1.Backend{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedBackend)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
		It("Should requeue deletion if delete of backend in azure failed", func() {
			beforeTest := &apimv1alpha1.Backend{}
			err := k8sClient.Get(ctx, typeNamespacedName, beforeTest)
			Expect(err).NotTo(HaveOccurred())
			beforeTest.SetFinalizers([]string{BACKEND_FINALIZER})
			Expect(k8sClient.Update(ctx, beforeTest)).To(Succeed())
			Expect(k8sClient.Delete(ctx, beforeTest)).To(Succeed())
			fakeServer := testutils.GetFakeBackendServer(
				apim.BackendClientGetResponse{
					ETag: utils.ToPointer("fake-etag"),
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
						ID: utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/fake-apim-backend"),
					},
				},
				nil,
				apim.BackendClientCreateOrUpdateResponse{},
				utils.ToPointer(http.StatusInternalServerError),
				apim.BackendClientDeleteResponse{},
				utils.ToPointer(http.StatusInternalServerError),
			)
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
			Expect(err).To(HaveOccurred())
			Expect(rsp).To(Equal(reconcile.Result{}))
			// Fetch the updated Backend resource
			updatedBackend := &apimv1alpha1.Backend{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedBackend)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedBackend.Finalizers).To(HaveExactElements([]string{BACKEND_FINALIZER}))
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
		It("Should remove finalizer on deletion when azure backend not found", func() {
			beforeTest := &apimv1alpha1.Backend{}
			err := k8sClient.Get(ctx, typeNamespacedName, beforeTest)
			Expect(err).NotTo(HaveOccurred())
			beforeTest.SetFinalizers([]string{BACKEND_FINALIZER})
			Expect(k8sClient.Update(ctx, beforeTest)).To(Succeed())
			Expect(k8sClient.Delete(ctx, beforeTest)).To(Succeed())
			fakeServer := testutils.GetFakeBackendServer(
				apim.BackendClientGetResponse{},
				utils.ToPointer(http.StatusNotFound),
				apim.BackendClientCreateOrUpdateResponse{},
				utils.ToPointer(http.StatusInternalServerError),
				apim.BackendClientDeleteResponse{},
				utils.ToPointer(http.StatusInternalServerError),
			)
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
			Expect(rsp).To(Equal(reconcile.Result{}))
			// Fetch the updated Backend resource
			updatedBackend := &apimv1alpha1.Backend{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedBackend)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
