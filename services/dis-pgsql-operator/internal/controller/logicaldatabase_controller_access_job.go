package controller

import (
	"context"
	"fmt"
	"strings"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *LogicalDatabaseReconciler) ensureLogicalDatabaseAccess(
	ctx context.Context,
	logger logr.Logger,
	logicalDatabase *storagev1alpha1.LogicalDatabase,
) (bool, string, string, error) {
	accessReady := meta.FindStatusCondition(logicalDatabase.Status.Conditions, logicalDatabaseConditionAccessReady)
	if accessReady != nil &&
		accessReady.Status == metav1.ConditionTrue &&
		accessReady.ObservedGeneration == logicalDatabase.Generation {
		return true, logicalDatabaseReasonReady, "Logical database access is ready", nil
	}

	serverName := strings.TrimSpace(logicalDatabase.Spec.Server.Name)
	var db storagev1alpha1.DatabaseServer
	if err := r.Get(ctx, types.NamespacedName{
		Name:      serverName,
		Namespace: logicalDatabase.Namespace,
	}, &db); err != nil {
		if apierrors.IsNotFound(err) {
			return false, logicalDatabaseReasonProvisioning, "Referenced DatabaseServer is not available", nil
		}
		return false, "", "", fmt.Errorf("get DatabaseServer %s/%s: %w", logicalDatabase.Namespace, serverName, err)
	}

	adminIdentity, requeue, err := r.resolveAdminIdentity(ctx, logger, &db)
	if err != nil {
		return false, "", "", err
	}
	if requeue {
		return false, logicalDatabaseReasonProvisioning, "Waiting for DatabaseServer admin identity", nil
	}

	jobName := logicalDatabaseAccessProvisionJobName(logicalDatabase, serverName, adminIdentity)
	if err := r.ensureUserProvisionJobForTarget(ctx, logger, userProvisionJobSpec{
		Owner:               logicalDatabase,
		JobName:             jobName,
		Labels:              logicalDatabaseAccessJobLabels(serverName, logicalDatabase.Name),
		ServiceAccountName:  adminIdentity.ServiceAccountName,
		AdminIdentityName:   adminIdentity.Name,
		ServerName:          serverName,
		DatabaseHost:        logicalDatabase.Status.Host,
		DatabaseName:        logicalDatabase.Status.DatabaseName,
		SchemaName:          logicalDatabase.Status.DatabaseName,
		AppIdentityName:     logicalDatabase.Spec.Access.App.Name,
		AppPrincipalID:      logicalDatabase.Spec.Access.App.PrincipalId,
		OwnerIdentityName:   logicalDatabase.Spec.Access.Owner.Name,
		OwnerPrincipalID:    logicalDatabase.Spec.Access.Owner.PrincipalId,
		RevokePublicConnect: true,
		SearchPathScope:     "database",
	}); err != nil {
		return false, "", "", err
	}

	complete, err := r.logicalDatabaseAccessJobComplete(ctx, logicalDatabase, jobName)
	if err != nil {
		return false, "", "", err
	}
	if complete {
		return true, logicalDatabaseReasonReady, "Logical database access is ready", nil
	}

	return false, logicalDatabaseReasonProvisioning, "Logical database access provisioning job is running", nil
}

func (r *LogicalDatabaseReconciler) logicalDatabaseAccessJobComplete(
	ctx context.Context,
	logicalDatabase *storagev1alpha1.LogicalDatabase,
	jobName string,
) (bool, error) {
	var job batchv1.Job
	if err := r.Get(ctx, types.NamespacedName{
		Name:      jobName,
		Namespace: logicalDatabase.Namespace,
	}, &job); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get LogicalDatabase access Job %s/%s: %w", logicalDatabase.Namespace, jobName, err)
	}

	return jobConditionTrue(&job, batchv1.JobComplete), nil
}

func logicalDatabaseAccessProvisionJobName(
	logicalDatabase *storagev1alpha1.LogicalDatabase,
	serverName string,
	adminIdentity resolvedAdminIdentity,
) string {
	payload := strings.Join([]string{
		"server=" + serverName,
		"database=" + logicalDatabase.Status.DatabaseName,
		"host=" + logicalDatabase.Status.Host,
		"adminSA=" + adminIdentity.ServiceAccountName,
		"admin=" + adminIdentity.Name,
		"app=" + logicalDatabase.Spec.Access.App.Name,
		"appID=" + logicalDatabase.Spec.Access.App.PrincipalId,
		"owner=" + logicalDatabase.Spec.Access.Owner.Name,
		"ownerID=" + logicalDatabase.Spec.Access.Owner.PrincipalId,
		"schema=" + logicalDatabase.Status.DatabaseName,
		"revokePublicConnect=true",
		"searchPathScope=database",
	}, ";")
	hash := naming.StableSHA256Hex(payload)[:8]
	base := fmt.Sprintf("%s-access-provision", logicalDatabase.Name)
	return naming.WithRequiredSuffix(base, "-"+hash, 63, "ldb")
}

func logicalDatabaseAccessJobLabels(serverName, logicalDatabaseName string) map[string]string {
	return map[string]string{
		databaseNameLabelKey:                        serverName,
		logicalDatabaseLabelKey:                     logicalDatabaseName,
		"dis.altinn.cloud/access-provision":         "true",
		"dis.altinn.cloud/logical-access-provision": "true",
	}
}
