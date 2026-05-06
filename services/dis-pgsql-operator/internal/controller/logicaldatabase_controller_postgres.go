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
	logicalDatabaseConditionReady         = "Ready"
	logicalDatabaseConditionDatabaseReady = "DatabaseReady"
	logicalDatabaseConditionAccessReady   = "AccessReady"

	logicalDatabaseReasonValidationFailed     = "ValidationFailed"
	logicalDatabaseReasonProvisioning         = "Provisioning"
	logicalDatabaseReasonDatabaseReady        = "Ready"
	logicalDatabaseReasonProvisioningDeferred = "ProvisioningNotImplemented"

	logicalDatabasePort         = int32(5432)
	logicalDatabaseRequeueDelay = 15 * time.Second
	logicalDatabaseLabelKey     = "dis.altinn.cloud/logical-database-name"
)

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
