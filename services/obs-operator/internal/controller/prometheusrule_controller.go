package controller

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armalertsmanagement "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/alertsmanagement/armalertsmanagement"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/altinn/altinn-platform/services/obs-operator/pkg/utils"
)

// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules/status,verbs=get;update;patch

// PrometheusRuleReconciler reconciles a PrometheusRule object
type PrometheusRuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	SubscriptionID        string
	ResourceGroupName     string
	ClusterName           string
	AzureMonitorWorkspace string
	AzureRegion           string
	NewClientFactoryFunc  func(cred azcore.TokenCredential, options *arm.ClientOptions) (*armalertsmanagement.ClientFactory, error)
}

// Reconcile reconciles a PrometheusRule object and creates or updates corresponding PrometheusRuleGroup resources in Azure
func (r *PrometheusRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("PrometheusRule")

	// Fetch the PrometheusRule instance
	prometheusRule := &monitoringv1.PrometheusRule{}
	err := r.Get(ctx, req.NamespacedName, prometheusRule)
	if err != nil {
		if errors.IsNotFound(err) {
			// Resource not found. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get PrometheusRule")
		return ctrl.Result{}, err
	}

	// Authenticate with Azure
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		logger.Error(err, "Failed to authenticate with Azure")
		return ctrl.Result{}, err
	}

	// Use the injected client factory function
	clientFactory, err := r.NewClientFactoryFunc(cred, nil)
	if err != nil {
		logger.Error(err, "Failed to create Azure client factory")
		return ctrl.Result{}, err
	}

	client := clientFactory.NewPrometheusRuleGroupsClient()

	// Build a map of group names in the current PrometheusRule
	groupNames := make(map[string]struct{})
	for _, group := range prometheusRule.Spec.Groups {
		groupNames[group.Name] = struct{}{}

		// Generate a unique name for the PrometheusRuleGroup
		prgName := fmt.Sprintf("%s-%s", prometheusRule.Name, group.Name)

		// Convert the rules to Azure format
		azureRules, err := mapPrometheusRules(group.Rules)
		if err != nil {
			logger.Error(err, "Failed to map Prometheus rules")
			return ctrl.Result{}, err
		}

		// Handle Interval
		var interval string
		if group.Interval != nil {
			// Dereference the Duration (*monitoringv1.Duration) and convert it to ISO 8601
			durationStr := string(ptr.Deref(group.Interval, ""))
			isoDuration, err := utils.PromDurationToISO8601(durationStr)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to convert duration: %w", err)
			}
			interval = isoDuration // Assign ISO 8601 duration to a pointer
		} else {
			// TOD: Default interval ?
			interval = "PT1M"
		}

		// Create or update the PrometheusRuleGroup in Azure
		parameters := armalertsmanagement.PrometheusRuleGroupResource{
			Location: &r.AzureRegion,
			Properties: &armalertsmanagement.PrometheusRuleGroupProperties{
				ClusterName: &r.ClusterName,
				Enabled:     ptr.To(true),
				Description: &prgName,
				Interval:    &interval,
				Scopes:      []*string{&r.AzureMonitorWorkspace},
				Rules:       azureRules,
			},
		}

		// Call Azure SDK to create or update the resource
		_, err = client.CreateOrUpdate(ctx, r.ResourceGroupName, prgName, parameters, nil)
		if err != nil {
			logger.Error(err, "Failed to create or update PrometheusRuleGroup in Azure", "ruleGroup", prgName)
			return ctrl.Result{}, err
		}
		logger.Info("Successfully created or updated PrometheusRuleGroup in Azure", "ruleGroup", prgName)
	}

	// TODO: Where not handling deletion of prom rule groups yet - TBD

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrometheusRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1.PrometheusRule{}).
		Complete(r)
}

// mapPrometheusRules converts Prometheus rules to Azure PrometheusRule format
func mapPrometheusRules(rules []monitoringv1.Rule) ([]*armalertsmanagement.PrometheusRule, error) {
	azureRules := make([]*armalertsmanagement.PrometheusRule, len(rules))
	for i, r := range rules {
		azureRule := &armalertsmanagement.PrometheusRule{
			Expression:  ptr.To(r.Expr.String()),
			Annotations: copyStringMapToPtrMap(r.Annotations),
			Labels:      copyStringMapToPtrMap(r.Labels),
			Enabled:     ptr.To(true),
		}

		if r.Alert != "" {
			azureRule.Alert = ptr.To(r.Alert)
			if r.For != nil {
				isoDuration, err := utils.PromDurationToISO8601(string(ptr.Deref(r.For, "")))
				if err != nil {
					return nil, fmt.Errorf("invalid duration in rule: %w", err)
				}
				azureRule.For = ptr.To(isoDuration)
			}
			// TODO: set severity ?
			if severityStr, ok := r.Labels["severity"]; ok {
				severity, err := parseSeverity(severityStr)
				if err == nil {
					azureRule.Severity = ptr.To(severity)
				}
			}
			// TODO: handle actions for rule here
		}

		if r.Record != "" {
			azureRule.Record = ptr.To(r.Record)
		}

		azureRules[i] = azureRule
	}
	return azureRules, nil
}

// copyStringMapToPtrMap converts a map[string]string to map[string]*string
func copyStringMapToPtrMap(input map[string]string) map[string]*string {
	output := make(map[string]*string, len(input))
	for key, value := range input {
		output[key] = ptr.To(value)
	}
	return output
}

// parseSeverity converts a severity string to int32
func parseSeverity(severityStr string) (int32, error) {
	// TODO: we need to define severity leves and their corresponding values

	return 3, nil
}
