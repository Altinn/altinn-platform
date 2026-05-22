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
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
)

const (
	databaseValidationReasonRequired      = "Required"
	databaseValidationReasonNotFound      = "NotFound"
	databaseValidationReasonUnsupported   = "Unsupported"
	databaseValidationReasonInvalid       = "Invalid"
	databaseValidationReasonConflict      = "Conflict"
	databaseValidationReasonImmutable     = "Immutable"
	databaseValidationFieldMetadataName   = "metadata.name"
	databaseValidationFieldSpecName       = "spec.name"
	databaseValidationFieldServerName     = "spec.server.name"
	databaseValidationFieldDeletionPolicy = "spec.deletionPolicy"
	databaseValidationFieldDatabaseName   = "status.databaseName"
	databaseMaxNameLength                 = 63
)

// DatabaseReconciler reconciles a Database object.
type DatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Config config.OperatorConfig
}

// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=databases,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=databases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=databaseservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=application.dis.altinn.cloud,resources=applicationidentities,verbs=get;list;watch
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleserversdatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleserversdatabases/status,verbs=get
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("database", req.NamespacedName)

	var database storagev1alpha1.Database
	if err := r.Get(ctx, req.NamespacedName, &database); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !database.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	original := database.DeepCopy()
	validationErrors, databaseName, err := r.validateDatabase(ctx, &database)
	if err != nil {
		logger.Error(err, "failed to validate Database")
		return ctrl.Result{}, err
	}
	database.Status.ObservedGeneration = database.Generation
	database.Status.ValidationErrors = validationErrors

	result := ctrl.Result{}
	if len(validationErrors) > 0 {
		setDatabaseConditions(
			&database,
			metav1.ConditionFalse,
			databaseReasonValidationFailed,
			"Database validation failed",
		)
	} else {
		if err := r.ensureFlexibleServersDatabase(ctx, &database, databaseName); err != nil {
			var conflictErr *databaseASOResourceConflictError
			if !errors.As(err, &conflictErr) {
				logger.Error(err, "failed to ensure FlexibleServersDatabase for Database")
				return ctrl.Result{}, err
			}

			database.Status.ValidationErrors = appendDatabaseValidationError(
				database.Status.ValidationErrors,
				databaseValidationFieldSpecName,
				databaseValidationReasonConflict,
				fmt.Sprintf("database %q on server %q is already managed by %s; choose another spec.name", databaseName, database.Spec.Server.Name, conflictErr.ownerDescription()),
			)
			setDatabaseConditions(
				&database,
				metav1.ConditionFalse,
				databaseReasonValidationFailed,
				"Database validation failed",
			)
		} else {
			database.Status.DatabaseName = databaseName

			ready, host, err := r.databaseReady(ctx, logger, &database)
			if err != nil {
				logger.Error(err, "failed to check Database readiness")
				return ctrl.Result{}, err
			}
			if ready {
				database.Status.Host = host
				database.Status.Port = databasePort
				setDatabaseCondition(
					&database,
					databaseConditionDatabaseReady,
					metav1.ConditionTrue,
					databaseReasonDatabaseReady,
					"Database is ready",
				)

				accessReady, accessReason, accessMessage, err := r.ensureDatabaseAccess(ctx, logger, &database)
				if err != nil {
					logger.Error(err, "failed to ensure Database access")
					return ctrl.Result{}, err
				}
				if accessReady {
					setDatabaseCondition(
						&database,
						databaseConditionAccessReady,
						metav1.ConditionTrue,
						accessReason,
						accessMessage,
					)
					setDatabaseCondition(
						&database,
						databaseConditionReady,
						metav1.ConditionTrue,
						databaseReasonReady,
						"Database and access are ready",
					)
				} else {
					setDatabaseCondition(
						&database,
						databaseConditionAccessReady,
						metav1.ConditionFalse,
						accessReason,
						accessMessage,
					)
					setDatabaseCondition(
						&database,
						databaseConditionReady,
						metav1.ConditionFalse,
						databaseReasonProvisioning,
						"Database access is provisioning",
					)
					result = ctrl.Result{RequeueAfter: databaseRequeueDelay}
				}
			} else {
				setDatabaseCondition(
					&database,
					databaseConditionDatabaseReady,
					metav1.ConditionFalse,
					databaseReasonProvisioning,
					"Database is provisioning",
				)
				setDatabaseCondition(
					&database,
					databaseConditionAccessReady,
					metav1.ConditionFalse,
					databaseReasonProvisioning,
					"Database access is waiting for the database",
				)
				setDatabaseCondition(
					&database,
					databaseConditionReady,
					metav1.ConditionFalse,
					databaseReasonProvisioning,
					"Database is provisioning",
				)
				result = ctrl.Result{RequeueAfter: databaseRequeueDelay}
			}
		}
	}

	if apiequality.Semantic.DeepEqual(original.Status, database.Status) {
		return result, nil
	}

	if err := r.Status().Update(ctx, &database); err != nil {
		if apierrors.IsConflict(err) {
			logger.Info("Database status update conflict; requeueing")
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "failed to update Database status")
		return ctrl.Result{}, err
	}

	return result, nil
}

