package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	asoconditions "github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	networkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20240601"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/network"
)

const (
	databaseServerConditionReady = "Ready"
	databaseServerReasonReady    = "Ready"
	databaseServerReasonWaiting  = "Waiting"

	// databaseServerConditionDebugAccessReady reports the state of the auxiliary
	// debug-access role assignments. It is intentionally decoupled from the core
	// Ready condition: pending debug identities never block or affect Ready.
	databaseServerConditionDebugAccessReady = "DebugAccessReady"

	// databaseServerReasonDebugAccessProvisioned marks that all requested debug
	// principals were resolved and their role assignments reconciled.
	databaseServerReasonDebugAccessProvisioned = "Provisioned"

	// databaseServerReasonDebugAccessWaitingForIdentity marks that one or more
	// debug identityRef principals cannot be resolved yet because their
	// ApplicationIdentity is missing or not ready.
	databaseServerReasonDebugAccessWaitingForIdentity = "WaitingForIdentity"

	// databaseServerReasonServerNameConflict marks a DatabaseServer whose Azure server
	// name is already taken. Flexible Server names are globally unique, so this is a
	// blocked state the author must resolve by choosing a unique DatabaseServer name.
	databaseServerReasonServerNameConflict = "ServerNameConflict"

	// azureReasonServerNameAlreadyExists is the reason ASO sets on the FlexibleServer's
	// Ready condition when Azure rejects the create because the name is already in use.
	azureReasonServerNameAlreadyExists = "ServerNameAlreadyExists"

	// databaseServerFinalizer lets the controller order teardown of the owned network
	// resources on deletion. The private DNS zone cannot be deleted by Azure while its
	// virtual network links still exist, and the zone + links are sibling children of the
	// DatabaseServer, so unordered garbage collection wedges the zone deletion.
	databaseServerFinalizer = "storage.dis.altinn.cloud/databaseserver-finalizer"

	// databaseServerDeleteRequeueInterval backstops the owned-resource delete watches
	// while Azure tears the children down (deletion is not latency-sensitive).
	databaseServerDeleteRequeueInterval = 15 * time.Second
)

// DatabaseServerReconciler reconciles the current DatabaseServer CRD as a PostgreSQL server.
type DatabaseServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// SubnetCatalog is the static list of available subnets for this environment.
	// It is loaded once at startup (from Azure via FetchSubnetCatalog) and injected
	// into the reconciler.
	SubnetCatalog *network.SubnetCatalog

	Config config.OperatorConfig
}

// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=databaseservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=databaseservers/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// It compares the DatabaseServer object against the actual cluster state, and then
// performs operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/reconcile
// ASO: PostgreSQL Flexible Server
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleserversconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleserversconfigurations/status,verbs=get;update;patch

// ASO: Flexible Server AAD administrator
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleserversadministrators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleserversadministrators/status,verbs=get;update;patch

// ASO: Azure Reader role assignments for debug access
// +kubebuilder:rbac:groups=authorization.azure.com,resources=roleassignments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=authorization.azure.com,resources=roleassignments/status,verbs=get;update;patch

// ASO: Private DNS zone + vnet links
// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszones,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszones/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszonesvirtualnetworklinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszonesvirtualnetworklinks/status,verbs=get;update;patch

// ApplicationIdentity (dis-application)
// +kubebuilder:rbac:groups=application.dis.altinn.cloud,resources=applicationidentities,verbs=get;list;watch

func (r *DatabaseServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("databaseServer", req.NamespacedName)

	var db storagev1alpha1.DatabaseServer
	if err := r.Get(ctx, req.NamespacedName, &db); err != nil {
		if apierrors.IsNotFound(err) {
			// Deleted
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !db.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, logger, &db)
	}

	// Register the teardown finalizer before any owned resources are created, so deletion
	// can always sequence the network children. Returning after adding it lets the update
	// re-trigger reconciliation cleanly.
	if controllerutil.AddFinalizer(&db, databaseServerFinalizer) {
		if err := r.Update(ctx, &db); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer to DatabaseServer %s/%s: %w", db.Namespace, db.Name, err)
		}
		return ctrl.Result{}, nil
	}

	if databaseServerMode(&db) == storagev1alpha1.DatabaseServerModeShared {
		return r.reconcileSharedDatabaseServer(ctx, logger, &db)
	}
	return r.reconcileDedicatedDatabaseServer(ctx, logger, &db)
}

