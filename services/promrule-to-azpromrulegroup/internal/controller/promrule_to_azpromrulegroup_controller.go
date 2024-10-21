package controller

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/alertsmanagement/armalertsmanagement"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PromRuleToAzPromRuleGroupReconciler is a Reconciler that watches prometheus-operator/PrometheusRule CRs and
// makes sure the equivalent Microsoft.AlertsManagement/prometheusRuleGroups are kept in sync.
type PromRuleToAzPromRuleGroupReconciler struct {
	client.Client
	Scheme                     *runtime.Scheme
	DeploymentClient           *armresources.DeploymentsClient
	PrometheusRuleGroupsClient *armalertsmanagement.PrometheusRuleGroupsClient
}

/*
	TODO: review the permissions needed.
*/

// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrule,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrule/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *PromRuleToAzPromRuleGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	/*
		TODO: review this

		First naive implementation
		- Call the API to get the specific CR.
		- Extract the Spec from the PrometheusRule
		- json.Marshal the struct
		- Call the az-prom-rules-converter to get an ARM template with the equivalent PrometheusRuleGroup
		- Deploy the ARM template

		Second stage
		- How to handle the different kinds of events; create, update, delete?
			- Add the deployed ARM template as an annotation in the CR? We can re-generate the ARM template and compare with the previous one.
			- I lack experience with ARM templates but it looks like they aren't great for deletions. We might need to call a
			  armalertsmanagement.PrometheusRuleGroupsClient to perform the deletions.
			- ARM template deployments take some time to complete; Should we wait? Or requeue?

	*/

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PromRuleToAzPromRuleGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Watches(
			&monitoringv1.PrometheusRule{},
			&handler.EnqueueRequestForObject{},
		).
		Named("promrule-to-azpromrulegroup").
		Complete(r)
}
