package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
	to "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	genruntime "github.com/Azure/azure-service-operator/v2/pkg/genruntime"
)

// TODO: at the moment location is hardcoded here, but maybe
// in the future we want to derive it from the Database spec?
// The defaults for storage size and tier are also hardcoded
// until we release the beta version.
const (
	defaultStorageGB   int32  = 32
	defaultStorageTier string = "P10"
	loc                       = "norwayeast"
)

// Reuse the defaults for now
func desiredStorage(db *storagev1alpha1.Database) *dbforpostgresqlv1.Storage {

	sizeGB := defaultStorageGB
	tierStr := defaultStorageTier
	autoGrow := dbforpostgresqlv1.Storage_AutoGrow_Enabled

	if db.Spec.Storage != nil {
		if db.Spec.Storage.SizeGB != nil && *db.Spec.Storage.SizeGB > 0 {
			sizeGB = *db.Spec.Storage.SizeGB
		}
		if db.Spec.Storage.Tier != nil && *db.Spec.Storage.Tier != "" {
			tierStr = *db.Spec.Storage.Tier
		}
	}

	asoTier := dbforpostgresqlv1.Storage_Tier(tierStr)

	return &dbforpostgresqlv1.Storage{
		AutoGrow:      &autoGrow,
		StorageSizeGB: to.Ptr(int(sizeGB)),
		Tier:          &asoTier,
	}
}

// subnetARMID builds the ARM ID for a subnet in the DB VNet.
func (r *DatabaseReconciler) subnetARMIDResourceReference(subnetName string) *genruntime.ResourceReference {

	return &genruntime.ResourceReference{
		ARMID: fmt.Sprintf(
			"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
			r.Config.SubscriptionId,
			r.Config.ResourceGroup,
			r.Config.DBVNetName,
			subnetName,
		),
	}
}

// ensurePostgresServer ensures a PostgreSQL Flexible Server ASO resource exists
// for the given Database.
func (r *DatabaseReconciler) ensurePostgresServer(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
	zoneName string,
) error {
	ns := db.Namespace

	// use the Database resource name for now.
	serverName := db.Name

	key := types.NamespacedName{
		Name:      serverName,
		Namespace: ns,
	}

	var existing dbforpostgresqlv1.FlexibleServer
	found := true
	if err := r.Get(ctx, key, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			found = false
		} else {
			return fmt.Errorf("get FlexibleServer %s/%s: %w", ns, serverName, err)
		}
	}

	// define if dev/prod profile
	profile := dbUtil.GetProfile(db.Spec.ServerType)

	// define storage size and tier
	storage := desiredStorage(db)

	versionStr := fmt.Sprintf("%d", db.Spec.Version)
	version := dbforpostgresqlv1.ServerVersion(versionStr)

	// Use the subnet allocated to this database from the status
	if db.Status.SubnetCIDR == "" {
		return fmt.Errorf("database status has no SubnetCIDR; cannot build network for server")
	}

	// Make. sure the subnet exists in our catalog,
	// and therefore in the vnet.
	subnetInfo, ok := r.SubnetCatalog.FindByCIDR(db.Status.SubnetCIDR)
	if !ok {
		return fmt.Errorf("no subnet in catalog matching CIDR %q", db.Status.SubnetCIDR)
	}

	subnetID := r.subnetARMIDResourceReference(subnetInfo.Name)

	zoneID := r.privateZoneARMIDResourceReference(zoneName)

	network := &dbforpostgresqlv1.Network{
		DelegatedSubnetResourceReference:   subnetID,
		PrivateDnsZoneArmResourceReference: zoneID,
		PublicNetworkAccess:                to.Ptr(dbforpostgresqlv1.Network_PublicNetworkAccess_Disabled),
	}

	// AD auth settings
	adEnabled := dbforpostgresqlv1.AuthConfig_ActiveDirectoryAuth_Enabled
	pwDisabled := dbforpostgresqlv1.AuthConfig_PasswordAuth_Disabled

	authConfig := &dbforpostgresqlv1.AuthConfig{
		ActiveDirectoryAuth: &adEnabled,
		PasswordAuth:        &pwDisabled,
		TenantId:            to.Ptr(r.Config.TenantId),
	}

	// 5) Build server
	desiredSpec := dbforpostgresqlv1.FlexibleServer_Spec{
		AzureName: serverName,
		Location:  to.Ptr(loc),

		Owner: &genruntime.KnownResourceReference{
			ARMID: fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", r.Config.SubscriptionId, r.Config.ResourceGroup),
		},

		Version: &version,
		Network: network,
		Storage: storage,
		Sku: &dbforpostgresqlv1.Sku{
			Name: to.Ptr(profile.SkuName),
			Tier: to.Ptr(profile.SkuTier),
		},

		Tags: map[string]string{
			"dis-database": db.Name,
		},

		AuthConfig: authConfig,
	}

	desiredLabels := map[string]string{
		"dis.altinn.cloud/database-name": db.Name,
	}

	// Create when missing
	if !found {
		server := &dbforpostgresqlv1.FlexibleServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serverName,
				Namespace: ns,
				Labels:    desiredLabels,
			},
			Spec: desiredSpec,
		}

		if err := controllerutil.SetControllerReference(db, server, r.Scheme); err != nil {
			return fmt.Errorf("set controller reference on FlexibleServer: %w", err)
		}

		logger.Info("creating PostgreSQL FlexibleServer for database",
			"serverName", serverName,
			"namespace", ns,
			"location", loc,
			"version", versionStr,
			"subnetID", subnetID,
			"zoneID", zoneID,
			"skuName", profile.SkuName,
			"storageGB", storage.StorageSizeGB,
		)

		if err := r.Create(ctx, server); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return fmt.Errorf("create FlexibleServer %s/%s: %w", ns, serverName, err)
		}
		return nil
	}

	updated := false
	if !equality.Semantic.DeepEqual(existing.Spec, desiredSpec) {
		existing.Spec = desiredSpec
		updated = true
	}
	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	for k, v := range desiredLabels {
		if existing.Labels[k] != v {
			existing.Labels[k] = v
			updated = true
		}
	}
	if updated {
		logger.Info("updating PostgreSQL FlexibleServer to match Database",
			"serverName", serverName,
			"namespace", ns,
		)
		if err := r.Update(ctx, &existing); err != nil {
			return fmt.Errorf("update FlexibleServer %s/%s: %w", ns, serverName, err)
		}
	}

	return nil
}