// reconcileDelete tears down the owned resources in an order Azure accepts, then removes
// the finalizer. The private DNS zone can only be deleted once its virtual network links
// (and the FlexibleServer that references it) are gone, so those are deleted first and the
// zone is held back until they no longer exist. Resources without ordering constraints
// (server parameters, the administrator) are left to garbage collection once the finalizer
// is removed.
func (r *DatabaseServerReconciler) reconcileDelete(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(db, databaseServerFinalizer) {
		return ctrl.Result{}, nil
	}

	// Only dedicated servers own a private DNS zone and its links; shared servers reference
	// pre-existing external network resources that the operator must not delete.
	if databaseServerMode(db) == storagev1alpha1.DatabaseServerModeDedicated {
		pending, err := r.deleteDedicatedNetworkChildren(ctx, logger, db)
		if err != nil {
			return ctrl.Result{}, err
		}
		if pending {
			return ctrl.Result{RequeueAfter: databaseServerDeleteRequeueInterval}, nil
		}
	}

	controllerutil.RemoveFinalizer(db, databaseServerFinalizer)
	if err := r.Update(ctx, db); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer from DatabaseServer %s/%s: %w", db.Namespace, db.Name, err)
	}
	return ctrl.Result{}, nil
}

// deleteDedicatedNetworkChildren deletes the FlexibleServer and virtual network links,
// then the private DNS zone, never attempting the zone until the first wave is fully gone.
// It returns pending=true while any of them still exist so the caller can requeue.
func (r *DatabaseServerReconciler) deleteDedicatedNetworkChildren(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
) (pending bool, err error) {
	// First wave: the resources that block the private DNS zone deletion in Azure.
	firstWaveGone := true
	for _, child := range []struct {
		name string
		obj  client.Object
	}{
		{db.Name, &dbforpostgresqlv1.FlexibleServer{}},
		{dbVNetLinkNameForDatabaseServer(db), &networkv1.PrivateDnsZonesVirtualNetworkLink{}},
		{aksVNetLinkNameForDatabaseServer(db), &networkv1.PrivateDnsZonesVirtualNetworkLink{}},
	} {
		gone, err := r.ensureChildDeleted(ctx, logger, db.Namespace, child.name, child.obj)
		if err != nil {
			return false, err
		}
		if !gone {
			firstWaveGone = false
		}
	}
	if !firstWaveGone {
		return true, nil
	}

	// Second wave: the zone, now that nothing nested under it remains.
	zoneGone, err := r.ensureChildDeleted(ctx, logger, db.Namespace, zoneCRNameForDatabaseServer(db), &networkv1.PrivateDnsZone{})
	if err != nil {
		return false, err
	}
	return !zoneGone, nil
}

// ensureChildDeleted issues a delete for the named owned resource if it is still present
// and not already being deleted, and reports whether it is fully gone.
func (r *DatabaseServerReconciler) ensureChildDeleted(
	ctx context.Context,
	logger logr.Logger,
	namespace, name string,
	obj client.Object,
) (gone bool, err error) {
	key := types.NamespacedName{Name: name, Namespace: namespace}
	if err := r.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("get %T %s/%s for deletion: %w", obj, namespace, name, err)
	}

	if obj.GetDeletionTimestamp().IsZero() {
		logger.Info("deleting owned resource during DatabaseServer teardown",
			"kind", fmt.Sprintf("%T", obj),
			"name", name,
			"namespace", namespace,
		)
		if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("delete %T %s/%s: %w", obj, namespace, name, err)
		}
	}

	// Still present (deletion is in progress in Azure via ASO); not gone yet.
	return false, nil
}

func databaseServerMode(db *storagev1alpha1.DatabaseServer) storagev1alpha1.DatabaseServerMode {
	if db.Spec.Mode == storagev1alpha1.DatabaseServerModeShared {
		return storagev1alpha1.DatabaseServerModeShared
	}
	return storagev1alpha1.DatabaseServerModeDedicated
}

