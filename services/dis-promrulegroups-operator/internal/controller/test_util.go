package controller

import (
	"context"
	"net/http"

	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"

	armalertsmanagement "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/alertsmanagement/armalertsmanagement"
	alertsmanagement_fake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/alertsmanagement/armalertsmanagement/fake"

	armresources "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	armresources_fake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/fake"
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	pyrrav1alpha1 "github.com/pyrra-dev/pyrra/kubernetes/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Fetched and adapted from: https://github.com/pyrra-dev/pyrra/blob/main/kubernetes/controllers/servicelevelobjective_test.go

func NewExamplePrometheusRule() *monitoringv1.PrometheusRule {
	trueBool := true
	return &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoring.GroupName + "/" + monitoringv1.Version,
			Kind:       monitoringv1.PrometheusRuleKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "http",
			Namespace: "monitoring",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: pyrrav1alpha1.GroupVersion.Version,
					Kind:       "ServiceLevelObjective",
					Name:       "http",
					UID:        "123",
					Controller: &trueBool,
				},
			},
			Labels: map[string]string{
				"pyrra.dev/team": "foo",
				"team":           "bar",
			},
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name:     "http-increase",
					Interval: monitoringDuration("2m30s"),
					Rules: []monitoringv1.Rule{
						{
							Record: "http_requests:increase4w",
							Expr:   intstr.FromString(`sum by (status) (increase(http_requests_total{job="app"}[4w]))`),
							Labels: map[string]string{
								"job":  "app",
								"slo":  "http",
								"team": "foo",
							},
						},
						{
							Alert: "SLOMetricAbsent",
							Expr:  intstr.FromString(`absent(http_requests_total{job="app"}) == 1`),
							For:   monitoringDuration("2m"),
							Annotations: map[string]string{
								"description": "foo",
							},
							Labels: map[string]string{
								"severity": "critical",
								"job":      "app",
								"slo":      "http",
								"team":     "foo",
							},
						},
					},
				},
				{
					Name:     "http",
					Interval: monitoringDuration("30s"),
					Rules: []monitoringv1.Rule{
						{
							Record: "http_requests:burnrate5m",
							Expr:   intstr.FromString(`sum(rate(http_requests_total{job="app",status=~"5.."}[5m])) / sum(rate(http_requests_total{job="app"}[5m]))`),
							Labels: map[string]string{"job": "app", "slo": "http", "team": "foo"},
						},
						{
							Record: "http_requests:burnrate30m",
							Expr:   intstr.FromString(`sum(rate(http_requests_total{job="app",status=~"5.."}[30m])) / sum(rate(http_requests_total{job="app"}[30m]))`),
							Labels: map[string]string{"job": "app", "slo": "http", "team": "foo"},
						},
						{
							Record: "http_requests:burnrate1h",
							Expr:   intstr.FromString(`sum(rate(http_requests_total{job="app",status=~"5.."}[1h])) / sum(rate(http_requests_total{job="app"}[1h]))`),
							Labels: map[string]string{"job": "app", "slo": "http", "team": "foo"},
						},
						{
							Record: "http_requests:burnrate2h",
							Expr:   intstr.FromString(`sum(rate(http_requests_total{job="app",status=~"5.."}[2h])) / sum(rate(http_requests_total{job="app"}[2h]))`),
							Labels: map[string]string{"job": "app", "slo": "http", "team": "foo"},
						},
						{
							Record: "http_requests:burnrate6h",
							Expr:   intstr.FromString(`sum(rate(http_requests_total{job="app",status=~"5.."}[6h])) / sum(rate(http_requests_total{job="app"}[6h]))`),
							Labels: map[string]string{"job": "app", "slo": "http", "team": "foo"},
						},
						{
							Record: "http_requests:burnrate1d",
							Expr:   intstr.FromString(`sum(rate(http_requests_total{job="app",status=~"5.."}[1d])) / sum(rate(http_requests_total{job="app"}[1d]))`),
							Labels: map[string]string{"job": "app", "slo": "http", "team": "foo"},
						},
						{
							Record: "http_requests:burnrate4d",
							Expr:   intstr.FromString(`sum(rate(http_requests_total{job="app",status=~"5.."}[4d])) / sum(rate(http_requests_total{job="app"}[4d]))`),
							Labels: map[string]string{"job": "app", "slo": "http", "team": "foo"},
						},
						{
							Alert:       "ErrorBudgetBurn",
							Expr:        intstr.FromString(`http_requests:burnrate5m{job="app",slo="http"} > (14 * (1-0.995)) and http_requests:burnrate1h{job="app",slo="http"} > (14 * (1-0.995))`),
							For:         monitoringDuration("2m0s"),
							Labels:      map[string]string{"severity": "critical", "job": "app", "long": "1h", "slo": "http", "short": "5m", "team": "foo", "exhaustion": "2d"},
							Annotations: map[string]string{"description": "foo"},
						},
						{
							Alert:       "ErrorBudgetBurn",
							Expr:        intstr.FromString(`http_requests:burnrate30m{job="app",slo="http"} > (7 * (1-0.995)) and http_requests:burnrate6h{job="app",slo="http"} > (7 * (1-0.995))`),
							For:         monitoringDuration("15m0s"),
							Labels:      map[string]string{"severity": "critical", "job": "app", "long": "6h", "slo": "http", "short": "30m", "team": "foo", "exhaustion": "4d"},
							Annotations: map[string]string{"description": "foo"},
						},
						{
							Alert:       "ErrorBudgetBurn",
							Expr:        intstr.FromString(`http_requests:burnrate2h{job="app",slo="http"} > (2 * (1-0.995)) and http_requests:burnrate1d{job="app",slo="http"} > (2 * (1-0.995))`),
							For:         monitoringDuration("1h0m0s"),
							Labels:      map[string]string{"severity": "warning", "job": "app", "long": "1d", "slo": "http", "short": "2h", "team": "foo", "exhaustion": "2w"},
							Annotations: map[string]string{"description": "foo"},
						},
						{
							Alert:       "ErrorBudgetBurn",
							Expr:        intstr.FromString(`http_requests:burnrate6h{job="app",slo="http"} > (1 * (1-0.995)) and http_requests:burnrate4d{job="app",slo="http"} > (1 * (1-0.995))`),
							For:         monitoringDuration("3h0m0s"),
							Labels:      map[string]string{"severity": "warning", "job": "app", "long": "4d", "slo": "http", "short": "6h", "team": "foo", "exhaustion": "4w"},
							Annotations: map[string]string{"description": "foo"},
						},
					},
				},
			},
		},
	}
}

