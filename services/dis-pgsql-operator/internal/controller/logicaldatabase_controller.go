package controller

import (
	"context"
	"errors"
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
	logicalDatabaseValidationReasonUnsupported   = "Unsupported"
	logicalDatabaseValidationReasonInvalid       = "Invalid"
	logicalDatabaseValidationReasonConflict      = "Conflict"
	logicalDatabaseValidationReasonImmutable     = "Immutable"
	logicalDatabaseValidationFieldMetadataName   = "metadata.name"
	logicalDatabaseValidationFieldSpecName       = "spec.name"
	logicalDatabaseValidationFieldServerName     = "spec.server.name"
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
// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=databaseservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=application.dis.altinn.cloud,resources=applicationidentities,verbs=get;list;watch
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
	validationErrors, databaseName, err := r.validateLogicalDatabase(ctx, &logicalDatabase)
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
		if err := r.ensureFlexibleServersDatabase(ctx, &logicalDatabase, databaseName); err != nil {
			var conflictErr *logicalDatabaseASOResourceConflictError
			if !errors.As(err, &conflictErr) {
				logger.Error(err, "failed to ensure FlexibleServersDatabase for LogicalDatabase")
				return ctrl.Result{}, err
			}

			logicalDatabase.Status.ValidationErrors = append(logicalDatabase.Status.ValidationErrors, storagev1alpha1.LogicalDatabaseValidationError{
				Field:   logicalDatabaseValidationFieldSpecName,
				Reason:  logicalDatabaseValidationReasonConflict,
				Message: fmt.Sprintf("database %q on server %q is already managed by %s; choose another spec.name", databaseName, logicalDatabase.Spec.Server.Name, conflictErr.ownerDescription()),
			})
			setLogicalDatabaseConditions(
				&logicalDatabase,
				metav1.ConditionFalse,
				logicalDatabaseReasonValidationFailed,
				"LogicalDatabase validation failed",
			)
		} else {
			logicalDatabase.Status.DatabaseName = databaseName

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
) ([]storagev1alpha1.LogicalDatabaseValidationError, string, error) {
	var validationErrors []storagev1alpha1.LogicalDatabaseValidationError
	databaseName := dbUtil.LogicalDatabaseName(logicalDatabase.Spec.Name)
	serverName := strings.TrimSpace(logicalDatabase.Spec.Server.Name)

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

	addRequiredStringError(logicalDatabaseValidationFieldServerName, logicalDatabase.Spec.Server.Name)
	addRequiredStringError(logicalDatabaseValidationFieldMetadataName, logicalDatabase.Name)
	addRequiredStringError(logicalDatabaseValidationFieldSpecName, logicalDatabase.Spec.Name)
	addRequiredStringError("spec.access.app.name", logicalDatabase.Spec.Access.App.Name)
	addRequiredStringError("spec.access.app.principalId", logicalDatabase.Spec.Access.App.PrincipalId)
	addRequiredStringError("spec.access.owner.name", logicalDatabase.Spec.Access.Owner.Name)
	addRequiredStringError("spec.access.owner.principalId", logicalDatabase.Spec.Access.Owner.PrincipalId)

	if logicalDatabase.Spec.Name != "" && strings.TrimSpace(logicalDatabase.Spec.Name) != logicalDatabase.Spec.Name {
		validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
			Field:   logicalDatabaseValidationFieldSpecName,
			Reason:  logicalDatabaseValidationReasonInvalid,
			Message: "spec.name must not have leading or trailing whitespace",
		})
	}

	if len(databaseName) > dbUtil.MaxLogicalDatabaseNameLength {
		validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
			Field:   logicalDatabaseValidationFieldSpecName,
			Reason:  logicalDatabaseValidationReasonInvalid,
			Message: fmt.Sprintf("spec.name must be at most %d characters", dbUtil.MaxLogicalDatabaseNameLength),
		})
	}

	if logicalDatabase.Spec.Server.Name != "" && serverName != logicalDatabase.Spec.Server.Name {
		validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
			Field:   logicalDatabaseValidationFieldServerName,
			Reason:  logicalDatabaseValidationReasonInvalid,
			Message: "spec.server.name must not have leading or trailing whitespace",
		})
	}

	if logicalDatabase.Status.DatabaseName != "" && databaseName != "" && logicalDatabase.Status.DatabaseName != databaseName {
		validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
			Field:   logicalDatabaseValidationFieldDatabaseName,
			Reason:  logicalDatabaseValidationReasonImmutable,
			Message: fmt.Sprintf("database name changed from %q to %q; recreate LogicalDatabase to use a new name", logicalDatabase.Status.DatabaseName, databaseName),
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

	if serverName == "" {
		return validationErrors, databaseName, nil
	}

	var db storagev1alpha1.DatabaseServer
	if err := r.Get(ctx, types.NamespacedName{
		Name:      serverName,
		Namespace: logicalDatabase.Namespace,
	}, &db); err != nil {
		if apierrors.IsNotFound(err) {
			validationErrors = append(validationErrors, storagev1alpha1.LogicalDatabaseValidationError{
				Field:   logicalDatabaseValidationFieldServerName,
				Reason:  logicalDatabaseValidationReasonNotFound,
				Message: fmt.Sprintf("DatabaseServer %q was not found in namespace %q", serverName, logicalDatabase.Namespace),
			})
			return validationErrors, databaseName, nil
		}
		return nil, databaseName, fmt.Errorf("get DatabaseServer %s/%s: %w", logicalDatabase.Namespace, serverName, err)
	}

	return validationErrors, databaseName, nil
}

func (r *LogicalDatabaseReconciler) mapDatabaseServerToLogicalDatabases(
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
		if strings.TrimSpace(logicalDatabase.Spec.Server.Name) != obj.GetName() {
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
		Watches(&storagev1alpha1.DatabaseServer{}, handler.EnqueueRequestsFromMapFunc(r.mapDatabaseServerToLogicalDatabases)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
