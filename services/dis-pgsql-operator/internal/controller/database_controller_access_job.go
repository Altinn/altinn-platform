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

func (r *DatabaseReconciler) ensureDatabaseAccess(
	ctx context.Context,
	logger logr.Logger,
	database *storagev1alpha1.Database,
) (bool, string, string, error) {
	accessReady := meta.FindStatusCondition(database.Status.Conditions, databaseConditionAccessReady)
	if accessReady != nil &&
		accessReady.Status == metav1.ConditionTrue &&
		accessReady.ObservedGeneration == database.Generation {
		return true, databaseReasonReady, "Database access is ready", nil
	}

	serverName := strings.TrimSpace(database.Spec.Server.Name)
	var db storagev1alpha1.DatabaseServer
	if err := r.Get(ctx, types.NamespacedName{
		Name:      serverName,
		Namespace: database.Namespace,
	}, &db); err != nil {
		if apierrors.IsNotFound(err) {
			return false, databaseReasonProvisioning, "Referenced DatabaseServer is not available", nil
		}
		return false, "", "", fmt.Errorf("get DatabaseServer %s/%s: %w", database.Namespace, serverName, err)
	}

	adminIdentity, requeue, err := r.resolveAdminIdentity(ctx, logger, &db)
	if err != nil {
		return false, "", "", err
	}
	if requeue {
		return false, databaseReasonProvisioning, "Waiting for DatabaseServer admin identity", nil
	}

	jobName := databaseAccessProvisionJobName(database, serverName, adminIdentity)
	if err := r.ensureUserProvisionJobForTarget(ctx, logger, userProvisionJobSpec{
		Owner:               database,
		JobName:             jobName,
		Labels:              databaseAccessJobLabels(serverName, database.Name),
		ServiceAccountName:  adminIdentity.ServiceAccountName,
		AdminIdentityName:   adminIdentity.Name,
		ServerName:          serverName,
		DatabaseHost:        database.Status.Host,
		DatabaseName:        database.Status.DatabaseName,
		SchemaName:          database.Status.DatabaseName,
		AppIdentityName:     database.Spec.Access.App.Name,
		AppPrincipalID:      database.Spec.Access.App.PrincipalId,
		OwnerIdentityName:   database.Spec.Access.Owner.Name,
		OwnerPrincipalID:    database.Spec.Access.Owner.PrincipalId,
		RevokePublicConnect: true,
		SearchPathScope:     "database",
	}); err != nil {
		return false, "", "", err
	}

	complete, err := r.databaseAccessJobComplete(ctx, database, jobName)
	if err != nil {
		return false, "", "", err
	}
	if complete {
		return true, databaseReasonReady, "Database access is ready", nil
	}

	return false, databaseReasonProvisioning, "Database access provisioning job is running", nil
}

func (r *DatabaseReconciler) databaseAccessJobComplete(
	ctx context.Context,
	database *storagev1alpha1.Database,
	jobName string,
) (bool, error) {
	var job batchv1.Job
	if err := r.Get(ctx, types.NamespacedName{
		Name:      jobName,
		Namespace: database.Namespace,
	}, &job); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get Database access Job %s/%s: %w", database.Namespace, jobName, err)
	}

	return jobConditionTrue(&job, batchv1.JobComplete), nil
}

func databaseAccessProvisionJobName(
	database *storagev1alpha1.Database,
	serverName string,
	adminIdentity resolvedAdminIdentity,
) string {
	payload := strings.Join([]string{
		"server=" + serverName,
		"database=" + database.Status.DatabaseName,
		"host=" + database.Status.Host,
		"adminSA=" + adminIdentity.ServiceAccountName,
		"admin=" + adminIdentity.Name,
		"app=" + database.Spec.Access.App.Name,
		"appID=" + database.Spec.Access.App.PrincipalId,
		"owner=" + database.Spec.Access.Owner.Name,
		"ownerID=" + database.Spec.Access.Owner.PrincipalId,
		"schema=" + database.Status.DatabaseName,
		"revokePublicConnect=true",
		"searchPathScope=database",
	}, ";")
	hash := naming.StableSHA256Hex(payload)[:8]
	base := fmt.Sprintf("%s-access-provision", database.Name)
	return naming.WithRequiredSuffix(base, "-"+hash, 63, "ldb")
}

func databaseAccessJobLabels(serverName, databaseName string) map[string]string {
	return map[string]string{
		databaseNameLabelKey:                        serverName,
		databaseLabelKey:                            databaseName,
		"dis.altinn.cloud/access-provision":         "true",
		"dis.altinn.cloud/logical-access-provision": "true",
	}
}
