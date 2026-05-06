package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
	k8sutil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/k8s"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	genruntime "github.com/Azure/azure-service-operator/v2/pkg/genruntime"
)

const (
	logicalDatabaseConditionReady         = "Ready"
	logicalDatabaseConditionDatabaseReady = "DatabaseReady"
	logicalDatabaseConditionAccessReady   = "AccessReady"

	logicalDatabaseReasonValidationFailed        = "ValidationFailed"
	logicalDatabaseReasonProvisioning            = "Provisioning"
	logicalDatabaseReasonDatabaseReady           = "Ready"
	logicalDatabaseReasonProvisioningDeferred    = "ProvisioningNotImplemented"
	logicalDatabaseValidationReasonRequired      = "Required"
	logicalDatabaseValidationReasonNotFound      = "NotFound"
	logicalDatabaseValidationReasonNotShared     = "NotShared"
	logicalDatabaseValidationReasonUnsupported   = "Unsupported"
	logicalDatabaseValidationReasonImmutable     = "Immutable"
	logicalDatabaseValidationFieldServerRefName  = "spec.serverRef.name"
	logicalDatabaseValidationFieldDeletionPolicy = "spec.deletionPolicy"
	logicalDatabaseValidationFieldDatabaseName   = "status.databaseName"
	logicalDatabasePort                          = int32(5432)
	logicalDatabaseRequeueDelay                  = 15 * time.Second
	logicalDatabaseLabelKey                      = "dis.altinn.cloud/logical-database-name"
)

// LogicalDatabaseReconciler reconciles a LogicalDatabase object.
type LogicalDatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=logicaldatabases,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=logicaldatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=databases,verbs=get;list;watch
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleserversdatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleserversdatabases/status,verbs=get

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
			setLogicalDatabaseCondition(
				&logicalDatabase,
				logicalDatabaseConditionAccessReady,
				metav1.ConditionFalse,
				logicalDatabaseReasonProvisioningDeferred,
				"LogicalDatabase access provisioning is not implemented in this release",
			)
			setLogicalDatabaseCondition(
				&logicalDatabase,
				logicalDatabaseConditionReady,
				metav1.ConditionFalse,
				logicalDatabaseReasonProvisioningDeferred,
				"LogicalDatabase access provisioning is not implemented in this release",
			)
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
				logicalDatabaseReasonProvisioningDeferred,
				"LogicalDatabase access provisioning is not implemented in this release",
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
		logger.Error(err, "failed to update LogicalDatabase status")
		return ctrl.Result{}, err
	}

	return result, nil
}

func (r *LogicalDatabaseReconciler) ensureFlexibleServersDatabase(
	ctx context.Context,
	logicalDatabase *storagev1alpha1.LogicalDatabase,
) error {
	ns := logicalDatabase.Namespace
	serverName := strings.TrimSpace(logicalDatabase.Spec.ServerRef.Name)
	databaseName := logicalDatabase.Status.DatabaseName
	resourceName := logicalDatabaseASOResourceName(serverName, databaseName)

	desiredSpec := dbforpostgresqlv1.FlexibleServersDatabase_Spec{
		AzureName: databaseName,
		Owner: &genruntime.KnownResourceReference{
			Name: serverName,
		},
	}
	desiredLabels := map[string]string{
		databaseNameLabelKey:    serverName,
		logicalDatabaseLabelKey: logicalDatabase.Name,
	}
	desiredAnnotations := map[string]string{
		annotations.ReconcilePolicy: string(annotations.ReconcilePolicyDetachOnDelete),
	}

	var existing dbforpostgresqlv1.FlexibleServersDatabase
	if err := r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: ns}, &existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get FlexibleServersDatabase %s/%s: %w", ns, resourceName, err)
		}
		flexibleServersDatabase := &dbforpostgresqlv1.FlexibleServersDatabase{
			ObjectMeta: metav1.ObjectMeta{
				Name:        resourceName,
				Namespace:   ns,
				Labels:      desiredLabels,
				Annotations: desiredAnnotations,
			},
			Spec: desiredSpec,
		}
		if err := controllerutil.SetControllerReference(logicalDatabase, flexibleServersDatabase, r.Scheme); err != nil {
			return fmt.Errorf("set controller reference on FlexibleServersDatabase: %w", err)
		}
		if err := r.Create(ctx, flexibleServersDatabase); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return fmt.Errorf("create FlexibleServersDatabase %s/%s: %w", ns, resourceName, err)
		}
		return nil
	}

	updated := false
	if !apiequality.Semantic.DeepEqual(existing.Spec, desiredSpec) {
		existing.Spec = desiredSpec
		updated = true
	}

	var labelsUpdated bool
	existing.Labels, labelsUpdated = k8sutil.SyncSpecAndLabels(&existing.Spec, desiredSpec, existing.Labels, desiredLabels)
	updated = updated || labelsUpdated

	if existing.Annotations == nil {
		existing.Annotations = map[string]string{}
	}
	for key, value := range desiredAnnotations {
		if existing.Annotations[key] != value {
			existing.Annotations[key] = value
			updated = true
		}
	}

	if updated {
		if err := r.Update(ctx, &existing); err != nil {
			return fmt.Errorf("update FlexibleServersDatabase %s/%s: %w", ns, resourceName, err)
		}
	}

	return nil
}

