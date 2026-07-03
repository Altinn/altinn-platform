package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	k8sutil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/k8s"
	networkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20240601"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
)

// The suffix we use for per-database-server private DNS zones.
const postgresPrivateZoneSuffix = "private.postgres.database.azure.com"

// Private DNS zones (and their vnet links) use 'global' for Location in Azure.
const privateDNSZoneLocation = "global"

// zoneCRNameForDatabaseServer is the Kubernetes object name of the server's
// Private DNS zone CR. It stays equal to the DatabaseServer CR name (which is a
// valid flexible-server name, so <=63 chars) rather than the FQDN. ASO stamps a
// serviceoperator.azure.com/owner-name=<owner CR name> label on child resources
// (the vnet links), truncated to the 63-char label limit. Using the FQDN as the
// owner name made that label exceed 63 chars for longer server names, ending in
// a '.' and failing label validation, which blocked the vnet links entirely.
func zoneCRNameForDatabaseServer(db *storagev1alpha1.DatabaseServer) string {
	return db.Name
}

// resolveZoneCRName returns the zone CR name to use for this server. Servers
// created before the zone CR was renamed to db.Name own a zone CR named after
// the Azure FQDN, with vnet links owned by it; ASO forbids changing a link's
// owner, and creating a second zone CR would double-manage the same Azure
// zone, so the legacy name is preserved for as long as that CR exists.
func (r *DatabaseServerReconciler) resolveZoneCRName(
	ctx context.Context,
	db *storagev1alpha1.DatabaseServer,
) (string, error) {
	legacyName := zoneAzureNameForDatabaseServer(db)
	var legacy networkv1.PrivateDnsZone
	err := r.Get(ctx, types.NamespacedName{Name: legacyName, Namespace: db.Namespace}, &legacy)
	if err == nil {
		return legacyName, nil
	}
	if !errors.IsNotFound(err) {
		return "", fmt.Errorf("get legacy PrivateDnsZone %s/%s: %w", db.Namespace, legacyName, err)
	}
	return zoneCRNameForDatabaseServer(db), nil
}

// zoneAzureNameForDatabaseServer is the Azure-side name of the server's Private
// DNS zone (used only for Spec.AzureName) — the FQDN PostgreSQL Flexible Server
// requires.
func zoneAzureNameForDatabaseServer(db *storagev1alpha1.DatabaseServer) string {
	return fmt.Sprintf("%s.%s", db.Name, postgresPrivateZoneSuffix)
}

func dbVNetLinkNameForDatabaseServer(db *storagev1alpha1.DatabaseServer) string {
	return fmt.Sprintf("%s-vnetlink", db.Name)
}

func aksVNetLinkNameForDatabaseServer(db *storagev1alpha1.DatabaseServer) string {
	return fmt.Sprintf("%s-aks-vnetlink", db.Name)
}

// ensurePrivateDNSZone ensures that a Private DNS Zone exists for the given database server.
func (r *DatabaseServerReconciler) ensurePrivateDNSZone(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
	zoneCRName string,
) error {
	if r.Config.ResourceGroup == "" {
		return fmt.Errorf("ResourceGroup is not configured on DatabaseServerReconciler")
	}
	ns := db.Namespace
	zoneAzureName := zoneAzureNameForDatabaseServer(db)
	key := types.NamespacedName{
		Name:      zoneCRName,
		Namespace: ns,
	}

	var existing networkv1.PrivateDnsZone
	err := r.Get(ctx, key, &existing)
	if err == nil {
		logger.Info("private DNS zone already exists for database server",
			"zoneCRName", zoneCRName,
			"azureName", zoneAzureName,
			"asoNamespace", ns)
		return nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get PrivateDnsZone %s/%s: %w", key.Namespace, key.Name, err)
	}

	logger.Info("creating private DNS zone for database server",
		"zoneCRName", zoneCRName,
		"azureName", zoneAzureName,
		"asoNamespace", ns)

	loc := privateDNSZoneLocation

	zone := &networkv1.PrivateDnsZone{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zoneCRName,
			Namespace: ns,
			Labels: map[string]string{
				databaseServerNameLabelKey: db.Name,
			},
		},
		Spec: networkv1.PrivateDnsZone_Spec{
			AzureName: zoneAzureName,
			Location:  &loc,
			Owner: &genruntime.KnownResourceReference{
				ARMID: fmt.Sprintf(
					"/subscriptions/%s/resourceGroups/%s",
					r.Config.SubscriptionId,
					r.Config.ResourceGroup,
				),
			},
			Tags: map[string]string{
				disDatabaseNamePrefix: db.Name,
			},
		},
	}

	if err := controllerutil.SetControllerReference(db, zone, r.Scheme); err != nil {
		return fmt.Errorf("set controller reference on PrivateDnsZone: %w", err)
	}

	if err := r.Create(ctx, zone); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("private DNS zone already created by another reconcile",
				"zoneCRName", zoneCRName,
				"azureName", zoneAzureName,
				"asoNamespace", ns)
			return nil
		}
		return fmt.Errorf("create PrivateDnsZone %s/%s: %w", zone.Namespace, zone.Name, err)
	}

	return nil
}

