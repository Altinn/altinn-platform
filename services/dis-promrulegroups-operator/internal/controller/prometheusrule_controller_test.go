/*
Copyright 2024.

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
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/alertsmanagement/armalertsmanagement"

	armalertsmanagement_fake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/alertsmanagement/armalertsmanagement/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	armresources_fake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/fake"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("PrometheusRule Controller", func() {

	const (
		PrometheusRuleName      = "http"
		PrometheusRuleNamespace = "monitoring"

		timeout                   = time.Second * 20
		duration                  = time.Second * 10
		interval                  = time.Millisecond * 250
		eventuallyTimeout         = 2 * time.Minute
		eventuallyPollingInterval = time.Second
	)

	Context("When reconciling a resource", func() {
		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      PrometheusRuleNamespace,
				Namespace: PrometheusRuleNamespace,
			},
		}
		typeNamespacedName := types.NamespacedName{
			Name:      PrometheusRuleName,
			Namespace: PrometheusRuleNamespace,
		}

		SetDefaultEventuallyTimeout(eventuallyTimeout)
		SetDefaultEventuallyPollingInterval(eventuallyPollingInterval)

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).NotTo(HaveOccurred())

			By("Creating the custom resource for the Kind PrometheusRule")
			promRule := NewExamplePrometheusRule()

			err = k8sClient.Create(ctx, promRule)
			Expect(err).NotTo(HaveOccurred())

			promRuleFromCluster := &monitoringv1.PrometheusRule{}
			err = k8sClient.Get(ctx, typeNamespacedName, promRuleFromCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(promRuleFromCluster.Spec.Groups)).To(Equal(2))
		})

		AfterEach(func() {
			By("Removing the custom resource for the Kind PrometheusRule")
			found := &monitoringv1.PrometheusRule{}
			err := k8sClient.Get(ctx, typeNamespacedName, found)
			if !errors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Delete(context.TODO(), found)).To(Succeed())
				}).Should(Succeed())
			}
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)
		})

		It("should successfully reconcile a custom resource for PrometheusRule", func() {
			// Fake servers

			fakeDeploymentsServer := NewFakeDeploymentsServer()
			fakePrometheusRuleGroupsServer := NewFakeNewPrometheusRuleGroupsServer()
			// Clients
			deploymentsClient, err := armresources.NewDeploymentsClient("subscriptionID", &azfake.TokenCredential{}, &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Transport: armresources_fake.NewDeploymentsServerTransport(&fakeDeploymentsServer),
				},
			})
			if err != nil {
				log.Fatal(err)
			}

			prometheusRuleGroupsClient, err := armalertsmanagement.NewPrometheusRuleGroupsClient("subscriptionID", &azfake.TokenCredential{}, &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Transport: armalertsmanagement_fake.NewPrometheusRuleGroupsServerTransport(&fakePrometheusRuleGroupsServer),
				},
			})
			if err != nil {
				log.Fatal(err)
			}

			controllerReconciler := &PrometheusRuleReconciler{
				Client:                     k8sClient,
				Scheme:                     k8sClient.Scheme(),
				DeploymentClient:           deploymentsClient,
				PrometheusRuleGroupsClient: prometheusRuleGroupsClient,
				AzResourceGroupName:        "ResourceGroupName",
				AzResourceGroupLocation:    "ResourceGroupLocation",
				AzAzureMonitorWorkspace:    "AzureMonitorWorkspace",
				AzClusterName:              "ClusterName",
				NodePath:                   "node",
				AzPromRulesConverterPath:   "../../bin/az-tool/node_modules/az-prom-rules-converter/dist/cli.js",
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			promRuleFromCluster := &monitoringv1.PrometheusRule{}

			By("checking that our finalizer is added to the prometheusrule")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, promRuleFromCluster)).To(Succeed())
				g.Expect(slices.Contains(promRuleFromCluster.GetFinalizers(), finalizerName)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("checking that our annotations are added to the prometheusrule")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, promRuleFromCluster)).To(Succeed())
				g.Expect(len(promRuleFromCluster.GetAnnotations())).To(Equal(4))
				g.Expect(azArmDeploymentLastSuccessfulTimestampAnnotation).To(BeKeyOf(promRuleFromCluster.GetAnnotations()))
				g.Expect(azArmDeploymentNameAnnotation).To(BeKeyOf(promRuleFromCluster.GetAnnotations()))
				g.Expect(azArmTemplateHashAnnotation).To(BeKeyOf(promRuleFromCluster.GetAnnotations()))
				g.Expect(azPrometheusRuleGroupResourceNamesAnnotation).To(BeKeyOf(promRuleFromCluster.GetAnnotations()))
			}, timeout, interval).Should(Succeed())

			By("checking that changes to the prometheusrule are detected")
			err = k8sClient.Get(ctx, typeNamespacedName, promRuleFromCluster)
			Expect(err).NotTo(HaveOccurred())
			templateHash := strings.Clone(promRuleFromCluster.GetAnnotations()[azArmTemplateHashAnnotation])

			*promRuleFromCluster.Spec.Groups[0].Interval = monitoringv1.Duration("5m")
			err = k8sClient.Update(ctx, promRuleFromCluster)
			Expect(err).NotTo(HaveOccurred())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, promRuleFromCluster)).To(Succeed())
				g.Expect(templateHash).To(Not(Equal(promRuleFromCluster.GetAnnotations()[azArmTemplateHashAnnotation])))
			}, timeout, interval).Should(Succeed())

			By("checking that resources marked to be deleted, are deleted")
			err = k8sClient.Delete(ctx, promRuleFromCluster)
			Expect(err).NotTo(HaveOccurred())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				g.Expect(errors.IsNotFound(k8sClient.Get(ctx, typeNamespacedName, promRuleFromCluster))).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Checking that the ARM template is correctly generated")
			tmplt, err := controllerReconciler.generateArmTemplateFromPromRule(context.TODO(), *NewExamplePrometheusRule())
			Expect(err).NotTo(HaveOccurred())
			reftmplt, err := os.ReadFile("../../test/example_arm_template.json")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(reftmplt)).To(Equal(tmplt))
		})
	})
})

var _ = DescribeTable("Detecting PrometheusRuleGroups to delete", func(old, new, toDelete []string) {
	Expect(removedGroups(old, new)).To(Equal(toDelete))
},
	Entry(nil, []string{"a", "b", "c"}, []string{"a", "b", "c"}, []string{}),
	Entry(nil, []string{"a", "b", "c"}, []string{"a", "b", "d"}, []string{"c"}),
	Entry(nil, []string{"a", "b", "c"}, []string{"a", "b"}, []string{"c"}),
	Entry(nil, []string{"a", "b", "c"}, []string{"a", "b", "d", "e"}, []string{"c"}),
	Entry(nil, []string{"a", "b", "c"}, []string{"b", "a"}, []string{"c"}),
	Entry(nil, []string{"a", "b", "c"}, []string{"a", "b", "c", "d"}, []string{}),
	Entry(nil, []string{"a", "b", "c"}, []string{}, []string{"a", "b", "c"}),
)
