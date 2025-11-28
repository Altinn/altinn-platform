package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
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
}

// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=databases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.dis.altinn.cloud,resources=databases/status,verbs=get;update;patch

func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("database", req.NamespacedName.String())

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
			logger.Error(err, "failed to allocate subnet")
			return ctrl.Result{}, err
		}
	} else {
		logger.Info("database already has subnetCIDR", "subnetCIDR", db.Status.SubnetCIDR)
	}

	return ctrl.Result{}, nil
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
		WithOptions(controller.Options{
			// Force single-threaded reconciliation
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
