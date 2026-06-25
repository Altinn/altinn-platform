package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	armalertsmanagement "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/alertsmanagement/armalertsmanagement"
	armalertsmanagementfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/alertsmanagement/armalertsmanagement/fake"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/altinn/altinn-platform/services/obs-operator/pkg/utils"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("PrometheusRule Controller", func() {
	const (
		PrometheusRuleName      = "test-prometheusrule"
		PrometheusRuleNamespace = "default"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a PrometheusRule with multiple groups", func() {
		It("Should create corresponding PrometheusRuleGroups in Azure for each group", func() {
			By("Setting up the test environment")
			ctx := context.Background()

			scheme := runtime.NewScheme()
			Expect(monitoringv1.AddToScheme(scheme)).Should(Succeed())
			// TODO: more schemes? maybe not now

			k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			// Create a PrometheusRule resource
			prometheusRule := &monitoringv1.PrometheusRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PrometheusRuleName,
					Namespace: PrometheusRuleNamespace,
				},
				Spec: monitoringv1.PrometheusRuleSpec{
					Groups: []monitoringv1.RuleGroup{
						{
							Name:     "group1",
							Interval: utils.NewDuration("30s"),
							Rules: []monitoringv1.Rule{
								{
									Alert: "HighRequestLatency",
									Expr:  intstr.FromString(`request_latency_seconds_bucket{le="0.5"}`),
									Labels: map[string]string{
										"severity": "1",
									},
									Annotations: map[string]string{
										"summary": "High request latency",
									},
								},
							},
						},
						{
							Name:     "group2",
							Interval: utils.NewDuration("1m"),
							Rules: []monitoringv1.Rule{
								{
									Record: "job:http_inprogress_requests:sum",
									Expr:   intstr.FromString(`sum(http_inprogress_requests) by (job)`),
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, prometheusRule)).Should(Succeed())

			// Create the fake Azure PrometheusRuleGroupsServer
			fakeServer := &armalertsmanagementfake.PrometheusRuleGroupsServer{
				CreateOrUpdate: func(
					ctx context.Context,
					resourceGroupName string,
					ruleGroupName string,
					parameters armalertsmanagement.PrometheusRuleGroupResource,
					options *armalertsmanagement.PrometheusRuleGroupsClientCreateOrUpdateOptions,
				) (
					resp azfake.Responder[armalertsmanagement.PrometheusRuleGroupsClientCreateOrUpdateResponse],
					errResp azfake.ErrorResponder,
				) {
					// Verify the parameters
					Expect(resourceGroupName).To(Equal("my-resource-group"))
					Expect(ruleGroupName).To(Or(
						Equal(fmt.Sprintf("%s-%s", PrometheusRuleName, "group1")),
						Equal(fmt.Sprintf("%s-%s", PrometheusRuleName, "group2")),
					))

					// Fake a successful response
					response := armalertsmanagement.PrometheusRuleGroupsClientCreateOrUpdateResponse{
						PrometheusRuleGroupResource: parameters,
					}

					// create the responde etc
					responder := azfake.Responder[armalertsmanagement.PrometheusRuleGroupsClientCreateOrUpdateResponse]{}

					header := http.Header{}
					// TODO: just testing, remove later, not needed
					header.Set("custom-header1", "value1")

					responder.SetResponse(http.StatusOK, response, &azfake.SetResponseOptions{
						Header: header,
					})

					return responder, azfake.ErrorResponder{}
				},
				// TODO: more methods for the fake server?
			}

			// Create a transport using the fake server
			transport := armalertsmanagementfake.NewPrometheusRuleGroupsServerTransport(fakeServer)

			// Create client options with the fake transport
			clientOptions := &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Transport: transport,
				},
			}

			// Create the fake client factory function
			fakeClientFactoryFunc := func(cred azcore.TokenCredential, options *arm.ClientOptions) (*armalertsmanagement.ClientFactory, error) {
				return armalertsmanagement.NewClientFactory("fake-subscription-id", cred, clientOptions)
			}

			reconciler := &PrometheusRuleReconciler{
				Client: k8sClient,
				Scheme: scheme,

				SubscriptionID:        "fake-subscription-id",
				ResourceGroupName:     "my-resource-group",
				ClusterName:           "my-cluster",
				AzureMonitorWorkspace: "my-workspace",
				AzureRegion:           "westeurope",
				NewClientFactoryFunc:  fakeClientFactoryFunc,
			}

			By("Calling the Reconcile function")
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      prometheusRule.Name,
					Namespace: prometheusRule.Namespace,
				},
			}
			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())

		})
	})
})
