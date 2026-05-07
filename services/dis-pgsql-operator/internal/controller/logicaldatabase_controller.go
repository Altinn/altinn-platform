package controller

import (
	"context"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/config"
	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
)

const (
	logicalDatabaseValidationReasonRequired      = "Required"
	logicalDatabaseValidationReasonNotFound      = "NotFound"
	logicalDatabaseValidationReasonNotShared     = "NotShared"
	logicalDatabaseValidationReasonUnsupported   = "Unsupported"
	logicalDatabaseValidationReasonImmutable     = "Immutable"
	logicalDatabaseValidationFieldServerRefName  = "spec.serverRef.name"
	logicalDatabaseValidationFieldDeletionPolicy = "spec.deletionPolicy"
	logicalDatabaseValidationFieldDatabaseName   = "status.databaseName"
)

// LogicalDatabaseReconciler reconciles a LogicalDatabase object.
type LogicalDatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Config config.OperatorConfig
}

// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=logicaldatabases,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=logicaldatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=databases,verbs=get;list;watch
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleserversdatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleserversdatabases/status,verbs=get
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

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
	validationErrors, err := r.validateLogicalDatabase(ctx, &logicalDatabase)
	if err != nil {
		logger.Error(err, "failed to validate LogicalDatabase")
		return ctrl.Result{}, err
	}
	logicalDatabase.Status.ObservedGeneration = logicalDatabase.Generation
	logicalDatabase.Status.ValidationErrors = validationErrors

	result := ctrl.Result{}
	if len(validationErrors) > 0 {
		setLogicalDatabaseConditions(
			&logicalDatabase,
			metav1.ConditionFalse,
			logicalDatabaseReasonValidationFailed,
			"LogicalDatabase validation failed",
		)
	} else {
		databaseName := dbUtil.DeriveLogicalDatabaseName(
			logicalDatabase.Spec.Tenant.ID,
			logicalDatabase.Spec.Tenant.Environment,
			logicalDatabase.Spec.DatabaseKey,
		)
		logicalDatabase.Status.DatabaseName = databaseName

		if err := r.ensureFlexibleServersDatabase(ctx, &logicalDatabase); err != nil {
			logger.Error(err, "failed to ensure FlexibleServersDatabase for LogicalDatabase")
			return ctrl.Result{}, err
		}

		ready, host, err := r.logicalDatabaseReady(ctx, logger, &logicalDatabase)
		if err != nil {
			logger.Error(err, "failed to check LogicalDatabase readiness")
			return ctrl.Result{}, err
		}
		if ready {
			logicalDatabase.Status.Host = host
			logicalDatabase.Status.Port = logicalDatabasePort
			setLogicalDatabaseCondition(
				&logicalDatabase,
				logicalDatabaseConditionDatabaseReady,
				metav1.ConditionTrue,
				logicalDatabaseReasonDatabaseReady,
				"Logical database is ready",
			)

			accessReady, accessReason, accessMessage, err := r.ensureLogicalDatabaseAccess(ctx, logger, &logicalDatabase)
			if err != nil {
				logger.Error(err, "failed to ensure LogicalDatabase access")
				return ctrl.Result{}, err
			}
			if accessReady {
				setLogicalDatabaseCondition(
					&logicalDatabase,
					logicalDatabaseConditionAccessReady,
					metav1.ConditionTrue,
					accessReason,
					accessMessage,
				)
				setLogicalDatabaseCondition(
					&logicalDatabase,
					logicalDatabaseConditionReady,
					metav1.ConditionTrue,
					logicalDatabaseReasonReady,
					"Logical database and access are ready",
				)
			} else {
				setLogicalDatabaseCondition(
					&logicalDatabase,
					logicalDatabaseConditionAccessReady,
					metav1.ConditionFalse,
					accessReason,
					accessMessage,
				)
				setLogicalDatabaseCondition(
					&logicalDatabase,
					logicalDatabaseConditionReady,
					metav1.ConditionFalse,
					logicalDatabaseReasonProvisioning,
					"Logical database access is provisioning",
				)
				result = ctrl.Result{RequeueAfter: logicalDatabaseRequeueDelay}
			}
		} else {
			setLogicalDatabaseCondition(
				&logicalDatabase,
				logicalDatabaseConditionDatabaseReady,
				metav1.ConditionFalse,
				logicalDatabaseReasonProvisioning,
				"Logical database is provisioning",
			)
			setLogicalDatabaseCondition(
				&logicalDatabase,
				logicalDatabaseConditionAccessReady,
				metav1.ConditionFalse,
				logicalDatabaseReasonProvisioning,
				"Logical database access is waiting for the database",
			)
			setLogicalDatabaseCondition(
				&logicalDatabase,
				logicalDatabaseConditionReady,
				metav1.ConditionFalse,
				logicalDatabaseReasonProvisioning,
				"Logical database is provisioning",
			)
			result = ctrl.Result{RequeueAfter: logicalDatabaseRequeueDelay}
		}
	}

	if apiequality.Semantic.DeepEqual(original.Status, logicalDatabase.Status) {
		return result, nil
	}

	if err := r.Status().Update(ctx, &logicalDatabase); err != nil {
		if apierrors.IsConflict(err) {
			logger.Info("LogicalDatabase status update conflict; requeueing")
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "failed to update LogicalDatabase status")
		return ctrl.Result{}, err
	}

	return result, nil
}