func appendDatabaseValidationError(
	validationErrors []storagev1alpha1.DatabaseValidationError,
	field, reason, message string,
) []storagev1alpha1.DatabaseValidationError {
	for i := range validationErrors {
		if validationErrors[i].Field == field {
			return validationErrors
		}
	}

	return append(validationErrors, storagev1alpha1.DatabaseValidationError{
		Field:   field,
		Reason:  reason,
		Message: message,
	})
}

func (r *DatabaseReconciler) validateDatabase(
	ctx context.Context,
	database *storagev1alpha1.Database,
) ([]storagev1alpha1.DatabaseValidationError, string, error) {
	var validationErrors []storagev1alpha1.DatabaseValidationError
	databaseName := database.Spec.Name
	serverName := strings.TrimSpace(database.Spec.Server.Name)

	addRequiredStringError := func(field, value string) {
		if strings.TrimSpace(value) != "" {
			return
		}
		validationErrors = appendDatabaseValidationError(
			validationErrors,
			field,
			databaseValidationReasonRequired,
			fmt.Sprintf("%s must be set", field),
		)
	}

	addRequiredStringError(databaseValidationFieldServerName, database.Spec.Server.Name)
	addRequiredStringError(databaseValidationFieldMetadataName, database.Name)
	addRequiredStringError(databaseValidationFieldSpecName, database.Spec.Name)
	addRequiredStringError("spec.access.app.name", database.Spec.Access.App.Name)
	addRequiredStringError("spec.access.app.principalId", database.Spec.Access.App.PrincipalId)
	addRequiredStringError("spec.access.owner.name", database.Spec.Access.Owner.Name)
	addRequiredStringError("spec.access.owner.principalId", database.Spec.Access.Owner.PrincipalId)

	if database.Spec.Name != "" && strings.TrimSpace(database.Spec.Name) != database.Spec.Name {
		validationErrors = appendDatabaseValidationError(
			validationErrors,
			databaseValidationFieldSpecName,
			databaseValidationReasonInvalid,
			"spec.name must not have leading or trailing whitespace",
		)
	}

	if len(databaseName) > databaseMaxNameLength {
		validationErrors = appendDatabaseValidationError(
			validationErrors,
			databaseValidationFieldSpecName,
			databaseValidationReasonInvalid,
			fmt.Sprintf("spec.name must be at most %d characters", databaseMaxNameLength),
		)
	}

	if database.Spec.Server.Name != "" && serverName != database.Spec.Server.Name {
		validationErrors = appendDatabaseValidationError(
			validationErrors,
			databaseValidationFieldServerName,
			databaseValidationReasonInvalid,
			"spec.server.name must not have leading or trailing whitespace",
		)
	}

	if database.Status.DatabaseName != "" && databaseName != "" && database.Status.DatabaseName != databaseName {
		validationErrors = appendDatabaseValidationError(
			validationErrors,
			databaseValidationFieldDatabaseName,
			databaseValidationReasonImmutable,
			fmt.Sprintf("database name changed from %q to %q; recreate Database to use a new name", database.Status.DatabaseName, databaseName),
		)
	}

	if database.Spec.DeletionPolicy != "" &&
		database.Spec.DeletionPolicy != storagev1alpha1.DatabaseDeletionPolicyRetain {
		validationErrors = appendDatabaseValidationError(
			validationErrors,
			databaseValidationFieldDeletionPolicy,
			databaseValidationReasonUnsupported,
			"spec.deletionPolicy only supports Retain",
		)
	}

	if serverName == "" {
		return validationErrors, databaseName, nil
	}

	var db storagev1alpha1.DatabaseServer
	if err := r.Get(ctx, types.NamespacedName{
		Name:      serverName,
		Namespace: database.Namespace,
	}, &db); err != nil {
		if apierrors.IsNotFound(err) {
			validationErrors = appendDatabaseValidationError(
				validationErrors,
				databaseValidationFieldServerName,
				databaseValidationReasonNotFound,
				fmt.Sprintf("DatabaseServer %q was not found in namespace %q", serverName, database.Namespace),
			)
			return validationErrors, databaseName, nil
		}
		return nil, databaseName, fmt.Errorf("get DatabaseServer %s/%s: %w", database.Namespace, serverName, err)
	}

	return validationErrors, databaseName, nil
}

func (r *DatabaseReconciler) mapDatabaseServerToDatabases(
	ctx context.Context,
	obj client.Object,
) []ctrl.Request {
	var list storagev1alpha1.DatabaseList
	if err := r.List(ctx, &list, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}

	requests := make([]ctrl.Request, 0)
	for i := range list.Items {
		database := list.Items[i]
		if strings.TrimSpace(database.Spec.Server.Name) != obj.GetName() {
			continue
		}
		requests = append(requests, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      database.Name,
				Namespace: database.Namespace,
			},
		})
	}
	return requests
}

func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.Database{}).
		Owns(&dbforpostgresqlv1.FlexibleServersDatabase{}).
		Owns(&batchv1.Job{}).
		Watches(&storagev1alpha1.DatabaseServer{}, handler.EnqueueRequestsFromMapFunc(r.mapDatabaseServerToDatabases)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