// ensurePrivateDNSVNetLink ensures a Private DNS virtual network link exists
// for the given database server.
func (r *DatabaseServerReconciler) ensurePrivateDNSVNetLink(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
	zoneCRName string,
	linkName string,
	targetVNetName string,
	vnetID string,
) error {
	ns := db.Namespace
	loc := privateDNSZoneLocation
	regFalse := false
	desiredLabels := map[string]string{
		databaseServerNameLabelKey: db.Name,
	}
	desiredSpec := networkv1.PrivateDnsZonesVirtualNetworkLink_Spec{
		AzureName: linkName,
		Location:  &loc,

		// REQUIRED: owner is the PrivateDnsZone CR
		Owner: &genruntime.KnownResourceReference{
			Name: zoneCRName,
		},

		RegistrationEnabled: &regFalse,

		VirtualNetwork: &networkv1.SubResource{
			Reference: &genruntime.ResourceReference{
				ARMID: vnetID,
			},
		},

		Tags: map[string]string{
			disDatabaseNamePrefix: db.Name,
		},
	}

	key := types.NamespacedName{
		Name:      linkName,
		Namespace: ns,
	}

	var existing networkv1.PrivateDnsZonesVirtualNetworkLink
	if err := r.Get(ctx, key, &existing); err == nil {
		var updated bool
		existing.Labels, updated = k8sutil.SyncSpecAndLabels(
			&existing.Spec,
			desiredSpec,
			existing.Labels,
			desiredLabels,
		)
		if updated {
			logger.Info("updating private DNS VNet link",
				"linkName", existing.Name,
				"zoneCRName", zoneCRName,
				"vnetName", targetVNetName,
				"vnetID", vnetID,
			)
			if err := r.Update(ctx, &existing); err != nil {
				return fmt.Errorf("update PrivateDnsZonesVirtualNetworkLink %s/%s: %w", existing.Namespace, existing.Name, err)
			}
		}
		return nil
	} else if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get PrivateDnsZonesVirtualNetworkLink %s/%s: %w", key.Namespace, key.Name, err)
	}

	link := &networkv1.PrivateDnsZonesVirtualNetworkLink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      linkName,
			Namespace: ns,
			Labels:    desiredLabels,
		},
		Spec: desiredSpec,
	}

	logger.Info("creating private DNS VNet link",
		"linkName", linkName,
		"zoneCRName", zoneCRName,
		"vnetName", targetVNetName,
		"vnetID", vnetID,
	)

	if err := controllerutil.SetControllerReference(db, link, r.Scheme); err != nil {
		return fmt.Errorf("set controller reference on VNetLink: %w", err)
	}

	if err := r.Create(ctx, link); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create PrivateDnsZonesVirtualNetworkLink %s/%s: %w", link.Namespace, link.Name, err)
	}

	return nil
}

// privateZoneARMIDResourceReference builds the resource reference for the server's
// Private DNS zone. Name is a Kubernetes cross-resource reference (the zone CR
// name), not the Azure FQDN.
func (r *DatabaseServerReconciler) privateZoneARMIDResourceReference(zoneCRName string) *genruntime.ResourceReference {
	return &genruntime.ResourceReference{
		Group: "network.azure.com",
		Kind:  "PrivateDnsZone",
		Name:  zoneCRName,
	}
}