func (r *DatabaseServerReconciler) reconcileDedicatedDatabaseServer(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
) (ctrl.Result, error) {
	if r.SubnetCatalog == nil {
		return ctrl.Result{}, fmt.Errorf("SubnetCatalog is not configured on DatabaseServerReconciler")
	}

	// Only allocate if the server doesn't already have one.
	if db.Status.SubnetCIDR == "" {
		logger.Info("allocating subnet for database server")
		if err := r.allocateSubnetForDatabaseServer(ctx, logger, db); err != nil {
			if errors.Is(err, network.ErrNoFreeSubnets) {
				logger.Info("no free subnets available, will retry later", "error", err.Error())

				if err := r.setDatabaseServerReadyCondition(ctx, db, metav1.ConditionFalse, "NoFreeSubnets", "No free subnet CIDRs available in the configured catalog"); err != nil {
					logger.Error(err, "failed to update database server status after no free subnets")
					return ctrl.Result{}, err
				}

				// Requeue after some delay so we can pick up newly freed subnets.
				// e.g. if another database server is deleted and frees a CIDR.
				// TODO: define later if 5 minutes is a good interval here.
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}

			// All other errors are real failures; let controller-runtime backoff
			logger.Error(err, "failed to allocate subnet")
			return ctrl.Result{}, err
		}
		// We don't need to requeue here, as the status update by allocating the subnet
		// will trigger another reconciliation.
		return ctrl.Result{}, nil
	} else {
		logger.Info("database server already has subnetCIDR", "subnetCIDR", db.Status.SubnetCIDR)
	}

	// Private Dns zone
	if err := r.ensurePrivateDNSZone(ctx, logger, db); err != nil {
		logger.Error(err, "failed to ensure private DNS zone")
		return ctrl.Result{}, err
	}

	// create ARM IDs for vnet links
	dbVnetID := fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s",
		r.Config.SubscriptionId,
		r.Config.ResourceGroup,
		r.Config.DBVNetName,
	)

	aksVnetID := fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s",
		r.Config.SubscriptionId,
		r.Config.AKSResourceGroup,
		r.Config.AKSVNetName,
	)

	// DB VNet link
	if err := r.ensurePrivateDNSVNetLink(
		ctx, logger, db,
		zoneCRNameForDatabaseServer(db),
		dbVNetLinkNameForDatabaseServer(db),
		r.Config.DBVNetName,
		dbVnetID,
	); err != nil {
		logger.Error(err, "failed to ensure private DNS vnet link for DB VNet")
		return ctrl.Result{}, err
	}

	// AKS VNet link
	if err := r.ensurePrivateDNSVNetLink(
		ctx, logger, db,
		zoneCRNameForDatabaseServer(db),
		aksVNetLinkNameForDatabaseServer(db),
		r.Config.AKSVNetName,
		aksVnetID,
	); err != nil {
		logger.Error(err, "failed to ensure private DNS vnet link for AKS VNet")
		return ctrl.Result{}, err
	}

	networkConfig, err := r.dedicatedPostgresNetworkConfig(db, zoneCRNameForDatabaseServer(db))
	if err != nil {
		logger.Error(err, "failed to build dedicated PostgreSQL network config")
		return ctrl.Result{}, err
	}

	if err := r.ensurePostgresServer(ctx, logger, db, networkConfig); err != nil {
		logger.Error(err, "failed to ensure PostgreSQLFlexibleServer for database server")
		return ctrl.Result{}, err
	}

	return r.reconcileCommonDatabaseServerResources(ctx, logger, db)
}

func (r *DatabaseServerReconciler) reconcileSharedDatabaseServer(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
) (ctrl.Result, error) {
	networkConfig, err := r.sharedPostgresNetworkConfig(db)
	if err != nil {
		logger.Error(err, "failed to build shared PostgreSQL network config")
		return ctrl.Result{}, err
	}

	if err := r.ensurePostgresServer(ctx, logger, db, networkConfig); err != nil {
		logger.Error(err, "failed to ensure PostgreSQLFlexibleServer for shared database server")
		return ctrl.Result{}, err
	}

	return r.reconcileCommonDatabaseServerResources(ctx, logger, db)
}