func (r *LogicalDatabaseReconciler) logicalDatabaseReady(
	ctx context.Context,
	logger logr.Logger,
	logicalDatabase *storagev1alpha1.LogicalDatabase,
) (bool, string, error) {
	ns := logicalDatabase.Namespace
	serverName := strings.TrimSpace(logicalDatabase.Spec.ServerRef.Name)
	resourceName := logicalDatabaseASOResourceName(serverName, logicalDatabase.Status.DatabaseName)

	var flexibleServersDatabase dbforpostgresqlv1.FlexibleServersDatabase
	if err := r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: ns}, &flexibleServersDatabase); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("FlexibleServersDatabase not found yet", "database", resourceName)
			return false, "", nil
		}
		return false, "", fmt.Errorf("get FlexibleServersDatabase %s/%s: %w", ns, resourceName, err)
	}

	databaseStatus, databaseReason, databaseMessage, databaseReady := readyConditionInfo(flexibleServersDatabase.Status.Conditions)
	if !databaseReady || databaseStatus != metav1.ConditionTrue {
		logger.Info("FlexibleServersDatabase not ready yet",
			"database", resourceName,
			"status", databaseStatus,
			"reason", databaseReason,
			"message", databaseMessage,
		)
		return false, "", nil
	}

	var server dbforpostgresqlv1.FlexibleServer
	if err := r.Get(ctx, types.NamespacedName{Name: serverName, Namespace: ns}, &server); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("FlexibleServer not found yet", "server", serverName)
			return false, "", nil
		}
		return false, "", fmt.Errorf("get FlexibleServer %s/%s: %w", ns, serverName, err)
	}

	if server.Status.FullyQualifiedDomainName == nil || strings.TrimSpace(*server.Status.FullyQualifiedDomainName) == "" {
		logger.Info("FlexibleServer host not available yet", "server", serverName)
		return false, "", nil
	}

	return true, strings.TrimSpace(*server.Status.FullyQualifiedDomainName), nil
}

func logicalDatabaseASOResourceName(serverName, databaseName string) string {
	const maxResourceNameLen = 253

	source := naming.SanitizeLowerHyphen(serverName + "-" + databaseName)
	if source == "" {
		source = "logicaldatabase"
	}
	if len(source) <= maxResourceNameLen {
		return source
	}

	hash := naming.StableSHA256Hex(source)[:8]
	return naming.WithHashSuffixOnOverflow(source, maxResourceNameLen, hash, "logicaldatabase")
}

func setLogicalDatabaseCondition(
	logicalDatabase *storagev1alpha1.LogicalDatabase,
	conditionType string,
	status metav1.ConditionStatus,
	reason,
	message string,
) {
	meta.SetStatusCondition(&logicalDatabase.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: logicalDatabase.Generation,
	})
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
		setLogicalDatabaseCondition(
			logicalDatabase,
			conditionType,
			status,
			reason,
			message,
		)
	}
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
	addRequiredStringError("spec.access.identity.name", logicalDatabase.Spec.Access.Identity.Name)
	addRequiredStringError("spec.access.identity.principalId", logicalDatabase.Spec.Access.Identity.PrincipalId)

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
		Watches(&storagev1alpha1.Database{}, handler.EnqueueRequestsFromMapFunc(r.mapDatabaseToLogicalDatabases)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
