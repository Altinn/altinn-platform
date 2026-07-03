package controller

import (
	"context"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
)

const (
	// debugBuiltinRolePgMonitor grants full monitoring visibility (pg_stat_*,
	// pg_ls_*, etc.). It leaks statistics across databases on a server, which is
	// why debug access is dedicated-only.
	debugBuiltinRolePgMonitor = "pg_monitor"
	// debugBuiltinRolePgReadAllData grants read-only SELECT across all tables.
	debugBuiltinRolePgReadAllData = "pg_read_all_data"
)

// debugAccessBuiltinRoles are the built-in PostgreSQL roles granted to the
// managed debug role.
var debugAccessBuiltinRoles = []string{debugBuiltinRolePgMonitor, debugBuiltinRolePgReadAllData}

// debugAccessProvisionMaintenanceDatabase is the database the debug provisioning
// Job connects to. The pgaadauth_* helpers and GRANT CONNECT on every database
// are only available from the maintenance database, not an app database.
const debugAccessProvisionMaintenanceDatabase = "postgres"

// ensureDebugAccessProvisioning grants read-only PostgreSQL debug access on a
// dedicated server: it runs a user-provisioning Job in server-debug mode that
// ensures a managed NOLOGIN role holding pg_monitor + pg_read_all_data and
// CONNECT on all databases, and makes each resolved debug principal a member.
//
// It mirrors ensureDatabaseAccess but at server scope. The Job name embeds a
// hash of the resolved principal set, the built-in roles, and the server's
// Database resources, so changing any of them produces a new Job that re-runs
// the reconcile (which also revokes principals no longer desired and grants
// CONNECT on new databases). Debug access is dedicated-only; the caller must
// not invoke it for shared servers. The DebugAccessReady condition remains
// control-plane-driven (set by ensureDebugAccessRoleAssignments) and is not
// touched here.
//
// status.debugAccessProvisionedHash records that grants may exist in the
// server. When spec.debugAccess is removed after having been provisioned, one
// revocation Job runs with an empty principal set (the membership reconcile
// then revokes every member) and the marker is cleared once that Job
// completes. The managed debug role itself is kept: it is inert without
// members.
func (r *DatabaseServerReconciler) ensureDebugAccessProvisioning(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
	adminIdentity resolvedAdminIdentity,
) error {
	// The provisioner connects to the real Flexible Server FQDN, published on the
	// status once ASO reports it. Under az fakes (Kind) the status host is never
	// populated and the provisioner falls back to the in-cluster Postgres, so the
	// host gate only applies to real Azure runs. Skipping here just defers to a
	// later reconcile (asoResourcesReady sets the host and requeues).
	if !r.Config.UseAzFakes && strings.TrimSpace(db.Status.Host) == "" {
		logger.Info("DatabaseServer status host not populated yet; deferring debug access provisioning")
		return nil
	}

	if db.Spec.DebugAccess == nil || len(db.Spec.DebugAccess.Principals) == 0 {
		if db.Status.DebugAccessProvisionedHash == "" {
			// Never provisioned: nothing to revoke.
			return nil
		}
		return r.ensureDebugAccessRevocation(ctx, logger, db, adminIdentity)
	}

	accessPrincipals, err := r.resolveDebugAccessDataPlanePrincipals(ctx, logger, db)
	if err != nil {
		return err
	}
	if len(accessPrincipals) == 0 {
		// Every principal is an identityRef that is not ready yet; skip until a
		// later reconcile (the ApplicationIdentity watch re-triggers us). This is
		// not a removal, so no revocation runs and the provisioned marker stays.
		logger.Info("no debug access principals ready yet; skipping data-plane provisioning")
		return nil
	}

	databaseNames, err := r.serverDatabaseNames(ctx, db)
	if err != nil {
		return err
	}

	builtinRoles := dbUtil.NormalizeBuiltinRoles(strings.Join(debugAccessBuiltinRoles, ","))
	jobName := debugAccessProvisionJobName(db, adminIdentity, accessPrincipals, builtinRoles, databaseNames)

	if err := ensureUserProvisionJobForReconciler(ctx, logger, r, userProvisionJobSpec{
		Owner:              db,
		JobName:            jobName,
		Labels:             debugAccessProvisionJobLabels(db.Name),
		ServiceAccountName: adminIdentity.ServiceAccountName,
		AdminIdentityName:  adminIdentity.Name,
		ServerName:         db.Name,
		DatabaseHost:       db.Status.Host,
		DatabaseName:       debugAccessProvisionMaintenanceDatabase,
		AccessPrincipals:   accessPrincipals,
		ServerDebugAccess:  true,
		DebugBuiltinRoles:  builtinRoles,
	}); err != nil {
		return err
	}

	return r.persistDebugAccessProvisionedHash(ctx, db, debugAccessPrincipalsHash(accessPrincipals))
}

