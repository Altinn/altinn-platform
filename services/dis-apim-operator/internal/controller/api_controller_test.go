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
	"time"

	apimv1alpha1 "github.com/Altinn/altinn-platform/services/dis-apim-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/utils"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	timeout  = time.Second * 60
	interval = time.Millisecond * 250
)

var _ = Describe("Api Controller", func() {

	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		const resourceNamespace = "default-test"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		It("should successfully reconcile the resource", func() {
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
							Content:     utils.ToPointer(`{"openapi": "3.0.0","info": {"title": "Minimal API v1","version": "1.0.0"},""paths": {}}`),
							DisplayName: "the default version",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			updatedApi := getUpdatedApi(ctx, typeNamespacedName)
			Expect(updatedApi.Spec.DisplayName).To(Equal("test-api"))
			Eventually(fakeApim.APIMVersionSets).Should(HaveLen(1))
			apimResourceName := resourceNamespace + "-" + resourceName
			Expect(fakeApim.APIMVersionSets[apimResourceName].Properties.DisplayName).To(Equal(utils.ToPointer("test-api")))
			Expect(fakeApim.APIMVersionSets[apimResourceName].Name).To(Equal(utils.ToPointer(fmt.Sprintf("%s-%s", resourceNamespace, resourceName))))
			Expect(fakeApim.APIMVersionSets[apimResourceName].Properties).NotTo(BeNil())
			Expect(fakeApim.APIMVersionSets[apimResourceName].Properties.DisplayName).To(Equal(utils.ToPointer("test-api")))
			Expect(*fakeApim.APIMVersionSets[apimResourceName].Properties.VersioningScheme).To(Equal(apim.VersioningSchemeSegment))
			Expect(fakeApim.APIMVersionSets[apimResourceName].Properties.VersionQueryName).To(BeNil())
			var apiVersionList apimv1alpha1.ApiVersionList
			Eventually(func(g Gomega) {
				err := k8sClient.List(ctx, &apiVersionList)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(apiVersionList.Items).To(HaveLen(1))
			}, timeout, interval).Should(Succeed(), "list of apiVersions should have length 1")
			Expect(apiVersionList.Items[0].Spec.DisplayName).To(Equal("the default version"))
			Expect(*apiVersionList.Items[0].Spec.Name).To(Equal("v1"))
			Expect(*apiVersionList.Items[0].Spec.Content).To(Equal(`{"openapi": "3.0.0","info": {"title": "Minimal API v1","version": "1.0.0"},""paths": {}}`))

			By("updating the openapi content in the apiversion if it has changed")
			Eventually(func(g Gomega) {
				updatedApi = getUpdatedApi(ctx, typeNamespacedName)
				updatedApi.Spec.Versions[0].Content = utils.ToPointer(`{"openapi": "3.0.0","info": {"title": "Minimal API v1","version": "1.0.0"},""paths": {"test": "test"}}`)
				g.Expect(k8sClient.Update(ctx, &updatedApi)).To(Succeed())
			}, timeout, interval).Should(Succeed(), "apiVersion content should eventually be updated")

			Eventually(func(g Gomega) {
				g.Expect(fakeApim.APIMVersionSets).To(HaveLen(1))
				g.Expect(k8sClient.List(ctx, &apiVersionList)).Should(Succeed())
				g.Expect(apiVersionList.Items).To(HaveLen(1))
				g.Expect(*apiVersionList.Items[0].Spec.Content).To(Equal(`{"openapi": "3.0.0","info": {"title": "Minimal API v1","version": "1.0.0"},""paths": {"test": "test"}}`))
			}, timeout, interval).Should(Succeed(), "apiVersion content should eventually be updated")

			By("adding a new apiVersion if it has been added to the custom resource")
			Eventually(func(g Gomega) {
				updatedApi = getUpdatedApi(ctx, typeNamespacedName)
				updatedApi.Spec.Versions = append(updatedApi.Spec.Versions, apimv1alpha1.ApiVersionSubSpec{
					Name:        utils.ToPointer("v2"),
					Content:     utils.ToPointer(`{"openapi": "3.0.0","info": {"title": "Minimal API v2","version": "1.0.0"},""paths": {}}`),
					DisplayName: "the second version",
				})

				g.Expect(k8sClient.Update(ctx, &updatedApi)).To(Succeed())
			}, timeout, interval).Should(Succeed(), "api should eventually be updated")
			Expect(fakeApim.APIMVersionSets).To(HaveLen(1))
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, &apiVersionList)).To(Succeed())
				g.Expect(apiVersionList.Items).To(HaveLen(2))
			}, timeout, interval).Should(Succeed(), "apiVersion list should eventually have length 2")

			By("deleting the api if it has been marked for deletion")
			err := k8sClient.Get(ctx, typeNamespacedName, &updatedApi)
			Expect(err).NotTo(HaveOccurred())
			Eventually(k8sClient.Delete).WithArguments(ctx, &updatedApi).Should(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.List(ctx, &apiVersionList)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(apiVersionList.Items).To(BeEmpty())
			}, timeout, interval).Should(Succeed(), "list of apiVersions should have length 0")
			Eventually(func(g Gomega) {
				g.Expect(fakeApim.APIMVersionSets).To(BeEmpty())
				err = k8sClient.Get(ctx, typeNamespacedName, &updatedApi)
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}, timeout, interval).Should(Succeed(), "api should eventually be deleted")
		})
	})
})

func getUpdatedApi(ctx context.Context, typeNamespacedName types.NamespacedName) apimv1alpha1.Api {
	resource := apimv1alpha1.Api{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, typeNamespacedName, &resource)).To(Succeed())
	}, timeout, interval).Should(Succeed())
	return resource
}
