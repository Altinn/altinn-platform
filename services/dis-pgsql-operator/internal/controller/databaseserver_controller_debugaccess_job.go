package controller

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

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
// hash of the resolved principal set (and built-in roles) so adding or removing
// a principal produces a new Job that re-runs the reconcile (which also revokes
// principals no longer desired). Debug access is dedicated-only; the caller must
// not invoke it for shared servers. The DebugAccessReady condition remains
// control-plane-driven (set by ensureDebugAccessRoleAssignments) and is not
// touched here.
func (r *DatabaseServerReconciler) ensureDebugAccessProvisioning(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
	adminIdentity resolvedAdminIdentity,
) error {
	if db.Spec.DebugAccess == nil || len(db.Spec.DebugAccess.Principals) == 0 {
		// No debug access requested: nothing to provision. Any previously created
		// Job is short-lived (TTLSecondsAfterFinished) and self-reaps.
		return nil
	}

	// The provisioner connects to the real Flexible Server FQDN, published on the
	// status once ASO reports it. Under az fakes (Kind) the status host is never
	// populated and the provisioner falls back to the in-cluster Postgres, so the
	// host gate only applies to real Azure runs. Skipping here just defers to a
	// later reconcile (asoResourcesReady sets the host and requeues).
	if !r.Config.UseAzFakes && strings.TrimSpace(db.Status.Host) == "" {
		logger.Info("DatabaseServer status host not populated yet; deferring debug access provisioning")
		return nil
	}

	accessPrincipals, err := r.resolveDebugAccessDataPlanePrincipals(ctx, logger, db)
	if err != nil {
		return err
	}
	if len(accessPrincipals) == 0 {
		// Every principal is an identityRef that is not ready yet; skip until a
		// later reconcile (the ApplicationIdentity watch re-triggers us).
		logger.Info("no debug access principals ready yet; skipping data-plane provisioning")
		return nil
	}

	builtinRoles := dbUtil.NormalizeBuiltinRoles(strings.Join(debugAccessBuiltinRoles, ","))
	jobName := debugAccessProvisionJobName(db, adminIdentity, accessPrincipals, builtinRoles)

	return ensureUserProvisionJobForReconciler(ctx, logger, r, userProvisionJobSpec{
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
	})
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

// debugAccessProvisionJobName returns a deterministic Job name whose hash covers
// the admin identity, the resolved principal set, and the built-in roles, so any
// change to the desired debug access produces a new Job that re-runs the
// reconcile (adds/removes principals).
func debugAccessProvisionJobName(
	db *storagev1alpha1.DatabaseServer,
	adminIdentity resolvedAdminIdentity,
	accessPrincipals []dbUtil.AccessPrincipal,
	builtinRoles []string,
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
