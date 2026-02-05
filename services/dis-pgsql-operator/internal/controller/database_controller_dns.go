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
	networkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20240601"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
)

// The suffix we use for per-DB private DNS zones
const postgresPrivateZoneSuffix = "private.postgres.database.azure.com"

// Helper func to compute the Private DNS zone name for a DB
func zoneNameForDatabase(db *storagev1alpha1.Database) string {
	return fmt.Sprintf("%s.%s", db.Name, postgresPrivateZoneSuffix)
}

func vnetLinkNameForDB(db *storagev1alpha1.Database) string {
	return fmt.Sprintf("%s-vnetlink", db.Name)
}

func vnetLinkNameForAKS(db *storagev1alpha1.Database) string {
	return fmt.Sprintf("%s-aks-vnetlink", db.Name)
}

// ensurePrivateDNSZone ensures that a Private Dns Zone exists for the given Database.
func (r *DatabaseReconciler) ensurePrivateDNSZone(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
) error {
	if r.Config.ResourceGroup == "" {
		return fmt.Errorf("ResourceGroup is not configured on DatabaseReconciler")
	}
	ns := db.Namespace
	zoneName := zoneNameForDatabase(db)
	key := types.NamespacedName{
		Name:      zoneName,
		Namespace: ns,
	}

	var existing networkv1.PrivateDnsZone
	err := r.Get(ctx, key, &existing)
	if err == nil {
		logger.Info("private DNS zone already exists for database",
			"zoneName", zoneName,
			"asoNamespace", ns)
		return nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get PrivateDnsZone %s/%s: %w", key.Namespace, key.Name, err)
	}

	logger.Info("creating private DNS zone for database",
		"zoneName", zoneName,
		"asoNamespace", ns)

	loc := "global" // Private DNS zones use 'global' for Location in Azure.

	zone := &networkv1.PrivateDnsZone{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zoneName,
			Namespace: ns,
			Labels: map[string]string{
				"dis.altinn.cloud/database-name": db.Name,
			},
		},
		Spec: networkv1.PrivateDnsZone_Spec{
			AzureName: zoneName,
			Location:  &loc,
			Owner: &genruntime.KnownResourceReference{
				ARMID: fmt.Sprintf(
					"/subscriptions/%s/resourceGroups/%s",
					r.Config.SubscriptionId,
					r.Config.ResourceGroup,
				),
			},
			Tags: map[string]string{
				"dis-database": db.Name,
			},
		},
	}

	if err := controllerutil.SetControllerReference(db, zone, r.Scheme); err != nil {
		return fmt.Errorf("set controller reference on PrivateDnsZone: %w", err)
	}

	if err := r.Create(ctx, zone); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("private DNS zone already created by another reconcile",
				"zoneName", zoneName,
				"asoNamespace", ns)
			return nil
		}
		return fmt.Errorf("create PrivateDnsZone %s/%s: %w", zone.Namespace, zone.Name, err)
	}

	return nil
}

// ensurePrivateDNSVNetLink ensures a Private DNS virtual network link exists
// for the given Database.
func (r *DatabaseReconciler) ensurePrivateDNSVNetLink(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
	zoneName string,
	linkName string,
	targetVNetName string,
	vnetID string,
) error {
	ns := db.Namespace

	key := types.NamespacedName{
		Name:      linkName,
		Namespace: ns,
	}

	var existing networkv1.PrivateDnsZonesVirtualNetworkLink
	if err := r.Get(ctx, key, &existing); err == nil {
		return nil
	} else if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get PrivateDnsZonesVirtualNetworkLink %s/%s: %w", key.Namespace, key.Name, err)
	}

	loc := "global"
	regFalse := false

	link := &networkv1.PrivateDnsZonesVirtualNetworkLink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      linkName,
			Namespace: ns,
			Labels: map[string]string{
				"dis.altinn.cloud/database-name": db.Name,
			},
		},
		Spec: networkv1.PrivateDnsZonesVirtualNetworkLink_Spec{
			AzureName: linkName,
			Location:  &loc,

			// REQUIRED: owner is the PrivateDnsZone CR
			Owner: &genruntime.KnownResourceReference{
				Name: zoneName,
			},

			RegistrationEnabled: &regFalse,

			VirtualNetwork: &networkv1.SubResource{
				Reference: &genruntime.ResourceReference{
					ARMID: vnetID,
				},
			},

			Tags: map[string]string{
				"dis-database": db.Name,
			},
		},
	}

	logger.Info("creating private DNS VNet link",
		"linkName", linkName,
		"zoneName", zoneName,
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

// privateZoneARMID builds the ARM ID for the per-DB Private DNS zone.
func (r *DatabaseReconciler) privateZoneARMIDResourceReference(zoneName string) *genruntime.ResourceReference {
	return &genruntime.ResourceReference{
		Group: "network.azure.com",
		Kind:  "PrivateDnsZone",
		Name:  zoneName,
	}
}
