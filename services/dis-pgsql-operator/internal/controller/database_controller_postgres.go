package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
	k8sutil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/k8s"
	to "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	genruntime "github.com/Azure/azure-service-operator/v2/pkg/genruntime"
)

// TODO: at the moment location is hardcoded here, but maybe
// in the future we want to derive it from the database server spec?
// The defaults for storage size and tier are also hardcoded
// until we release the beta version.
const (
	defaultStorageGB               int32 = 32
	loc                                  = "norwayeast"
	defaultAvailabilityZone              = "1"
	defaultHAStandbyZone                 = "2"
	defaultMaintenanceDayOfWeek          = 0
	defaultMaintenanceStartHour          = 3
	defaultMaintenanceStartMinute        = 0
	maintenanceCustomWindowEnabled       = "Enabled"
)

type postgresNetworkConfig struct {
	Network *dbforpostgresqlv1.Network
}

const (
	azureSubnetResourceType         = "Microsoft.Network/virtualNetworks/subnets"
	azurePrivateDNSZoneResourceType = "Microsoft.Network/privateDnsZones"
)

// Reuse the defaults for now
func desiredStorage(db *storagev1alpha1.DatabaseServer) *dbforpostgresqlv1.Storage {
	sizeGB := defaultStorageGB
	autoGrow := dbforpostgresqlv1.Storage_AutoGrow_Enabled
	storageType := dbforpostgresqlv1.Storage_Type_Premium_LRS

	var requestedTier *string

	if db.Spec.Storage != nil {
		if db.Spec.Storage.SizeGB != nil && *db.Spec.Storage.SizeGB > 0 {
			sizeGB = *db.Spec.Storage.SizeGB
		}
		if db.Spec.Storage.Tier != nil && *db.Spec.Storage.Tier != "" {
			requestedTier = db.Spec.Storage.Tier
		}
	}

	asoTier := dbUtil.ResolveStorageTier(sizeGB, requestedTier)

	return &dbforpostgresqlv1.Storage{
		AutoGrow:      &autoGrow,
		StorageSizeGB: to.Ptr(int(sizeGB)),
		Tier:          &asoTier,
		Type:          &storageType,
	}
}

func desiredBackup(db *storagev1alpha1.DatabaseServer) *dbforpostgresqlv1.Backup {
	geoRedundantBackup := dbforpostgresqlv1.Backup_GeoRedundantBackup_Disabled
	return &dbforpostgresqlv1.Backup{
		BackupRetentionDays: to.Ptr(dbUtil.ResolveBackupRetentionDays(db.Spec.ServerType, db.Spec.BackupRetentionDays)),
		GeoRedundantBackup:  &geoRedundantBackup,
	}
}

func desiredHighAvailability(db *storagev1alpha1.DatabaseServer) *dbforpostgresqlv1.HighAvailability {
	mode := dbUtil.ResolveHighAvailabilityMode(db.Spec.ServerType, db.Spec.HighAvailabilityEnabled)
	highAvailability := &dbforpostgresqlv1.HighAvailability{
		Mode: &mode,
	}

	if mode != dbforpostgresqlv1.HighAvailability_Mode_Disabled {
		highAvailability.StandbyAvailabilityZone = to.Ptr(defaultHAStandbyZone)
	}

	return highAvailability
}

func desiredMaintenanceWindow() *dbforpostgresqlv1.MaintenanceWindow {
	return &dbforpostgresqlv1.MaintenanceWindow{
		CustomWindow: to.Ptr(maintenanceCustomWindowEnabled),
		DayOfWeek:    to.Ptr(defaultMaintenanceDayOfWeek),
		StartHour:    to.Ptr(defaultMaintenanceStartHour),
		StartMinute:  to.Ptr(defaultMaintenanceStartMinute),
	}
}

