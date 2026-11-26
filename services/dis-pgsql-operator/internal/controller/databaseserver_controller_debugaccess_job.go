package controller

import (
	"context"
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
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
// dedicated server. A user provisioning Job in server-debug mode does the
// work. The Job makes sure that a managed NOLOGIN role has pg_monitor,
// pg_read_all_data and CONNECT on all databases. The Job also makes each
// resolved debug principal a member of that role.
//
// Debug access is for dedicated servers only. The caller must not call this
// function for shared servers. The DebugAccessReady condition belongs to
// ensureDebugAccessRoleAssignments. This function does not change it.
//
// Two status fields control the Job:
//
//   - status.debugAccessAppliedHash is the hash of the debug access that the
//     last completed Job applied. The operator creates no Job while this hash
//     is equal to the wanted hash. A finished Job that its TTL deleted thus
//     does not cause a new Job.
//   - status.debugAccessProvisionedHash records that grants can exist in the
//     DIS DB server. The operator sets it before a Job starts. When
//     spec.debugAccess is removed, one revocation Job runs with no principals
//     and the operator clears the field when that Job completes. The managed
//     debug role stays. It has no effect without members.
func (r *DatabaseServerReconciler) ensureDebugAccessProvisioning(
	ctx context.Context,
	db *storagev1alpha1.DatabaseServer,
	adminIdentity resolvedAdminIdentity,
) error {
	logger := log.FromContext(ctx)
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
		return r.ensureDebugAccessRevocation(ctx, db, adminIdentity)
	}

	accessPrincipals, err := r.resolveDebugAccessDataPlanePrincipals(ctx, db)
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

	var flexibleServer dbforpostgresqlv1.FlexibleServer
	if err := r.Get(ctx, client.ObjectKeyFromObject(db), &flexibleServer); err != nil {
		// A later reconcile continues when the FlexibleServer exists.
		return client.IgnoreNotFound(err)
	}
	databases, err := r.serverReadyDatabases(ctx, db)
	if err != nil {
		return err
	}

	builtinRoles := dbUtil.NormalizeBuiltinRoles(strings.Join(debugAccessBuiltinRoles, ","))
	wantedHash := debugAccessHash(db, flexibleServer.UID, adminIdentity, accessPrincipals, builtinRoles, databases)
	if db.Status.DebugAccessAppliedHash == wantedHash {
		// The wanted debug access is in place. A Job is not necessary.
		// The finished Job can be gone, because its TTL deletes it.
		return nil
	}
	if db.Status.DebugAccessAppliedHash != "" {
		// The wanted debug access changed. Clear the record before a new Job
		// starts. A Job thus runs only while the record is empty.
		// The status update triggers the reconcile that creates the Job.
		logger.Info("debug access changed; clearing the applied record")
		return r.persistDebugAccessHashes(ctx, db, db.Status.DebugAccessProvisionedHash, "")
	}

	// Record that grants can exist before the Job starts.
	if err := r.persistDebugAccessHashes(ctx, db, wantedHash, ""); err != nil {
		return err
	}

	jobName := debugAccessProvisionJobName(db, wantedHash)
	complete, err := ensureUserProvisionJobForReconciler(
		ctx, r, debugAccessJobSpec(db, adminIdentity, jobName, accessPrincipals, builtinRoles),
	)
	if err != nil || !complete {
		return err
	}
	logger.Info("debug access provisioning Job completed", "jobName", jobName)
	return r.persistDebugAccessHashes(ctx, db, wantedHash, wantedHash)
}

// ensureDebugAccessRevocation runs the server-debug Job with no principals.
// The membership reconcile of that Job revokes every member of the managed
// debug role. When the Job completes, this function clears
// status.debugAccessProvisionedHash. The Owns(batchv1.Job) watch triggers a
// reconcile when the Job completes.
func (r *DatabaseServerReconciler) ensureDebugAccessRevocation(
	ctx context.Context,
	db *storagev1alpha1.DatabaseServer,
	adminIdentity resolvedAdminIdentity,
) error {
	logger := log.FromContext(ctx)
	if db.Status.DebugAccessAppliedHash != "" {
		// The revocation Job removes the applied debug access. Clear the record
		// before that Job starts. The status update triggers the reconcile that
		// creates the Job.
		return r.persistDebugAccessHashes(ctx, db, db.Status.DebugAccessProvisionedHash, "")
	}

	builtinRoles := dbUtil.NormalizeBuiltinRoles(strings.Join(debugAccessBuiltinRoles, ","))
	// The hash gives the name of the revocation Job only. The targets are not
	// necessary for that.
	jobName := debugAccessProvisionJobName(db, debugAccessHash(db, "", adminIdentity, nil, builtinRoles, nil))

	complete, err := ensureUserProvisionJobForReconciler(
		ctx, r, debugAccessJobSpec(db, adminIdentity, jobName, nil, builtinRoles),
	)
	if err != nil || !complete {
		return err
	}
	logger.Info("debug access revocation Job completed; clearing provisioned marker", "jobName", jobName)
	return r.persistDebugAccessHashes(ctx, db, "", "")
}