func (r *DatabaseServerReconciler) reconcileCommonDatabaseServerResources(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
) (ctrl.Result, error) {
	// Surface a blocked/failed FlexibleServer (e.g. a globally-taken server name) on the
	// DatabaseServer before reconciling the server's child resources. The owned
	// FlexibleServersConfiguration/administrator children only report a misleading
	// "owner cannot be found" error while the server itself is the resource that failed,
	// so checking the server first keeps the real cause visible. Skipped under az fakes,
	// mirroring the asoResourcesReady check below.
	if !r.Config.UseAzFakes {
		blocked, result, err := r.surfaceBlockedFlexibleServer(ctx, logger, db)
		if err != nil || blocked {
			return result, err
		}
	}

	if err := r.ensurePostgresExtensionSettings(ctx, logger, db); err != nil {
		logger.Error(err, "failed to ensure PostgreSQL extension settings for database server")
		return ctrl.Result{}, err
	}

	if err := r.ensurePostgresServerParameters(ctx, logger, db); err != nil {
		logger.Error(err, "failed to ensure PostgreSQL server parameters for database server")
		return ctrl.Result{}, err
	}

	adminIdentity, requeue, err := r.resolveAdminIdentity(ctx, logger, db)
	if err != nil {
		logger.Error(err, "failed to resolve admin identity")
		return ctrl.Result{}, err
	}
	if requeue {
		logger.Info("waiting for admin ApplicationIdentity to be ready")
		if err := r.setDatabaseServerReadyCondition(ctx, db, metav1.ConditionFalse, databaseServerReasonWaiting, "Waiting for admin ApplicationIdentity to be ready"); err != nil {
			logger.Error(err, "failed to update database server readiness while waiting for admin identity")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	// Flexible Server admin
	if err := r.ensureFlexibleServerAdministrator(ctx, logger, db, adminIdentity); err != nil {
		logger.Error(err, "failed to ensure FlexibleServerAdministrator for database server")
		return ctrl.Result{}, err
	}

	// Debug access: Azure Reader on the Flexible Server.
	if err := r.ensureDebugAccessRoleAssignments(ctx, logger, db); err != nil {
		logger.Error(err, "failed to ensure debug access role assignments for database server")
		return ctrl.Result{}, err
	}

	// Debug access (data plane): read-only PostgreSQL access for the same
	// principals. Dedicated-only, mirroring the role-assignment path above.
	if databaseServerMode(db) != storagev1alpha1.DatabaseServerModeShared {
		if err := r.ensureDebugAccessProvisioning(ctx, logger, db, adminIdentity); err != nil {
			logger.Error(err, "failed to ensure debug access provisioning for database server")
			return ctrl.Result{}, err
		}
	}

	if !r.Config.UseAzFakes {
		ready, err := r.asoResourcesReady(ctx, logger, db)
		if err != nil {
			logger.Error(err, "failed to check ASO readiness for database server")
			return ctrl.Result{}, err
		}
		if !ready {
			logger.Info("waiting for ASO resources to be ready")
			if err := r.setDatabaseServerReadyCondition(ctx, db, metav1.ConditionFalse, databaseServerReasonWaiting, "Waiting for ASO resources to be ready"); err != nil {
				logger.Error(err, "failed to update database server readiness while waiting for ASO resources")
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}
	}

	if err := r.setDatabaseServerReadyCondition(ctx, db, metav1.ConditionTrue, databaseServerReasonReady, "DatabaseServer is ready"); err != nil {
		logger.Error(err, "failed to update database server readiness")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DatabaseServerReconciler) setDatabaseServerReadyCondition(
	ctx context.Context,
	db *storagev1alpha1.DatabaseServer,
	status metav1.ConditionStatus,
	reason,
	message string,
) error {
	previousStatus := db.Status.DeepCopy()
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type:               databaseServerConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: db.Generation,
	})

	if apiequality.Semantic.DeepEqual(previousStatus, &db.Status) {
		return nil
	}

	return r.Status().Update(ctx, db)
}

func (r *DatabaseServerReconciler) allocateSubnetForDatabaseServer(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
) error {
	// Collect used subnets from all DatabaseServer resources that currently represent servers.
	var dbList storagev1alpha1.DatabaseServerList
	if err := r.List(ctx, &dbList); err != nil {
		return fmt.Errorf("list database servers: %w", err)
	}

	var used []string
	for _, other := range dbList.Items {
		if other.Status.SubnetCIDR != "" {
			used = append(used, other.Status.SubnetCIDR)
		}
	}

	logger.Info("collected used subnets", "used", used)

	free, err := r.SubnetCatalog.FirstFreeSubnet(used)
	if err != nil {
		return fmt.Errorf("find first free subnet: %w", err)
	}

	logger.Info("allocated subnet", "cidr", free.CIDR)

	// Write to status and persist it
	db.Status.SubnetCIDR = free.CIDR
	if err := r.Status().Update(ctx, db); err != nil {
		return fmt.Errorf("update database server status with SubnetCIDR: %w", err)
	}

	return nil
}

func (r *DatabaseServerReconciler) asoResourcesReady(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
) (bool, error) {
	ns := db.Namespace

	serverName := db.Name
	adminName := fmt.Sprintf("%s-admin", db.Name)

	var server dbforpostgresqlv1.FlexibleServer
	if err := r.Get(ctx, types.NamespacedName{Name: serverName, Namespace: ns}, &server); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("FlexibleServer not found yet", "server", serverName)
			return false, nil
		}
		return false, fmt.Errorf("get FlexibleServer %s/%s: %w", ns, serverName, err)
	}

	// Publish the Azure-side identity on the DatabaseServer status as soon as ASO
	// has filled it in. setDatabaseServerReadyCondition flushes the status, so
	// this only needs to mutate the in-memory object.
	if server.Status.FullyQualifiedDomainName != nil {
		if host := strings.TrimSpace(*server.Status.FullyQualifiedDomainName); host != "" {
			db.Status.Host = host
		}
	}

	serverStatus, serverReason, serverMessage, serverReady := readyConditionInfo(server.Status.Conditions)
	if !serverReady || serverStatus != metav1.ConditionTrue {
		logger.Info("FlexibleServer not ready yet",
			"server", serverName,
			"status", serverStatus,
			"reason", serverReason,
			"message", serverMessage,
		)
		return false, nil
	}

	var admin dbforpostgresqlv1.FlexibleServersAdministrator
	if err := r.Get(ctx, types.NamespacedName{Name: adminName, Namespace: ns}, &admin); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("FlexibleServersAdministrator not found yet", "admin", adminName)
			return false, nil
		}
		return false, fmt.Errorf("get FlexibleServersAdministrator %s/%s: %w", ns, adminName, err)
	}

	adminStatus, adminReason, adminMessage, adminReady := readyConditionInfo(admin.Status.Conditions)
	if !adminReady || adminStatus != metav1.ConditionTrue {
		logger.Info("FlexibleServersAdministrator not ready yet",
			"admin", adminName,
			"status", adminStatus,
			"reason", adminReason,
			"message", adminMessage,
		)
		return false, nil
	}

	return true, nil
}

