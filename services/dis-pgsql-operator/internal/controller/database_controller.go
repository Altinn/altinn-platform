package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	networkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20240601"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/network"
)

// DatabaseReconciler reconciles a Database object
type DatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// SubnetCatalog is the static list of available subnets for this environment.
	// It is loaded once at startup (from Azure via FetchSubnetCatalog) and injected
	// into the reconciler.
	SubnetCatalog *network.SubnetCatalog

	Config config.OperatorConfig
}

// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=databases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=databases/status,verbs=get;update;patch

// ASO: PostgreSQL Flexible Server
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleservers/status,verbs=get;update;patch

// ASO: Flexible Server AAD administrator
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleserversadministrators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dbforpostgresql.azure.com,resources=flexibleserversadministrators/status,verbs=get;update;patch

// ASO: Private DNS zone + vnet links
// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszones,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszones/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszonesvirtualnetworklinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszonesvirtualnetworklinks/status,verbs=get;update;patch

func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("database", req.NamespacedName)

	if r.SubnetCatalog == nil {
		return ctrl.Result{}, fmt.Errorf("SubnetCatalog is not configured on DatabaseReconciler")
	}

	var db storagev1alpha1.Database
	if err := r.Get(ctx, req.NamespacedName, &db); err != nil {
		if apierrors.IsNotFound(err) {
			// Deleted
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !db.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Only allocate if we don't already have one
	if db.Status.SubnetCIDR == "" {
		logger.Info("allocating subnet for database")
		if err := r.allocateSubnetForDatabase(ctx, logger, &db); err != nil {
			if errors.Is(err, network.ErrNoFreeSubnets) {
				logger.Info("no free subnets available, will retry later", "error", err.Error())

				meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
					Type:    "Ready",
					Status:  metav1.ConditionFalse,
					Reason:  "NoFreeSubnets",
					Message: "No free subnet CIDRs available in the configured catalog",
				})

				if err := r.Status().Update(ctx, &db); err != nil {
					logger.Error(err, "failed to update Database status after no free subnets")
					return ctrl.Result{}, err
				}

				// Requeue after some delay so we can pick up newly freed subnets.
				// e.g. if another Database is deleted and frees a CIDR.
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
		logger.Info("database already has subnetCIDR", "subnetCIDR", db.Status.SubnetCIDR)
	}

	// Private Dns zone
	if err := r.ensurePrivateDNSZone(ctx, logger, &db); err != nil {
		logger.Error(err, "failed to ensure private DNS zone")
		return ctrl.Result{}, err
	}

	// DB VNet link
	if err := r.ensurePrivateDNSVNetLink(
		ctx, logger, &db,
		zoneNameForDatabase(&db),
		vnetLinkNameForDB(&db),
		r.Config.DBVNetName,
	); err != nil {
		logger.Error(err, "failed to ensure private DNS vnet link for DB VNet")
		return ctrl.Result{}, err
	}

	// AKS VNet link
	if err := r.ensurePrivateDNSVNetLink(
		ctx, logger, &db,
		zoneNameForDatabase(&db),
		vnetLinkNameForAKS(&db),
		r.Config.AKSVNetName,
	); err != nil {
		logger.Error(err, "failed to ensure private DNS vnet link for AKS VNet")
		return ctrl.Result{}, err
	}

	// PostgreSQL Flexible Server
	if err := r.ensurePostgresServer(ctx, logger, &db, zoneNameForDatabase(&db)); err != nil {
		logger.Error(err, "failed to ensure PostgreSQLFlexibleServer for database")
		return ctrl.Result{}, err
	}

	// Flexible Server admin
	if err := r.ensureFlexibleServerAdministrator(ctx, logger, &db); err != nil {
		logger.Error(err, "failed to ensure FlexibleServerAdministrator for database")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DatabaseReconciler) vnetARMID(vnetName string) (string, error) {
	if vnetName == "" {
		return "", fmt.Errorf("vnet name is empty")
	}

	return fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s",
		r.Config.SubscriptionId,
		r.Config.ResourceGroup,
		vnetName,
	), nil
}

func (r *DatabaseReconciler) allocateSubnetForDatabase(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
) error {
	// Collect used subnets from all Database resources.
	var dbList storagev1alpha1.DatabaseList
	if err := r.List(ctx, &dbList); err != nil {
		return fmt.Errorf("list Databases: %w", err)
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
		return fmt.Errorf("update Database status with SubnetCIDR: %w", err)
	}

	return nil
}

func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.Database{}).
		Owns(&networkv1.PrivateDnsZone{}).
		Owns(&networkv1.PrivateDnsZonesVirtualNetworkLink{}).
		Owns(&dbforpostgresqlv1.FlexibleServer{}).
		Owns(&dbforpostgresqlv1.FlexibleServersAdministrator{}).
		WithOptions(controller.Options{
			// Force single-threaded reconciliation
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