// ensureDebugAccessRevocation runs the server-debug Job with an empty principal
// set so the membership reconcile revokes every member of the managed debug
// role, and clears status.debugAccessProvisionedHash once that Job completes.
// The Owns(batchv1.Job) watch re-triggers the reconcile on Job completion.
func (r *DatabaseServerReconciler) ensureDebugAccessRevocation(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
	adminIdentity resolvedAdminIdentity,
) error {
	builtinRoles := dbUtil.NormalizeBuiltinRoles(strings.Join(debugAccessBuiltinRoles, ","))
	jobName := debugAccessProvisionJobName(db, adminIdentity, nil, builtinRoles, nil)

	var job batchv1.Job
	err := r.Get(ctx, types.NamespacedName{Namespace: db.Namespace, Name: jobName}, &job)
	switch {
	case err == nil && jobConditionTrue(&job, batchv1.JobComplete):
		logger.Info("debug access revocation Job completed; clearing provisioned marker", "jobName", jobName)
		return r.persistDebugAccessProvisionedHash(ctx, db, "")
	case err != nil && !apierrors.IsNotFound(err):
		return err
	}

	return ensureUserProvisionJobForReconciler(ctx, logger, r, userProvisionJobSpec{
		Owner:              db,
		JobName:            jobName,
		Labels:             debugAccessProvisionJobLabels(db.Name),
		ServiceAccountName: adminIdentity.ServiceAccountName,
		AdminIdentityName:  adminIdentity.Name,
		ServerName:         db.Name,
		DatabaseHost:       db.Status.Host,
		DatabaseName:       debugAccessProvisionMaintenanceDatabase,
		AccessPrincipals:   nil,
		ServerDebugAccess:  true,
		DebugBuiltinRoles:  builtinRoles,
	})
}

// persistDebugAccessProvisionedHash updates status.debugAccessProvisionedHash
// when it changed.
func (r *DatabaseServerReconciler) persistDebugAccessProvisionedHash(
	ctx context.Context,
	db *storagev1alpha1.DatabaseServer,
	hash string,
) error {
	if db.Status.DebugAccessProvisionedHash == hash {
		return nil
	}
	db.Status.DebugAccessProvisionedHash = hash
	return r.Status().Update(ctx, db)
}