func readyConditionInfo(
	conds []asoconditions.Condition,
) (status metav1.ConditionStatus, reason, message string, ok bool) {
	cond, found := findReadyCondition(conds)
	if !found {
		return "", "", "", false
	}
	return cond.Status, cond.Reason, cond.Message, true
}

// findReadyCondition returns the ASO Ready condition, if present.
func findReadyCondition(conds []asoconditions.Condition) (asoconditions.Condition, bool) {
	for i := range conds {
		if conds[i].Type == asoconditions.ConditionTypeReady {
			return conds[i], true
		}
	}
	return asoconditions.Condition{}, false
}

// surfaceBlockedFlexibleServer inspects the owned FlexibleServer after it has been
// ensured. When the server reports a non-transient Ready=False state (an actual Azure
// failure such as ServerNameAlreadyExists, as opposed to the normal "Reconciling"
// progress, which is also Ready=False but with Info severity), it records that failure on
// the DatabaseServer Ready condition, clears any stale server-parameter errors, and asks
// the caller to stop before the server's child resources are reconciled. It returns
// blocked=false for the healthy/in-progress cases so reconciliation proceeds as usual.
func (r *DatabaseServerReconciler) surfaceBlockedFlexibleServer(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
) (bool, ctrl.Result, error) {
	var server dbforpostgresqlv1.FlexibleServer
	if err := r.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, &server); err != nil {
		if apierrors.IsNotFound(err) {
			// Just created (or cache lag): nothing to surface yet.
			return false, ctrl.Result{}, nil
		}
		return false, ctrl.Result{}, fmt.Errorf("get FlexibleServer %s/%s: %w", db.Namespace, db.Name, err)
	}

	cond, ok := findReadyCondition(server.Status.Conditions)
	if !ok || cond.Status != metav1.ConditionFalse {
		// No Ready condition yet, or it is True/Unknown: not a failure.
		return false, ctrl.Result{}, nil
	}
	if cond.Severity == asoconditions.ConditionSeverityInfo || cond.Severity == asoconditions.ConditionSeverityNone {
		// Ready=False at Info severity is the normal in-progress (Reconciling) state.
		return false, ctrl.Result{}, nil
	}

	reason, message := describeBlockedFlexibleServer(db.Name, cond.Reason, cond.Message)
	logger.Info("FlexibleServer reported a blocked state; surfacing it on the DatabaseServer and deferring child resources",
		"server", db.Name,
		"flexibleServerReason", cond.Reason,
		"severity", string(cond.Severity),
	)
	if err := r.setDatabaseServerBlockedCondition(ctx, db, reason, message); err != nil {
		return false, ctrl.Result{}, err
	}

	// Requeue: the block may clear once the conflicting name is freed.
	return true, ctrl.Result{RequeueAfter: time.Minute}, nil
}

