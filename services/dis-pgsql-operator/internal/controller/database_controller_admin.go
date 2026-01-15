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
	to "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	genruntime "github.com/Azure/azure-service-operator/v2/pkg/genruntime"
)

func (r *DatabaseReconciler) ensureFlexibleServerAdministrator(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
) error {
	ns := db.Namespace

	adminName := fmt.Sprintf("%s-admin", db.Name)

	key := types.NamespacedName{Name: adminName, Namespace: ns}
	var existing dbforpostgresqlv1.FlexibleServersAdministrator
	if err := r.Get(ctx, key, &existing); err == nil {
		return nil
	} else if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get FlexibleServersAdministrator %s/%s: %w", ns, adminName, err)
	}

	// TODO: adminAppIdentity is the Entra principal OBJECT ID (GUID).
	// As of now we can't pass just the name of a managed identity
	// due to https://github.com/Azure/azure-service-operator/issues/5035
	principalID := db.Spec.Auth.AdminAppIdentity
	if principalID == "" {
		return fmt.Errorf("spec.auth.adminAppIdentity must be set (Entra principal object id)")
	}
	if r.Config.TenantId == "" {
		return fmt.Errorf("TenantID is not configured")
	}

	pt := dbforpostgresqlv1.AdministratorMicrosoftEntraPropertiesForAdd_PrincipalType_ServicePrincipal

	admin := &dbforpostgresqlv1.FlexibleServersAdministrator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      adminName,
			Namespace: ns,
			Labels: map[string]string{
				"dis.altinn.cloud/database-name": db.Name,
			},
		},
		Spec: dbforpostgresqlv1.FlexibleServersAdministrator_Spec{
			// AzureName is the principal object id
			AzureName: principalID,

			Owner: &genruntime.KnownResourceReference{
				// Owner is the FlexibleServer k8s object name
				Name: db.Name,
			},

			PrincipalName: to.Ptr(principalID),
			PrincipalType: &pt,
			TenantId:      to.Ptr(r.Config.TenantId),
		},
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
