package controller

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
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
	databaseValidationReasonRequired        = "Required"
	databaseValidationReasonNotFound        = "NotFound"
	databaseValidationReasonUnsupported     = "Unsupported"
	databaseValidationReasonInvalid         = "Invalid"
	databaseValidationReasonConflict        = "Conflict"
	databaseValidationReasonImmutable       = "Immutable"
	databaseValidationFieldMetadataName     = "metadata.name"
	databaseValidationFieldSpecName         = "spec.name"
	databaseValidationFieldServerName       = "spec.server.name"
	databaseValidationFieldAccessPrincipals = "spec.access.principals"
	databaseValidationFieldDeletionPolicy   = "spec.deletionPolicy"
	databaseValidationFieldDatabaseName     = "status.databaseName"
	databaseMaxNameLength                   = 63
	databaseMaxPrincipalNameLength          = 63
)

var entraPrincipalIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

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
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

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
	validationErrors = validateDatabaseAccess(validationErrors, database)

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

func validateDatabaseAccess(
	validationErrors []storagev1alpha1.DatabaseValidationError,
	database *storagev1alpha1.Database,
) []storagev1alpha1.DatabaseValidationError {
	if len(database.Spec.Access.Principals) == 0 {
		return appendDatabaseValidationError(
			validationErrors,
			databaseValidationFieldAccessPrincipals,
			databaseValidationReasonRequired,
			"spec.access.principals must contain at least one principal",
		)
	}

	seen := map[string]string{}
	for i, principal := range database.Spec.Access.Principals {
		field := func(suffix string) string {
			if suffix == "" {
				return fmt.Sprintf("spec.access.principals[%d]", i)
			}
			return fmt.Sprintf("spec.access.principals[%d].%s", i, suffix)
		}

		roleField := field("role")
		switch principal.Role {
		case storagev1alpha1.DatabaseAccessRoleReader,
			storagev1alpha1.DatabaseAccessRoleWriter,
			storagev1alpha1.DatabaseAccessRoleOwner:
		case "":
			validationErrors = appendDatabaseValidationError(
				validationErrors,
				roleField,
				databaseValidationReasonRequired,
				"role must be set",
			)
		default:
			validationErrors = appendDatabaseValidationError(
				validationErrors,
				roleField,
				databaseValidationReasonInvalid,
				"role must be one of Reader, Writer, or Owner",
			)
		}

		hasIdentityRef := principal.IdentityRef != nil
		hasGroup := principal.Group != nil
		if hasIdentityRef == hasGroup {
			validationErrors = appendDatabaseValidationError(
				validationErrors,
				field(""),
				databaseValidationReasonInvalid,
				"exactly one principal source must be set: identityRef or group",
			)
			continue
		}

		var principalKey string
		if hasIdentityRef {
			refName := principal.IdentityRef.Name
			refField := field("identityRef.name")
			validationErrors = validateAccessName(validationErrors, refField, refName, databaseMaxPrincipalNameLength)
			if refName != "" {
				for _, msg := range validation.IsDNS1123Subdomain(refName) {
					validationErrors = appendDatabaseValidationError(
						validationErrors,
						refField,
						databaseValidationReasonInvalid,
						fmt.Sprintf("identityRef.name must be a valid Kubernetes name: %s", msg),
					)
				}
				principalKey = "identityRef:" + refName
			}
		}

		if hasGroup {
			groupName := principal.Group.Name
			groupPrincipalID := principal.Group.PrincipalId
			groupNameField := field("group.name")
			groupPrincipalIDField := field("group.principalId")

			validationErrors = validateAccessName(validationErrors, groupNameField, groupName, databaseMaxPrincipalNameLength)
			if strings.TrimSpace(groupPrincipalID) == "" {
				validationErrors = appendDatabaseValidationError(
					validationErrors,
					groupPrincipalIDField,
					databaseValidationReasonRequired,
					"group.principalId must be set",
				)
			} else if groupPrincipalID != strings.TrimSpace(groupPrincipalID) {
				validationErrors = appendDatabaseValidationError(
					validationErrors,
					groupPrincipalIDField,
					databaseValidationReasonInvalid,
					"group.principalId must not have leading or trailing whitespace",
				)
			} else if !entraPrincipalIDPattern.MatchString(groupPrincipalID) {
				validationErrors = appendDatabaseValidationError(
					validationErrors,
					groupPrincipalIDField,
					databaseValidationReasonInvalid,
					"group.principalId must be an Entra object ID GUID",
				)
			}
			if groupPrincipalID != "" {
				principalKey = "group:" + strings.ToLower(groupPrincipalID)
			} else if groupName != "" {
				principalKey = "group-name:" + groupName
			}
		}

		if principalKey == "" {
			continue
		}
		if firstField, ok := seen[principalKey]; ok {
			validationErrors = appendDatabaseValidationError(
				validationErrors,
				field(""),
				databaseValidationReasonConflict,
				fmt.Sprintf("principal duplicates %s", firstField),
			)
			continue
		}
		seen[principalKey] = field("")
	}

	return validationErrors
}

func validateAccessName(
	validationErrors []storagev1alpha1.DatabaseValidationError,
	field, value string,
	maxLength int,
) []storagev1alpha1.DatabaseValidationError {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return appendDatabaseValidationError(
			validationErrors,
			field,
			databaseValidationReasonRequired,
			fmt.Sprintf("%s must be set", field),
		)
	}
	if trimmed != value {
		return appendDatabaseValidationError(
			validationErrors,
			field,
			databaseValidationReasonInvalid,
			fmt.Sprintf("%s must not have leading or trailing whitespace", field),
		)
	}
	if strings.ContainsRune(value, 0) {
		return appendDatabaseValidationError(
			validationErrors,
			field,
			databaseValidationReasonInvalid,
			fmt.Sprintf("%s must not contain NUL bytes", field),
		)
	}
	if len(value) > maxLength {
		return appendDatabaseValidationError(
			validationErrors,
			field,
			databaseValidationReasonInvalid,
			fmt.Sprintf("%s must be at most %d characters", field, maxLength),
		)
	}
	return validationErrors
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

func (r *DatabaseReconciler) mapApplicationIdentityToDatabases(
	ctx context.Context,
	obj client.Object,
) []ctrl.Request {
	identityName := obj.GetName()
	identityNamespace := obj.GetNamespace()

	var list storagev1alpha1.DatabaseList
	if err := r.List(ctx, &list, client.InNamespace(identityNamespace)); err != nil {
		return nil
	}

	requests := make([]ctrl.Request, 0)
	for i := range list.Items {
		database := list.Items[i]
		if !databaseReferencesApplicationIdentity(&database, identityName) {
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

func databaseReferencesApplicationIdentity(database *storagev1alpha1.Database, identityName string) bool {
	for _, principal := range database.Spec.Access.Principals {
		if principal.IdentityRef != nil && principal.IdentityRef.Name == identityName {
			return true
		}
	}
	return false
}

func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.Database{}).
		Owns(&dbforpostgresqlv1.FlexibleServersDatabase{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		Watches(&storagev1alpha1.DatabaseServer{}, handler.EnqueueRequestsFromMapFunc(r.mapDatabaseServerToDatabases)).
		Watches(&identityv1alpha1.ApplicationIdentity{}, handler.EnqueueRequestsFromMapFunc(r.mapApplicationIdentityToDatabases)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
