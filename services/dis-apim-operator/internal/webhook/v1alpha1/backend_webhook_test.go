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

package v1alpha1

import (
	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apimv1alpha1 "github.com/Altinn/altinn-platform/services/dis-apim-operator/api/v1alpha1"
	// TODO (user): Add any additional imports if needed
)

var _ = Describe("Backend Webhook", func() {
	var (
		obj       *apimv1alpha1.Backend
		oldObj    *apimv1alpha1.Backend
		defaulter BackendCustomDefaulter
	)

	BeforeEach(func() {
		obj = &apimv1alpha1.Backend{}
		oldObj = &apimv1alpha1.Backend{}
		defaulter = BackendCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {
	})

	Context("When creating Backend under Defaulting Webhook", func() {
		It("Should add a random string to AzureResourcePrefix if not provided", func() {
			By("simulating a scenario where AzureResourcePrefix is not provided")
			obj.Spec.AzureResourcePrefix = nil
			By("calling the Default method to apply defaults")
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			By("checking that a random string is added to AzureResourcePrefix")
			Expect(obj.Spec.AzureResourcePrefix).NotTo(BeNil())
			Expect(*obj.Spec.AzureResourcePrefix).To(HaveLen(8))
		})
		It("Should not add a random string to AzureResourcePrefix if provided", func() {
			By("simulating a scenario where AzureResourcePrefix is provided")
			obj.Spec.AzureResourcePrefix = utils.ToPointer("test")
			By("calling the Default method to apply defaults")
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			By("checking that AzureResourcePrefix is not changed")
			Expect(*obj.Spec.AzureResourcePrefix).To(Equal("test"))
		})
	})

})
