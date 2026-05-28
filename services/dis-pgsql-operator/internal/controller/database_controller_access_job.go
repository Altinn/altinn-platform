package controller

import (
	"context"
	"fmt"
	"strings"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (r *DatabaseReconciler) ensureDatabaseAccess(
	ctx context.Context,
	logger logr.Logger,
	database *storagev1alpha1.Database,
) (bool, string, string, error) {
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

	accessPrincipals, requeue, message, err := r.resolveDatabaseAccessPrincipals(ctx, logger, database)
	if err != nil {
		return false, "", "", err
	}
	if requeue {
		return false, databaseReasonProvisioning, message, nil
	}

	jobName := databaseAccessProvisionJobName(database, serverName, adminIdentity, accessPrincipals)
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
		AccessPrincipals:    accessPrincipals,
		RevokePublicConnect: true,
		SearchPathScope:     searchPathScopeDatabase,
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

func (r *DatabaseReconciler) resolveDatabaseAccessPrincipals(
	ctx context.Context,
	logger logr.Logger,
	database *storagev1alpha1.Database,
) ([]dbUtil.AccessPrincipal, bool, string, error) {
	accessPrincipals := make([]dbUtil.AccessPrincipal, 0, len(database.Spec.Access.Principals))
	seen := map[string]struct{}{}

	for _, principal := range database.Spec.Access.Principals {
		role := databaseAccessPayloadRole(principal.Role)
		if principal.Group != nil {
			accessPrincipal := dbUtil.AccessPrincipal{
				Role:          role,
				Name:          strings.TrimSpace(principal.Group.Name),
				PrincipalID:   strings.TrimSpace(principal.Group.PrincipalId),
				PrincipalType: dbUtil.PrincipalTypeGroup,
			}
			key := string(accessPrincipal.PrincipalType) + ":" + strings.ToLower(accessPrincipal.PrincipalID)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			accessPrincipals = append(accessPrincipals, accessPrincipal)
			continue
		}

		if principal.IdentityRef == nil {
			continue
		}

		refName := strings.TrimSpace(principal.IdentityRef.Name)
		var appIdentity identityv1alpha1.ApplicationIdentity
		if err := r.Get(ctx, types.NamespacedName{Name: refName, Namespace: database.Namespace}, &appIdentity); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("ApplicationIdentity for Database access not found yet", "name", refName)
				return nil, true, fmt.Sprintf("Waiting for ApplicationIdentity %q", refName), nil
			}
			return nil, false, "", fmt.Errorf("get ApplicationIdentity %s/%s: %w", database.Namespace, refName, err)
		}

		ready, readyFound := applicationIdentityReady(&appIdentity)
		if readyFound && !ready {
			logger.Info("ApplicationIdentity for Database access not ready yet", "name", refName)
			return nil, true, fmt.Sprintf("Waiting for ApplicationIdentity %q to be ready", refName), nil
		}

		var managedIdentityName string
		if appIdentity.Status.ManagedIdentityName != nil {
			managedIdentityName = strings.TrimSpace(*appIdentity.Status.ManagedIdentityName)
		}
		var principalID string
		if appIdentity.Status.PrincipalID != nil {
			principalID = strings.TrimSpace(*appIdentity.Status.PrincipalID)
		}
		if managedIdentityName == "" || principalID == "" {
			logger.Info("ApplicationIdentity for Database access status not populated yet", "name", refName)
			return nil, true, fmt.Sprintf("Waiting for ApplicationIdentity %q status", refName), nil
		}

		accessPrincipal := dbUtil.AccessPrincipal{
			Role:          role,
			Name:          managedIdentityName,
			PrincipalID:   principalID,
			PrincipalType: dbUtil.PrincipalTypeService,
		}
		key := string(accessPrincipal.PrincipalType) + ":" + strings.ToLower(accessPrincipal.PrincipalID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		accessPrincipals = append(accessPrincipals, accessPrincipal)
	}

	if len(accessPrincipals) == 0 {
		return nil, false, "", fmt.Errorf("database access principal resolution produced no principals")
	}
	return accessPrincipals, false, "", nil
}

func databaseAccessPayloadRole(role storagev1alpha1.DatabaseAccessRole) dbUtil.AccessRole {
	switch role {
	case storagev1alpha1.DatabaseAccessRoleReader:
		return dbUtil.AccessRoleReader
	case storagev1alpha1.DatabaseAccessRoleWriter:
		return dbUtil.AccessRoleWriter
	case storagev1alpha1.DatabaseAccessRoleOwner:
		return dbUtil.AccessRoleOwner
	default:
		return dbUtil.AccessRole(role)
	}
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
	accessPrincipals []dbUtil.AccessPrincipal,
) string {
	accessPayload, err := dbUtil.MarshalAccessPrincipals(accessPrincipals)
	if err != nil {
		accessPayload = err.Error()
	}
	payload := strings.Join([]string{
		"server=" + serverName,
		"database=" + database.Status.DatabaseName,
		"host=" + database.Status.Host,
		"adminSA=" + adminIdentity.ServiceAccountName,
		"admin=" + adminIdentity.Name,
		"access=" + accessPayload,
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
		databaseServerNameLabelKey:                   serverName,
		databaseNameLabelKey:                         databaseName,
		databaseAccessProvisionLabelKey:              labelValueTrue,
		"dis.altinn.cloud/database-access-provision": labelValueTrue,
	}
}
