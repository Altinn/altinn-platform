package controller

import (
	"context"
	"fmt"
	"strings"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/connection"
	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *DatabaseReconciler) ensureDatabaseAccess(
	ctx context.Context,
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

	adminIdentity, requeue, err := r.resolveAdminIdentity(ctx, &db)
	if err != nil {
		return false, "", "", err
	}
	if requeue {
		return false, databaseReasonProvisioning, "Waiting for DatabaseServer admin identity", nil
	}

	accessPrincipals, serviceConnections, requeue, message, err := r.resolveDatabaseAccessPrincipals(ctx, database)
	if err != nil {
		return false, "", "", err
	}
	if requeue {
		return false, databaseReasonProvisioning, message, nil
	}

	var asoDatabase dbforpostgresqlv1.FlexibleServersDatabase
	asoDatabaseName := databaseASOResourceName(serverName, database.Status.DatabaseName)
	if err := r.Get(ctx, types.NamespacedName{
		Name:      asoDatabaseName,
		Namespace: database.Namespace,
	}, &asoDatabase); err != nil {
		if apierrors.IsNotFound(err) {
			return false, databaseReasonProvisioning, "Waiting for the database", nil
		}
		return false, "", "", fmt.Errorf("get FlexibleServersDatabase %s/%s: %w", database.Namespace, asoDatabaseName, err)
	}

	wantedHash := databaseAccessHash(database, serverName, db.UID, asoDatabase.UID, adminIdentity, accessPrincipals)
	if database.Status.ProvisionedAccessHash == wantedHash {
		// The wanted access is in place. A Job is not necessary.
		// The finished Job can be gone, because its TTL deletes it.
		if err := r.publishDatabaseConnections(ctx, database, serviceConnections); err != nil {
			return false, "", "", err
		}
		return true, databaseReasonReady, "Database access is ready", nil
	}
	if database.Status.ProvisionedAccessHash != "" {
		// The wanted access changed. Clear the record before a new Job starts.
		// A Job thus runs only while the record is empty.
		// The next reconcile creates the Job.
		database.Status.ProvisionedAccessHash = ""
		return false, databaseReasonProvisioning, "Database access changed and is provisioned again", nil
	}

	jobName := databaseAccessProvisionJobName(database, wantedHash)
	if err := r.ensureUserProvisionJobForTarget(ctx, userProvisionJobSpec{
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
		if err := r.publishDatabaseConnections(ctx, database, serviceConnections); err != nil {
			return false, "", "", err
		}
		database.Status.ProvisionedAccessHash = wantedHash
		return true, databaseReasonReady, "Database access is ready", nil
	}

	return false, databaseReasonProvisioning, "Database access provisioning job is running", nil
}

// publishDatabaseConnections writes one connection ConfigMap for each service
// connection of the Database.
func (r *DatabaseReconciler) publishDatabaseConnections(
	ctx context.Context,
	database *storagev1alpha1.Database,
	serviceConnections []resolvedServiceConnection,
) error {
	coords := make([]connection.Coordinates, 0, len(serviceConnections))
	for _, sc := range serviceConnections {
		coords = append(coords, connection.Coordinates{
			Host:        database.Status.Host,
			Port:        database.Status.Port,
			DBName:      database.Status.DatabaseName,
			User:        sc.ManagedIdentityName,
			IdentityRef: sc.IdentityRef,
		})
	}
	return r.reconcileConnectionConfigMaps(ctx, database, coords)
}

// resolvedServiceConnection pairs a service principal's spec identityRef.name
// (known at authoring time; drives the connection ConfigMap name and principal
// label) with the resolved managed-identity name the app authenticates as.
type resolvedServiceConnection struct {
	IdentityRef         string
	ManagedIdentityName string
}

func (r *DatabaseReconciler) resolveDatabaseAccessPrincipals(
	ctx context.Context,
	database *storagev1alpha1.Database,
) ([]dbUtil.AccessPrincipal, []resolvedServiceConnection, bool, string, error) {
	logger := log.FromContext(ctx)
	accessPrincipals := make([]dbUtil.AccessPrincipal, 0, len(database.Spec.Access.Principals))
	serviceConnections := make([]resolvedServiceConnection, 0, len(database.Spec.Access.Principals))
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

		if principal.ServicePrincipal != nil {
			accessPrincipal := dbUtil.AccessPrincipal{
				Role:          role,
				Name:          strings.TrimSpace(principal.ServicePrincipal.Name),
				PrincipalID:   strings.TrimSpace(principal.ServicePrincipal.PrincipalId),
				PrincipalType: dbUtil.PrincipalTypeService,
			}
			key := string(accessPrincipal.PrincipalType) + ":" + strings.ToLower(accessPrincipal.PrincipalID)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			// No connection ConfigMap for servicePrincipal principals (there is
			// no ApplicationIdentity to wire a consumer to): not appended to
			// serviceConnections.
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
				return nil, nil, true, fmt.Sprintf("Waiting for ApplicationIdentity %q", refName), nil
			}
			return nil, nil, false, "", fmt.Errorf("get ApplicationIdentity %s/%s: %w", database.Namespace, refName, err)
		}

		ready, readyFound := applicationIdentityReady(&appIdentity)
		if readyFound && !ready {
			logger.Info("ApplicationIdentity for Database access not ready yet", "name", refName)
			return nil, nil, true, fmt.Sprintf("Waiting for ApplicationIdentity %q to be ready", refName), nil
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
			return nil, nil, true, fmt.Sprintf("Waiting for ApplicationIdentity %q status", refName), nil
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
		serviceConnections = append(serviceConnections, resolvedServiceConnection{
			IdentityRef:         refName,
			ManagedIdentityName: managedIdentityName,
		})
	}

	if len(accessPrincipals) == 0 {
		return nil, nil, false, "", fmt.Errorf("database access principal resolution produced no principals")
	}
	return accessPrincipals, serviceConnections, false, "", nil
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

// databaseAccessHash returns the hash of the wanted access of a Database.
// The hash changes when an input of the provisioning Job changes.
// The hash also changes when the DatabaseServer resource or the database
// resource is created again, because a new resource has a new UID.
// A change to this payload makes the operator provision the access of all
// Databases again.
func databaseAccessHash(
	database *storagev1alpha1.Database,
	serverName string,
	serverUID types.UID,
	asoDatabaseUID types.UID,
	adminIdentity resolvedAdminIdentity,
	accessPrincipals []dbUtil.AccessPrincipal,
) string {
	accessPayload, err := dbUtil.MarshalAccessPrincipals(accessPrincipals)
	if err != nil {
		accessPayload = err.Error()
	}
	payload := strings.Join([]string{
		"server=" + serverName,
		"serverUID=" + string(serverUID),
		"database=" + database.Status.DatabaseName,
		"databaseUID=" + string(asoDatabaseUID),
		"host=" + database.Status.Host,
		"adminSA=" + adminIdentity.ServiceAccountName,
		"admin=" + adminIdentity.Name,
		"access=" + accessPayload,
		"schema=" + database.Status.DatabaseName,
		"revokePublicConnect=true",
		"searchPathScope=database",
	}, ";")
	return naming.StableSHA256Hex(payload)
}

// databaseAccessProvisionJobName returns the Job name for an access hash.
// A complete Job with this name shows that the access with this hash is
// provisioned.
func databaseAccessProvisionJobName(database *storagev1alpha1.Database, accessHash string) string {
	base := fmt.Sprintf("%s-access-provision", database.Name)
	return naming.WithRequiredSuffix(base, "-"+accessHash[:8], 63, "ldb")
}

func databaseAccessJobLabels(serverName, databaseName string) map[string]string {
	return map[string]string{
		databaseServerNameLabelKey:                   serverName,
		databaseNameLabelKey:                         databaseName,
		databaseAccessProvisionLabelKey:              labelValueTrue,
		"dis.altinn.cloud/database-access-provision": labelValueTrue,
	}
}