// describeBlockedFlexibleServer maps a FlexibleServer Ready=False reason/message to a
// DatabaseServer condition reason/message, giving known-terminal Azure errors an
// actionable explanation while passing other failures through unchanged.
func describeBlockedFlexibleServer(serverName, asoReason, asoMessage string) (reason, message string) {
	asoMessage = strings.TrimSpace(asoMessage)
	switch asoReason {
	case azureReasonServerNameAlreadyExists:
		message = fmt.Sprintf(
			"Azure PostgreSQL server name %q is already in use. Flexible Server names are globally unique, so choose a unique DatabaseServer name.",
			serverName,
		)
		if asoMessage != "" {
			message = fmt.Sprintf("%s Azure reported: %s", message, asoMessage)
		}
		return databaseServerReasonServerNameConflict, message
	default:
		if asoMessage == "" {
			asoMessage = "FlexibleServer is not ready"
		}
		if asoReason == "" {
			asoReason = databaseServerReasonWaiting
		}
		return asoReason, asoMessage
	}
}

// setDatabaseServerBlockedCondition records a blocked Ready=False state and drops any
// server-parameter errors/condition. While the server itself is blocked, those children
// only echo the misleading "owner cannot be found" failure, so they are cleared in the
// same status update to keep the real cause visible.
func (r *DatabaseServerReconciler) setDatabaseServerBlockedCondition(
	ctx context.Context,
	db *storagev1alpha1.DatabaseServer,
	reason, message string,
) error {
	previousStatus := db.Status.DeepCopy()

	db.Status.ServerParameterErrors = nil
	meta.RemoveStatusCondition(&db.Status.Conditions, serverParametersReadyConditionType)
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type:               databaseServerConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: db.Generation,
	})

	if apiequality.Semantic.DeepEqual(previousStatus, &db.Status) {
		return nil
	}

	return r.Status().Update(ctx, db)
}

func (r *DatabaseServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.DatabaseServer{}).
		Owns(&networkv1.PrivateDnsZone{}).
		Owns(&networkv1.PrivateDnsZonesVirtualNetworkLink{}).
		Owns(&dbforpostgresqlv1.FlexibleServer{}).
		Owns(&dbforpostgresqlv1.FlexibleServersConfiguration{}).
		Owns(&dbforpostgresqlv1.FlexibleServersAdministrator{}).
		Owns(&authorizationv1.RoleAssignment{}).
		Watches(&identityv1alpha1.ApplicationIdentity{}, handler.EnqueueRequestsFromMapFunc(r.mapApplicationIdentityToDatabaseServers)).
		WithOptions(controller.Options{
			// Force single-threaded reconciliation
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