// debugAccessJobSpec returns the spec of the server-debug Job. A nil principal
// list gives the revocation Job.
func debugAccessJobSpec(
	db *storagev1alpha1.DatabaseServer,
	adminIdentity resolvedAdminIdentity,
	jobName string,
	accessPrincipals []dbUtil.AccessPrincipal,
	builtinRoles []string,
) userProvisionJobSpec {
	return userProvisionJobSpec{
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
	}
}

// persistDebugAccessHashes updates status.debugAccessProvisionedHash and
// status.debugAccessAppliedHash when one of them changed.
func (r *DatabaseServerReconciler) persistDebugAccessHashes(
	ctx context.Context,
	db *storagev1alpha1.DatabaseServer,
	provisionedHash string,
	appliedHash string,
) error {
	if db.Status.DebugAccessProvisionedHash == provisionedHash && db.Status.DebugAccessAppliedHash == appliedHash {
		return nil
	}
	db.Status.DebugAccessProvisionedHash = provisionedHash
	db.Status.DebugAccessAppliedHash = appliedHash
	return r.Status().Update(ctx, db)
}

// serverReadyDatabases returns one sorted entry for each Database resource of
// this server whose database exists on the DIS DB server. The provisioning Job
// grants CONNECT only on databases that exist when it runs. A Database thus
// counts only when its DatabaseReady condition is true, and the Job runs
// again after a new database is ready. Each entry has the database name and
// the UID of the database resource, because a database that is created again
// has no grants.
func (r *DatabaseServerReconciler) serverReadyDatabases(
	ctx context.Context,
	db *storagev1alpha1.DatabaseServer,
) ([]string, error) {
	var databases storagev1alpha1.DatabaseList
	if err := r.List(ctx, &databases, client.InNamespace(db.Namespace)); err != nil {
		return nil, err
	}

	entries := make([]string, 0, len(databases.Items))
	for i := range databases.Items {
		item := &databases.Items[i]
		if item.Spec.Server.Name != db.Name {
			continue
		}
		name := item.Status.DatabaseName
		if name == "" || !meta.IsStatusConditionTrue(item.Status.Conditions, databaseConditionDatabaseReady) {
			continue
		}
		var asoDatabase dbforpostgresqlv1.FlexibleServersDatabase
		if err := r.Get(ctx, types.NamespacedName{
			Name:      databaseASOResourceName(db.Name, name),
			Namespace: db.Namespace,
		}, &asoDatabase); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		// The condition of the Database can be old. A database resource that
		// is created again is not ready, and its database does not exist yet.
		if status, _, _, ok := readyConditionInfo(asoDatabase.Status.Conditions); !ok || status != metav1.ConditionTrue {
			continue
		}
		entries = append(entries, name+"/"+string(asoDatabase.UID))
	}
	slices.Sort(entries)
	return entries, nil
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
	db *storagev1alpha1.DatabaseServer,
) ([]dbUtil.AccessPrincipal, error) {
	logger := log.FromContext(ctx)
	accessPrincipals := make([]dbUtil.AccessPrincipal, 0, len(db.Spec.DebugAccess.Principals))
	seen := map[string]struct{}{}

	for _, principal := range db.Spec.DebugAccess.Principals {
		resolved, ok, err := r.resolveDebugAccessPrincipal(ctx, db, principal)
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

// debugAccessHash returns the hash of the wanted debug access of a server.
// The hash changes when an input of the provisioning Job changes. The hash
// also changes when the set of ready databases changes, because the Job must
// grant CONNECT on each new database. The hash also changes when the
// DatabaseServer or the FlexibleServer is created again. The image of the
// provisioning Job is not an input. A new image thus does not start a Job.
// A change to this payload makes the operator provision the debug access of
// all servers again.
func debugAccessHash(
	db *storagev1alpha1.DatabaseServer,
	flexibleServerUID types.UID,
	adminIdentity resolvedAdminIdentity,
	accessPrincipals []dbUtil.AccessPrincipal,
	builtinRoles []string,
	databases []string,
) string {
	accessPayload, err := dbUtil.MarshalAccessPrincipals(accessPrincipals)
	if err != nil {
		accessPayload = err.Error()
	}
	payload := strings.Join([]string{
		"server=" + db.Name,
		"serverUID=" + string(db.UID),
		"flexibleServerUID=" + string(flexibleServerUID),
		"host=" + db.Status.Host,
		"adminSA=" + adminIdentity.ServiceAccountName,
		"admin=" + adminIdentity.Name,
		"access=" + accessPayload,
		"builtin=" + strings.Join(builtinRoles, ","),
		"databases=" + strings.Join(databases, ","),
		"mode=server-debug",
	}, ";")
	return naming.StableSHA256Hex(payload)
}

// debugAccessProvisionJobName returns the Job name for a debug access hash.
// A complete Job with this name shows that the debug access with this hash
// is provisioned.
func debugAccessProvisionJobName(db *storagev1alpha1.DatabaseServer, accessHash string) string {
	base := naming.EnsureLowerAlphaPrefix(naming.SanitizeLowerHyphen(db.Name), "db")
	return naming.WithRequiredSuffix(base+"-debug-provision", "-"+accessHash[:8], 63, "db")
}

func debugAccessProvisionJobLabels(serverName string) map[string]string {
	return map[string]string{
		databaseServerNameLabelKey:      serverName,
		databaseAccessProvisionLabelKey: labelValueTrue,
		debugAccessComponentLabelKey:    debugAccessComponentLabelValue,
	}
}