func monitoringDuration(d string) *monitoringv1.Duration {
	md := monitoringv1.Duration(d)
	return &md
}

func NewFakeDeploymentsServer() armresources_fake.DeploymentsServer {
	return armresources_fake.DeploymentsServer{
		BeginCreateOrUpdate: func(ctx context.Context, resourceGroupName, deploymentName string, parameters armresources.Deployment, options *armresources.DeploymentsClientBeginCreateOrUpdateOptions) (resp azfake.PollerResponder[armresources.DeploymentsClientCreateOrUpdateResponse], errResp azfake.ErrorResponder) {
			// Set the values for the success response
			resp.SetTerminalResponse(http.StatusCreated, armresources.DeploymentsClientCreateOrUpdateResponse{}, nil)
			// Set the values for the error response; mutually exclusive. If both configured, the error response prevails
			// errResp.SetResponseError(http.StatusBadRequest, "ThisIsASimulatedError")
			return
		},
	}
}

func NewFakeNewPrometheusRuleGroupsServer() alertsmanagement_fake.PrometheusRuleGroupsServer {
	return alertsmanagement_fake.PrometheusRuleGroupsServer{
		Delete: func(ctx context.Context, resourceGroupName, ruleGroupName string, options *armalertsmanagement.PrometheusRuleGroupsClientDeleteOptions) (resp azfake.Responder[armalertsmanagement.PrometheusRuleGroupsClientDeleteResponse], errResp azfake.ErrorResponder) {
			resp.SetResponse(http.StatusOK, armalertsmanagement.PrometheusRuleGroupsClientDeleteResponse{}, nil)
			// errResp.SetResponseError(http.StatusNotFound, http.StatusText(http.StatusNotFound))
			return
		},
	}
}
