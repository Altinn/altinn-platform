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
	"net/http"
	"time"

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
		fakeApim := testutils.NewFakeAPIMClientStruct()
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
			fakeApim.Backends = make(map[string]apim.BackendContract)
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

		_ = testutils.GetFakeBackendServer(
			&apim.BackendClientGetResponse{},
			utils.ToPointer(http.StatusNotFound),
			&apim.BackendClientCreateOrUpdateResponse{
				BackendContract: apim.BackendContract{
					ID:   utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/fake-apim-backend"),
					Name: utils.ToPointer("fake-apim-backend"),
					Type: utils.ToPointer("Microsoft.ApiManagement/service/backends"),
				},
			},
			nil,
			&apim.BackendClientDeleteResponse{},
			utils.ToPointer(http.StatusOK),
		)
		It("should set success status when azure resource created", func() {
			transport := apimfake.NewBackendServerTransport(fakeApim.GetFakeBackendServer(false, false, false))
			factoryClientOptions := &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Transport: transport,
				},
			}
			By("Creating the apim Backend if it does not exist")
			controllerReconciler := &BackendReconciler{
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
			Expect(updatedBackend.Status.BackendID).To(Equal("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/" + updatedBackend.GetAzureResourceName()))
			Expect(updatedBackend.Status.ProvisioningState).To(Equal(apimv1alpha1.BackendProvisioningStateSucceeded))
			Expect(fakeApim.Backends).To(HaveLen(1))
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			By("Updating the apim Backend if it does not match the desired state")
			updatedBackend.Spec.Url = "https://updated.example.com"
			err = k8sClient.Update(ctx, updatedBackend)
			Expect(err).NotTo(HaveOccurred())
			rsp, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(rsp).To(Equal(reconcile.Result{RequeueAfter: 1 * time.Minute}))
			// Fetch the updated Backend resource
			updatedBackend = &apimv1alpha1.Backend{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedBackend)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedBackend.Status.ProvisioningState).To(Equal(apimv1alpha1.BackendProvisioningStateSucceeded))
			Expect(updatedBackend.Status.BackendID).To(Equal("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/" + updatedBackend.GetAzureResourceName()))
			Expect(fakeApim.Backends).To(HaveLen(1))
			Expect(fakeApim.Backends).To(HaveKey(updatedBackend.GetAzureResourceName()))
			Expect(*fakeApim.Backends[updatedBackend.GetAzureResourceName()].Properties.URL).To(Equal(updatedBackend.Spec.Url))
			By("Deleting the apim Backend and removing the finalizer when the resource is deleted")
			err = k8sClient.Delete(ctx, updatedBackend)
			Expect(err).NotTo(HaveOccurred())
			rsp, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(rsp).To(Equal(reconcile.Result{}))
			// Fetch the updated Backend resource
			updatedBackend = &apimv1alpha1.Backend{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedBackend)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
			Expect(fakeApim.Backends).To(HaveLen(0))
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
		It("should set failed status when azure resource failed to create and requeue", func() {
			transport := apimfake.NewBackendServerTransport(fakeApim.GetFakeBackendServer(true, false, false))
			factoryClientOptions := &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Transport: transport,
				},
			}
			By("Reconciling the created resource")
			controllerReconciler := &BackendReconciler{
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
			backendID := "default-" + resourceName
			fakeApim.Backends[backendID] = apim.BackendContract{
				ID:   utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/" + backendID),
				Name: utils.ToPointer(backendID),
				Type: utils.ToPointer("Microsoft.ApiManagement/service/backends"),
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
			}
			transport := apimfake.NewBackendServerTransport(fakeApim.GetFakeBackendServer(true, false, false))
			factoryClientOptions := &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Transport: transport,
				},
			}
			By("Reconciling the created resource")
			controllerReconciler := &BackendReconciler{
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
			Expect(updatedBackend.Status.BackendID).To(Equal("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/" + backendID))
			Expect(fakeApim.Backends).To(HaveLen(1))
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
			backendID := "default-" + resourceName
			fakeApim.Backends[backendID] = apim.BackendContract{
				ID:   utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/" + backendID),
				Name: utils.ToPointer(backendID),
				Type: utils.ToPointer("Microsoft.ApiManagement/service/backends"),
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
			}
			transport := apimfake.NewBackendServerTransport(fakeApim.GetFakeBackendServer(false, false, true))
			factoryClientOptions := &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Transport: transport,
				},
			}
			By("Reconciling the created resource")
			controllerReconciler := &BackendReconciler{
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
		})
		It("Should remove finalizer on deletion when azure backend not found", func() {
			beforeTest := &apimv1alpha1.Backend{}
			err := k8sClient.Get(ctx, typeNamespacedName, beforeTest)
			Expect(err).NotTo(HaveOccurred())
			beforeTest.SetFinalizers([]string{BACKEND_FINALIZER})
			Expect(k8sClient.Update(ctx, beforeTest)).To(Succeed())
			Expect(k8sClient.Delete(ctx, beforeTest)).To(Succeed())
			transport := apimfake.NewBackendServerTransport(fakeApim.GetFakeBackendServer(true, false, true))
			factoryClientOptions := &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Transport: transport,
				},
			}
			By("Reconciling the created resource")
			controllerReconciler := &BackendReconciler{
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
		})
	})
})
