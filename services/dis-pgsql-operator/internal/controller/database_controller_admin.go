package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	k8sutil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/k8s"
	to "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	genruntime "github.com/Azure/azure-service-operator/v2/pkg/genruntime"
)

func (r *DatabaseReconciler) ensureFlexibleServerAdministrator(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
	adminIdentity resolvedAdminIdentity,
) error {
	ns := db.Namespace

	adminName := fmt.Sprintf("%s-admin", db.Name)

	key := types.NamespacedName{Name: adminName, Namespace: ns}
	var existing dbforpostgresqlv1.FlexibleServersAdministrator
	found := true
	if err := r.Get(ctx, key, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			found = false
		} else {
			return fmt.Errorf("get FlexibleServersAdministrator %s/%s: %w", ns, adminName, err)
		}
	}

	// adminAppPrincipalId is the Entra principal OBJECT ID (GUID).
	// This is required by ASO for FlexibleServersAdministrator.
	principalID := adminIdentity.PrincipalID
	if principalID == "" {
		return fmt.Errorf("resolved admin principal ID is empty")
	}
	principalName := adminIdentity.Name
	if principalName == "" {
		return fmt.Errorf("resolved admin principal name is empty")
	}
	if r.Config.TenantId == "" {
		return fmt.Errorf("TenantID is not configured")
	}

	pt := dbforpostgresqlv1.AdministratorMicrosoftEntraPropertiesForAdd_PrincipalType_ServicePrincipal

	desiredSpec := dbforpostgresqlv1.FlexibleServersAdministrator_Spec{
		// AzureName is the principal object id
		AzureName: principalID,

		Owner: &genruntime.KnownResourceReference{
			// Owner is the FlexibleServer k8s object name
			Name: db.Name,
		},

		PrincipalName: to.Ptr(principalName),
		PrincipalType: &pt,
		TenantId:      to.Ptr(r.Config.TenantId),
	}

	desiredLabels := map[string]string{
		"dis.altinn.cloud/database-name": db.Name,
	}

	if !found {
		admin := &dbforpostgresqlv1.FlexibleServersAdministrator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      adminName,
				Namespace: ns,
				Labels:    desiredLabels,
			},
			Spec: desiredSpec,
		}

		if err := controllerutil.SetControllerReference(db, admin, r.Scheme); err != nil {
			return fmt.Errorf("set controller reference on FlexibleServersAdministrator: %w", err)
		}

		logger.Info("creating FlexibleServersAdministrator for database",
			"adminName", adminName,
			"namespace", ns,
			"principalID", principalID,
		)

		if err := r.Create(ctx, admin); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return fmt.Errorf("create FlexibleServersAdministrator %s/%s: %w", ns, adminName, err)
		}
		return nil
	}

	var updated bool
	existing.Labels, updated = k8sutil.SyncSpecAndLabels(&existing.Spec, desiredSpec, existing.Labels, desiredLabels)

	if updated {
		logger.Info("updating FlexibleServersAdministrator to match Database",
			"adminName", adminName,
			"namespace", ns,
			"principalID", principalID,
		)
		if err := r.Update(ctx, &existing); err != nil {
			return fmt.Errorf("update FlexibleServersAdministrator %s/%s: %w", ns, adminName, err)
		}
	}

	return nil
}