func (r *LogicalDatabaseReconciler) validateLogicalDatabase(
	ctx context.Context,
	logicalDatabase *storagev1alpha1.LogicalDatabase,
) ([]storagev1alpha1.LogicalDatabaseValidationError, error) {
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

	addRequiredStringError(logicalDatabaseValidationFieldServerRefName, logicalDatabase.Spec.ServerRef.Name)
	addRequiredStringError("spec.databaseKey", logicalDatabase.Spec.DatabaseKey)
	addRequiredStringError("spec.tenant.id", logicalDatabase.Spec.Tenant.ID)
	addRequiredStringError("spec.tenant.environment", logicalDatabase.Spec.Tenant.Environment)
	addRequiredStringError("spec.access.app.name", logicalDatabase.Spec.Access.App.Name)
	addRequiredStringError("spec.access.app.principalId", logicalDatabase.Spec.Access.App.PrincipalId)
	addRequiredStringError("spec.access.owner.name", logicalDatabase.Spec.Access.Owner.Name)
	addRequiredStringError("spec.access.owner.principalId", logicalDatabase.Spec.Access.Owner.PrincipalId)

	derivedDatabaseName := dbUtil.DeriveLogicalDatabaseName(
		logicalDatabase.Spec.Tenant.ID,
		logicalDatabase.Spec.Tenant.Environment,
		logicalDatabase.Spec.DatabaseKey,
	)
	if logicalDatabase.Status.DatabaseName != "" && logicalDatabase.Status.DatabaseName != derivedDatabaseName {
		validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
			Field:   logicalDatabaseValidationFieldDatabaseName,
			Reason:  logicalDatabaseValidationReasonImmutable,
			Message: fmt.Sprintf("derived database name changed from %q to %q; recreate LogicalDatabase to use a new name", logicalDatabase.Status.DatabaseName, derivedDatabaseName),
		})
	}

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
		return validationErrors, nil
	}

	var db storagev1alpha1.Database
	if err := r.Get(ctx, types.NamespacedName{
		Name:      serverName,
		Namespace: logicalDatabase.Namespace,
	}, &db); err != nil {
		if apierrors.IsNotFound(err) {
			validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
				Field:   logicalDatabaseValidationFieldServerRefName,
				Reason:  logicalDatabaseValidationReasonNotFound,
				Message: fmt.Sprintf("Database %q was not found in namespace %q", serverName, logicalDatabase.Namespace),
			})
			return validationErrors, nil
		}
		return nil, fmt.Errorf("get Database %s/%s: %w", logicalDatabase.Namespace, serverName, err)
	}

	if databaseMode(&db) != storagev1alpha1.DatabaseModeShared {
		validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
			Field:   logicalDatabaseValidationFieldServerRefName,
			Reason:  logicalDatabaseValidationReasonNotShared,
			Message: fmt.Sprintf("Database %q must have spec.mode Shared", serverName),
		})
	}

	return validationErrors, nil
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
		Owns(&dbforpostgresqlv1.FlexibleServersDatabase{}).
		Owns(&batchv1.Job{}).
		Watches(&storagev1alpha1.Database{}, handler.EnqueueRequestsFromMapFunc(r.mapDatabaseToLogicalDatabases)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
