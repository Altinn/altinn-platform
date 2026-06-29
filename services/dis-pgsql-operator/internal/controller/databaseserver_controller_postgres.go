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
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
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

	// flexibleServerNameMaxLen is Azure's upper bound on a PostgreSQL Flexible
	// Server name (3-63 chars, lowercase letters/digits/hyphens, starts with a
	// letter). New AzureNames must respect it even after the uniqueness suffix.
	flexibleServerNameMaxLen = 63

	// flexibleServerNameFallback is used when db.Name is empty or sanitizes to
	// nothing usable; the helper requires a non-empty fallback to stay inside
	// Azure's naming rules.
	flexibleServerNameFallback = "dispg-srv"
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
	autoGrow := dbforpostgresqlv1.StorageAutoGrow_Enabled
	storageType := dbforpostgresqlv1.StorageType_Premium_LRS

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
	zoneCRName string,
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
	zoneID := r.privateZoneARMIDResourceReference(zoneCRName)

	publicNetworkAccess := dbforpostgresqlv1.ServerPublicNetworkAccessState_Disabled
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

	publicNetworkAccess := dbforpostgresqlv1.ServerPublicNetworkAccessState_Disabled
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

// flexibleServerAzureName resolves the globally-unique Azure name for the
// FlexibleServer that backs this DatabaseServer.
//
// When a FlexibleServer already exists with a non-empty Spec.AzureName, that
// value is reused so existing servers keep their Azure identity (changing
// AzureName triggers a destructive recreate in ASO). When the FlexibleServer
// does not exist yet, a new name "<db.Name>-<cluster-id>" is derived from the
// operator's --cluster-id flag, which is the same per-cluster suffix
// out-of-cluster consumers (service-owner Terraform that cannot read K8s
// status) use to compute the server name. Two DatabaseServers with the same
// CR name in different clusters get distinct AzureNames because each cluster
// has its own cluster-id.
func (r *DatabaseServerReconciler) flexibleServerAzureName(
	db *storagev1alpha1.DatabaseServer,
	existing *dbforpostgresqlv1.FlexibleServer,
) string {
	if existing != nil {
		if name := strings.TrimSpace(existing.Spec.AzureName); name != "" {
			return name
		}
	}
	suffix := naming.SanitizeLowerHyphen(r.Config.ClusterId)
	return naming.WithRequiredSuffix(db.Name, "-"+suffix, flexibleServerNameMaxLen, flexibleServerNameFallback)
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

	// The FlexibleServer's Kubernetes object name stays equal to the DatabaseServer
	// CR name (used as a stable identifier across watches, owner refs, status
	// lookups). The Azure-side server name (Spec.AzureName) is the globally unique
	// identifier in Azure's PostgreSQL namespace and is resolved separately below
	// after we know whether the resource already exists.
	k8sName := db.Name

	key := types.NamespacedName{
		Name:      k8sName,
		Namespace: ns,
	}

	var existing dbforpostgresqlv1.FlexibleServer
	found := true
	if err := r.Get(ctx, key, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			found = false
		} else {
			return fmt.Errorf("get FlexibleServer %s/%s: %w", ns, k8sName, err)
		}
	}

	var existingPtr *dbforpostgresqlv1.FlexibleServer
	if found {
		existingPtr = &existing
	}
	serverAzureName := r.flexibleServerAzureName(db, existingPtr)
	db.Status.ServerName = serverAzureName

	// define if dev/prod profile
	profile := dbUtil.GetProfile(db.Spec.ServerType)

	// define storage size and tier
	storage := desiredStorage(db)
	backup := desiredBackup(db)
	highAvailability := desiredHighAvailability(db)
	maintenanceWindow := desiredMaintenanceWindow()

	versionStr := fmt.Sprintf("%d", db.Spec.Version)
	version := dbforpostgresqlv1.PostgresMajorVersion(versionStr)

	if networkConfig.Network == nil {
		return fmt.Errorf("postgres network config must be set")
	}

	// AD auth settings
	adEnabled := dbforpostgresqlv1.MicrosoftEntraAuth_Enabled
	pwDisabled := dbforpostgresqlv1.AuthConfig_PasswordAuth_Disabled

	authConfig := &dbforpostgresqlv1.AuthConfig{
		ActiveDirectoryAuth: &adEnabled,
		PasswordAuth:        &pwDisabled,
		TenantId:            to.Ptr(r.Config.TenantId),
	}

	// 5) Build server
	desiredSpec := dbforpostgresqlv1.FlexibleServer_Spec{
		AzureName: serverAzureName,
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
			disDatabaseNamePrefix: db.Name,
		},

		AuthConfig: authConfig,
	}

	desiredLabels := map[string]string{
		databaseServerNameLabelKey: db.Name,
	}

	// Create when missing
	if !found {
		server := &dbforpostgresqlv1.FlexibleServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      k8sName,
				Namespace: ns,
				Labels:    desiredLabels,
			},
			Spec: desiredSpec,
		}

		if err := controllerutil.SetControllerReference(db, server, r.Scheme); err != nil {
			return fmt.Errorf("set controller reference on FlexibleServer: %w", err)
		}

		logger.Info("creating PostgreSQL FlexibleServer for database server",
			"k8sName", k8sName,
			"azureName", serverAzureName,
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
			return fmt.Errorf("create FlexibleServer %s/%s: %w", ns, k8sName, err)
		}
		return nil
	}

	var updated bool
	existing.Labels, updated = k8sutil.SyncSpecAndLabels(&existing.Spec, desiredSpec, existing.Labels, desiredLabels)
	if updated {
		logger.Info("updating PostgreSQL FlexibleServer to match database server",
			"k8sName", k8sName,
			"azureName", serverAzureName,
			"namespace", ns,
		)
		if err := r.Update(ctx, &existing); err != nil {
			return fmt.Errorf("update FlexibleServer %s/%s: %w", ns, k8sName, err)
		}
	}

	return nil
}
