package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	k8sutil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/k8s"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	"github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	databaseConditionReady         = "Ready"
	databaseConditionDatabaseReady = "DatabaseReady"
	databaseConditionAccessReady   = "AccessReady"

	databaseReasonValidationFailed = "ValidationFailed"
	databaseReasonProvisioning     = "Provisioning"
	databaseReasonReady            = "Ready"
	databaseReasonDatabaseReady    = "Ready"

	databasePort         = int32(5432)
	databaseRequeueDelay = 15 * time.Second
	databaseNameLabelKey = "dis.altinn.cloud/database-name"

	// String literals used as configuration values or fallback identifiers.
	searchPathScopeDatabase         = "database"
	databaseFallbackName            = "database"
	disDatabaseNamePrefix           = "dis-database"
	labelValueTrue                  = "true"
	databaseAccessProvisionLabelKey = "dis.altinn.cloud/access-provision"
	userProvisionLabelKey           = "dis.altinn.cloud/user-provision"
)

func (r *DatabaseReconciler) ensureFlexibleServersDatabase(
	ctx context.Context,
	database *storagev1alpha1.Database,
	databaseName string,
) error {
	ns := database.Namespace
	serverName := strings.TrimSpace(database.Spec.Server.Name)
	resourceName := databaseASOResourceName(serverName, databaseName)

	desiredSpec := dbforpostgresqlv1.FlexibleServersDatabase_Spec{
		AzureName: databaseName,
		Owner: &genruntime.KnownResourceReference{
			Name: serverName,
		},
	}
	desiredLabels := map[string]string{
		databaseServerNameLabelKey: serverName,
		databaseNameLabelKey:       database.Name,
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
		if err := controllerutil.SetControllerReference(database, flexibleServersDatabase, r.Scheme); err != nil {
			return fmt.Errorf("set controller reference on FlexibleServersDatabase: %w", err)
		}
		if err := r.Create(ctx, flexibleServersDatabase); err != nil {
			if apierrors.IsAlreadyExists(err) {
				if err := r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: ns}, &existing); err != nil {
					return fmt.Errorf("get FlexibleServersDatabase %s/%s after create conflict: %w", ns, resourceName, err)
				}
				return ensureDatabaseASOResourceOwnedBy(database, &existing)
			}
			return fmt.Errorf("create FlexibleServersDatabase %s/%s: %w", ns, resourceName, err)
		}
		return nil
	}

	if err := ensureDatabaseASOResourceOwnedBy(database, &existing); err != nil {
		return err
	}

	updated := false
	existing.Labels, updated = k8sutil.SyncSpecAndLabels(&existing.Spec, desiredSpec, existing.Labels, desiredLabels)

	if existing.Annotations == nil {
		existing.Annotations = map[string]string{}
	}
	for key, value := range desiredAnnotations {
		if existing.Annotations[key] != value {
			existing.Annotations[key] = value
			updated = true
		}
	}

	if !updated {
		return nil
	}

	if err := r.Update(ctx, &existing); err != nil {
		return fmt.Errorf("update FlexibleServersDatabase %s/%s: %w", ns, resourceName, err)
	}

	return nil
}

func (r *DatabaseReconciler) databaseReady(
	ctx context.Context,
	logger logr.Logger,
	database *storagev1alpha1.Database,
) (bool, string, error) {
	ns := database.Namespace
	serverName := strings.TrimSpace(database.Spec.Server.Name)
	resourceName := databaseASOResourceName(serverName, database.Status.DatabaseName)

	var flexibleServersDatabase dbforpostgresqlv1.FlexibleServersDatabase
	if err := r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: ns}, &flexibleServersDatabase); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("FlexibleServersDatabase not found yet", "database", resourceName)
			return false, "", nil
		}
		return false, "", fmt.Errorf("get FlexibleServersDatabase %s/%s: %w", ns, resourceName, err)
	}

	if err := ensureDatabaseASOResourceOwnedBy(database, &flexibleServersDatabase); err != nil {
		return false, "", err
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

func ensureDatabaseASOResourceOwnedBy(
	database *storagev1alpha1.Database,
	asoDatabase *dbforpostgresqlv1.FlexibleServersDatabase,
) error {
	if metav1.IsControlledBy(asoDatabase, database) {
		return nil
	}

	resource := fmt.Sprintf("%s/%s", asoDatabase.Namespace, asoDatabase.Name)
	controllerRef := metav1.GetControllerOf(asoDatabase)
	if controllerRef == nil {
		return &databaseASOResourceConflictError{
			resource:          resource,
			databaseNamespace: database.Namespace,
			databaseName:      database.Name,
		}
	}

	return &databaseASOResourceConflictError{
		resource:          resource,
		controllerKind:    controllerRef.Kind,
		controllerName:    controllerRef.Name,
		databaseNamespace: database.Namespace,
		databaseName:      database.Name,
	}
}

type databaseASOResourceConflictError struct {
	resource          string
	controllerKind    string
	controllerName    string
	databaseNamespace string
	databaseName      string
}

func (err *databaseASOResourceConflictError) Error() string {
	if err.controllerKind == "" {
		return fmt.Sprintf(
			"FlexibleServersDatabase %s exists but is not controlled by Database %s/%s",
			err.resource,
			err.databaseNamespace,
			err.databaseName,
		)
	}

	return fmt.Sprintf(
		"FlexibleServersDatabase %s is controlled by %s %s, not Database %s/%s",
		err.resource,
		err.controllerKind,
		err.controllerName,
		err.databaseNamespace,
		err.databaseName,
	)
}

func (err *databaseASOResourceConflictError) ownerDescription() string {
	if err.controllerKind == "" {
		return "an existing Kubernetes resource"
	}
	return fmt.Sprintf("%s %s", err.controllerKind, err.controllerName)
}

func databaseASOResourceName(serverName, databaseName string) string {
	const maxResourceNameLen = 253

	source := naming.SanitizeLowerHyphen(serverName + "-" + databaseName)
	if source == "" {
		source = databaseFallbackName
	}
	if len(source) <= maxResourceNameLen {
		return source
	}

	hash := naming.StableSHA256Hex(source)[:8]
	return naming.WithHashSuffixOnOverflow(source, maxResourceNameLen, hash, databaseFallbackName)
}

func setDatabaseCondition(
	database *storagev1alpha1.Database,
	conditionType string,
	status metav1.ConditionStatus,
	reason,
	message string,
) {
	meta.SetStatusCondition(&database.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: database.Generation,
	})
}

func setDatabaseConditions(
	database *storagev1alpha1.Database,
	status metav1.ConditionStatus,
	reason,
	message string,
) {
	for _, conditionType := range []string{
		databaseConditionReady,
		databaseConditionDatabaseReady,
		databaseConditionAccessReady,
	} {
		setDatabaseCondition(
			database,
			conditionType,
			status,
			reason,
			message,
		)
	}
}