// subnetARMID builds the ARM ID for a subnet in the DB VNet.
func (r *DatabaseServerReconciler) subnetARMIDResourceReference(subnetName string) *genruntime.ResourceReference {

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

func (r *DatabaseServerReconciler) dedicatedPostgresNetworkConfig(
	db *storagev1alpha1.DatabaseServer,
	zoneName string,
) (postgresNetworkConfig, error) {
	// Use the subnet allocated to this database server from the status.
	if db.Status.SubnetCIDR == "" {
		return postgresNetworkConfig{}, fmt.Errorf("database server status has no SubnetCIDR; cannot build network for server")
	}

	// Make sure the subnet exists in our catalog, and therefore in the vnet.
	subnetInfo, ok := r.SubnetCatalog.FindByCIDR(db.Status.SubnetCIDR)
	if !ok {
		return postgresNetworkConfig{}, fmt.Errorf("no subnet in catalog matching CIDR %q", db.Status.SubnetCIDR)
	}

	subnetID := r.subnetARMIDResourceReference(subnetInfo.Name)
	zoneID := r.privateZoneARMIDResourceReference(zoneName)

	publicNetworkAccess := dbforpostgresqlv1.Network_PublicNetworkAccess_Disabled
	return postgresNetworkConfig{
		Network: &dbforpostgresqlv1.Network{
			DelegatedSubnetResourceReference:   subnetID,
			PrivateDnsZoneArmResourceReference: zoneID,
			PublicNetworkAccess:                to.Ptr(publicNetworkAccess),
		},
	}, nil
}

func (r *DatabaseServerReconciler) sharedPostgresNetworkConfig(db *storagev1alpha1.DatabaseServer) (postgresNetworkConfig, error) {
	if db.Spec.Network == nil {
		return postgresNetworkConfig{}, fmt.Errorf("spec.network must be set when mode is Shared")
	}

	subnetResourceID, err := r.validateSharedNetworkResourceID(
		"spec.network.delegatedSubnetResourceId",
		db.Spec.Network.DelegatedSubnetResourceID,
		azureSubnetResourceType,
	)
	if err != nil {
		return postgresNetworkConfig{}, err
	}

	zoneResourceID, err := r.validateSharedNetworkResourceID(
		"spec.network.privateDnsZoneResourceId",
		db.Spec.Network.PrivateDNSZoneResourceID,
		azurePrivateDNSZoneResourceType,
	)
	if err != nil {
		return postgresNetworkConfig{}, err
	}

	publicNetworkAccess := dbforpostgresqlv1.Network_PublicNetworkAccess_Disabled
	return postgresNetworkConfig{
		Network: &dbforpostgresqlv1.Network{
			DelegatedSubnetResourceReference: &genruntime.ResourceReference{
				ARMID: subnetResourceID,
			},
			PrivateDnsZoneArmResourceReference: &genruntime.ResourceReference{
				ARMID: zoneResourceID,
			},
			PublicNetworkAccess: to.Ptr(publicNetworkAccess),
		},
	}, nil
}

func (r *DatabaseServerReconciler) validateSharedNetworkResourceID(fieldPath, resourceID, expectedResourceType string) (string, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return "", fmt.Errorf("%s must be set when mode is Shared", fieldPath)
	}

	parsed, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return "", fmt.Errorf("%s must be a valid ARM resource ID: %w", fieldPath, err)
	}

	subscriptionID := strings.TrimSpace(r.Config.SubscriptionId)
	if subscriptionID == "" {
		return "", fmt.Errorf("operator subscription id is not configured; cannot validate %s", fieldPath)
	}
	if !strings.EqualFold(parsed.SubscriptionID, subscriptionID) {
		return "", fmt.Errorf("%s must be in subscription %q", fieldPath, subscriptionID)
	}

	actualResourceType := parsed.ResourceType.String()
	if !strings.EqualFold(actualResourceType, expectedResourceType) {
		return "", fmt.Errorf("%s must reference %s, got %s", fieldPath, expectedResourceType, actualResourceType)
	}

	return resourceID, nil
}

func resourceReferenceLogValue(ref *genruntime.ResourceReference) string {
	if ref == nil {
		return ""
	}
	if ref.ARMID != "" {
		return ref.ARMID
	}
	if ref.Group != "" || ref.Kind != "" {
		return fmt.Sprintf("%s/%s/%s", ref.Group, ref.Kind, ref.Name)
	}
	return ref.Name
}

// ensurePostgresServer ensures a PostgreSQL Flexible Server ASO resource exists
// for the given database server.
func (r *DatabaseServerReconciler) ensurePostgresServer(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
	networkConfig postgresNetworkConfig,
) error {
	ns := db.Namespace

	// The current DatabaseServer CR represents a PostgreSQL server, so the FlexibleServer
	// uses the CR name.
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
	backup := desiredBackup(db)
	highAvailability := desiredHighAvailability(db)
	maintenanceWindow := desiredMaintenanceWindow()

	versionStr := fmt.Sprintf("%d", db.Spec.Version)
	version := dbforpostgresqlv1.ServerVersion(versionStr)

	if networkConfig.Network == nil {
		return fmt.Errorf("postgres network config must be set")
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

		Version:           &version,
		Network:           networkConfig.Network,
		Storage:           storage,
		Backup:            backup,
		HighAvailability:  highAvailability,
		AvailabilityZone:  to.Ptr(defaultAvailabilityZone),
		MaintenanceWindow: maintenanceWindow,
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

		logger.Info("creating PostgreSQL FlexibleServer for database server",
			"serverName", serverName,
			"namespace", ns,
			"location", loc,
			"version", versionStr,
			"subnetID", resourceReferenceLogValue(networkConfig.Network.DelegatedSubnetResourceReference),
			"zoneID", resourceReferenceLogValue(networkConfig.Network.PrivateDnsZoneArmResourceReference),
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

	var updated bool
	existing.Labels, updated = k8sutil.SyncSpecAndLabels(&existing.Spec, desiredSpec, existing.Labels, desiredLabels)
	if updated {
		logger.Info("updating PostgreSQL FlexibleServer to match database server",
			"serverName", serverName,
			"namespace", ns,
		)
		if err := r.Update(ctx, &existing); err != nil {
			return fmt.Errorf("update FlexibleServer %s/%s: %w", ns, serverName, err)
		}
	}

	return nil
}
