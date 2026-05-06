package controller

import (
	"context"
	"fmt"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
)

const (
	logicalDatabaseConditionReady         = "Ready"
	logicalDatabaseConditionDatabaseReady = "DatabaseReady"
	logicalDatabaseConditionAccessReady   = "AccessReady"

	logicalDatabaseReasonValidationFailed        = "ValidationFailed"
	logicalDatabaseReasonProvisioningDeferred    = "ProvisioningNotImplemented"
	logicalDatabaseValidationReasonRequired      = "Required"
	logicalDatabaseValidationReasonNotFound      = "NotFound"
	logicalDatabaseValidationReasonNotShared     = "NotShared"
	logicalDatabaseValidationReasonUnsupported   = "Unsupported"
	logicalDatabaseValidationFieldDeletionPolicy = "spec.deletionPolicy"
)

// LogicalDatabaseReconciler reconciles a LogicalDatabase object.
type LogicalDatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=logicaldatabases,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=logicaldatabases/status,verbs=get;update;patch

func (r *LogicalDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("logicaldatabase", req.NamespacedName)

	var logicalDatabase storagev1alpha1.LogicalDatabase
	if err := r.Get(ctx, req.NamespacedName, &logicalDatabase); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !logicalDatabase.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	original := logicalDatabase.DeepCopy()
	validationErrors := r.validateLogicalDatabase(ctx, &logicalDatabase)
	logicalDatabase.Status.ObservedGeneration = logicalDatabase.Generation
	logicalDatabase.Status.ValidationErrors = validationErrors

	if len(validationErrors) > 0 {
		setLogicalDatabaseConditions(
			&logicalDatabase,
			metav1.ConditionFalse,
			logicalDatabaseReasonValidationFailed,
			"LogicalDatabase validation failed",
		)
	} else {
		setLogicalDatabaseConditions(
			&logicalDatabase,
			metav1.ConditionFalse,
			logicalDatabaseReasonProvisioningDeferred,
			"LogicalDatabase provisioning is not implemented in this release",
		)
	}

	if apiequality.Semantic.DeepEqual(original.Status, logicalDatabase.Status) {
		return ctrl.Result{}, nil
	}

	if err := r.Status().Update(ctx, &logicalDatabase); err != nil {
		logger.Error(err, "failed to update LogicalDatabase status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *LogicalDatabaseReconciler) validateLogicalDatabase(
	ctx context.Context,
	logicalDatabase *storagev1alpha1.LogicalDatabase,
) []storagev1alpha1.LogicalDatabaseValidationError {
	var validationErrors []storagev1alpha1.LogicalDatabaseValidationError

	addRequiredStringError := func(field, value string) {
		if strings.TrimSpace(value) != "" {
			return
		}
		validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
			Field:   field,
			Reason:  logicalDatabaseValidationReasonRequired,
			Message: fmt.Sprintf("%s must be set", field),
		})
	}

	addRequiredStringError("spec.serverRef.name", logicalDatabase.Spec.ServerRef.Name)
	addRequiredStringError("spec.databaseKey", logicalDatabase.Spec.DatabaseKey)
	addRequiredStringError("spec.tenant.id", logicalDatabase.Spec.Tenant.ID)
	addRequiredStringError("spec.tenant.environment", logicalDatabase.Spec.Tenant.Environment)
	addRequiredStringError("spec.access.identity.name", logicalDatabase.Spec.Access.Identity.Name)
	addRequiredStringError("spec.access.identity.principalId", logicalDatabase.Spec.Access.Identity.PrincipalId)

	if logicalDatabase.Spec.DeletionPolicy != "" &&
		logicalDatabase.Spec.DeletionPolicy != storagev1alpha1.LogicalDatabaseDeletionPolicyRetain {
		validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
			Field:   logicalDatabaseValidationFieldDeletionPolicy,
			Reason:  logicalDatabaseValidationReasonUnsupported,
			Message: "spec.deletionPolicy only supports Retain",
		})
	}

	serverName := strings.TrimSpace(logicalDatabase.Spec.ServerRef.Name)
	if serverName == "" {
		return validationErrors
	}

	var db storagev1alpha1.Database
	if err := r.Get(ctx, types.NamespacedName{
		Name:      serverName,
		Namespace: logicalDatabase.Namespace,
	}, &db); err != nil {
		if apierrors.IsNotFound(err) {
			validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
				Field:   "spec.serverRef.name",
				Reason:  logicalDatabaseValidationReasonNotFound,
				Message: fmt.Sprintf("Database %q was not found in namespace %q", serverName, logicalDatabase.Namespace),
			})
			return validationErrors
		}
		validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
			Field:   "spec.serverRef.name",
			Reason:  "GetFailed",
			Message: fmt.Sprintf("failed to get Database %q in namespace %q: %v", serverName, logicalDatabase.Namespace, err),
		})
		return validationErrors
	}

	if databaseMode(&db) != storagev1alpha1.DatabaseModeShared {
		validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
			Field:   "spec.serverRef.name",
			Reason:  logicalDatabaseValidationReasonNotShared,
			Message: fmt.Sprintf("Database %q must have spec.mode Shared", serverName),
		})
	}

	return validationErrors
}

func setLogicalDatabaseConditions(
	logicalDatabase *storagev1alpha1.LogicalDatabase,
	status metav1.ConditionStatus,
	reason,
	message string,
) {
	for _, conditionType := range []string{
		logicalDatabaseConditionReady,
		logicalDatabaseConditionDatabaseReady,
		logicalDatabaseConditionAccessReady,
	} {
		meta.SetStatusCondition(&logicalDatabase.Status.Conditions, metav1.Condition{
			Type:               conditionType,
			Status:             status,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: logicalDatabase.Generation,
		})
	}
}

func (r *LogicalDatabaseReconciler) mapDatabaseToLogicalDatabases(
	ctx context.Context,
	obj client.Object,
) []ctrl.Request {
	var list storagev1alpha1.LogicalDatabaseList
	if err := r.List(ctx, &list, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}

	requests := make([]ctrl.Request, 0)
	for i := range list.Items {
		logicalDatabase := list.Items[i]
		if logicalDatabase.Spec.ServerRef.Name != obj.GetName() {
			continue
		}
		requests = append(requests, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      logicalDatabase.Name,
				Namespace: logicalDatabase.Namespace,
			},
		})
	}
	return requests
}

func (r *LogicalDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.LogicalDatabase{}).
		Watches(&storagev1alpha1.Database{}, handler.EnqueueRequestsFromMapFunc(r.mapDatabaseToLogicalDatabases)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