// serverDatabaseNames returns the sorted PostgreSQL database names of the
// Database resources that target this server, so the provisioning Job re-runs
// (and grants CONNECT) when a database is added or removed.
func (r *DatabaseServerReconciler) serverDatabaseNames(
	ctx context.Context,
	db *storagev1alpha1.DatabaseServer,
) ([]string, error) {
	var databases storagev1alpha1.DatabaseList
	if err := r.List(ctx, &databases, client.InNamespace(db.Namespace)); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(databases.Items))
	for i := range databases.Items {
		item := databases.Items[i]
		if item.Spec.Server.Name != db.Name {
			continue
		}
		name := strings.TrimSpace(item.Spec.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// The DatabaseServerReconciler implements userProvisionJobReconciler so the
// shared Job machinery (ensureUserProvisionJobForReconciler) can create the
// server-debug provisioning Job, exactly as DatabaseReconciler does for the
// per-database access Job.
func (r *DatabaseServerReconciler) userProvisionJobScheme() *runtime.Scheme {
	return r.Scheme
}

func (r *DatabaseServerReconciler) userProvisionJobImage() string {
	return r.Config.UserProvisionImage
}

func (r *DatabaseServerReconciler) userProvisionJobUseAzFakes() bool {
	return r.Config.UseAzFakes
}

// resolveDebugAccessDataPlanePrincipals resolves each debug principal to the
// PostgreSQL principal fields the provisioner needs. It reuses the control-plane
// resolveDebugAccessPrincipal and skips not-ready identityRefs (and any resolved
// principal missing a name) without failing the reconcile, mirroring how
// ensureDebugAccessRoleAssignments tolerates pending identities. Duplicates
// (by principal id) are collapsed.
func (r *DatabaseServerReconciler) resolveDebugAccessDataPlanePrincipals(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
) ([]dbUtil.AccessPrincipal, error) {
	accessPrincipals := make([]dbUtil.AccessPrincipal, 0, len(db.Spec.DebugAccess.Principals))
	seen := map[string]struct{}{}

	for _, principal := range db.Spec.DebugAccess.Principals {
		resolved, ok, err := r.resolveDebugAccessPrincipal(ctx, logger, db, principal)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		name := strings.TrimSpace(resolved.Name)
		if name == "" {
			// A resolved principal without a PostgreSQL name cannot be provisioned
			// (an identityRef whose managedIdentityName is not populated yet). Treat
			// it as not-ready and skip until a later reconcile.
			logger.Info("debug access principal has no PostgreSQL name yet; skipping", "principalId", resolved.PrincipalID)
			continue
		}

		principalType := debugAccessPrincipalTypeToPayload(resolved.PrincipalType)
		key := string(principalType) + ":" + strings.ToLower(resolved.PrincipalID)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}

		accessPrincipals = append(accessPrincipals, dbUtil.AccessPrincipal{
			Name:          name,
			PrincipalID:   strings.TrimSpace(resolved.PrincipalID),
			PrincipalType: principalType,
		})
	}

	return accessPrincipals, nil
}

// debugAccessPrincipalTypeToPayload maps the Azure role-assignment principal type
// enum used by the control plane to the payload principal type the provisioner
// understands. Groups map to "group"; everything else (service principals,
// managed identities) maps to "service".
func debugAccessPrincipalTypeToPayload(
	principalType authorizationv1.RoleAssignmentProperties_PrincipalType,
) dbUtil.PrincipalType {
	if principalType == authorizationv1.RoleAssignmentProperties_PrincipalType_Group {
		return dbUtil.PrincipalTypeGroup
	}
	return dbUtil.PrincipalTypeService
}

// debugAccessPrincipalsHash is the opaque status marker for a provisioned
// principal set.
func debugAccessPrincipalsHash(accessPrincipals []dbUtil.AccessPrincipal) string {
	accessPayload, err := dbUtil.MarshalAccessPrincipals(accessPrincipals)
	if err != nil {
		accessPayload = err.Error()
	}
	return naming.StableSHA256Hex(accessPayload)[:12]
}

// debugAccessProvisionJobName returns a deterministic Job name whose hash covers
// the admin identity, the resolved principal set, the built-in roles, and the
// server's databases, so any change to the desired debug access produces a new
// Job that re-runs the reconcile (adds/removes principals, grants CONNECT on
// new databases).
func debugAccessProvisionJobName(
	db *storagev1alpha1.DatabaseServer,
	adminIdentity resolvedAdminIdentity,
	accessPrincipals []dbUtil.AccessPrincipal,
	builtinRoles []string,
	databaseNames []string,
) string {
	accessPayload, err := dbUtil.MarshalAccessPrincipals(accessPrincipals)
	if err != nil {
		accessPayload = err.Error()
	}
	payload := strings.Join([]string{
		"server=" + db.Name,
		"host=" + db.Status.Host,
		"adminSA=" + adminIdentity.ServiceAccountName,
		"admin=" + adminIdentity.Name,
		"access=" + accessPayload,
		"builtin=" + strings.Join(builtinRoles, ","),
		"databases=" + strings.Join(databaseNames, ","),
		"mode=server-debug",
	}, ";")
	hash := naming.StableSHA256Hex(payload)[:8]
	base := naming.EnsureLowerAlphaPrefix(naming.SanitizeLowerHyphen(db.Name), "db")
	return naming.WithRequiredSuffix(base+"-debug-provision", "-"+hash, 63, "db")
}

func debugAccessProvisionJobLabels(serverName string) map[string]string {
	return map[string]string{
		databaseServerNameLabelKey:      serverName,
		databaseAccessProvisionLabelKey: labelValueTrue,
		debugAccessComponentLabelKey:    debugAccessComponentLabelValue,
	}
}
