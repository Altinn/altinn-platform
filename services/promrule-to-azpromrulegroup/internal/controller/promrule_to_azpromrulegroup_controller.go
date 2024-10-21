package controller

import (
	"context"

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
	Scheme *runtime.Scheme
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

	// TODO(user): your logic here

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
