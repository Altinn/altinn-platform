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
	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
	k8sutil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/k8s"
	to "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	genruntime "github.com/Azure/azure-service-operator/v2/pkg/genruntime"
)

// Taken from the ARM ids
// https://github.com/MicrosoftDocs/azure-databases-docs/blob/main/articles/postgresql/extensions/how-to-allow-extensions.md
const (
	azureExtensionsConfigName        = "azure.extensions"
	sharedPreloadLibrariesConfigName = "shared_preload_libraries"
	configSourceUserOverride         = "user-override"
)

func extensionsConfigResourceName(dbName string) string {
	return fmt.Sprintf("%s-extensions", dbName)
}

func sharedPreloadLibrariesConfigResourceName(dbName string) string {
	return fmt.Sprintf("%s-shared-preload-libraries", dbName)
}

func (r *DatabaseReconciler) ensurePostgresExtensionSettings(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
) error {
	// Treat nil as "not managed yet" for backward compatibility with existing Database resources.
	if db.Spec.EnableExtensions == nil {
		return nil
	}

	extensionsValue, preloadValue, err := dbUtil.ResolveExtensionSettings(db.Spec.EnableExtensions)
	if err != nil {
		return fmt.Errorf("resolve extension settings: %w", err)
	}

	// Enable curated extensions on the server (maps to the azure.extensions parameter).
	if err := r.ensureFlexibleServerConfiguration(
		ctx,
		logger,
		db,
		extensionsConfigResourceName(db.Name),
		azureExtensionsConfigName,
		extensionsValue,
	); err != nil {
		return fmt.Errorf("ensure %q configuration: %w", azureExtensionsConfigName, err)
	}

	// Ensure required shared libraries are preloaded for extensions that need it.
	if err := r.ensureFlexibleServerConfiguration(
		ctx,
		logger,
		db,
		sharedPreloadLibrariesConfigResourceName(db.Name),
		sharedPreloadLibrariesConfigName,
		preloadValue,
	); err != nil {
		return fmt.Errorf("ensure %q configuration: %w", sharedPreloadLibrariesConfigName, err)
	}

	return nil
}

func (r *DatabaseReconciler) ensureFlexibleServerConfiguration(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
	resourceName string,
	parameterName string,
	value string,
) error {
	key := types.NamespacedName{Name: resourceName, Namespace: db.Namespace}

	var existing dbforpostgresqlv1.FlexibleServersConfiguration
	found := true
	if err := r.Get(ctx, key, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			found = false
		} else {
			return fmt.Errorf("get FlexibleServersConfiguration %s/%s: %w", db.Namespace, resourceName, err)
		}
	}

	desiredSpec := dbforpostgresqlv1.FlexibleServersConfiguration_Spec{
		AzureName: parameterName,
		Owner: &genruntime.KnownResourceReference{
			Name: db.Name,
		},
		Source: to.Ptr(configSourceUserOverride),
		Value:  to.Ptr(value),
	}

	desiredLabels := map[string]string{
		"dis.altinn.cloud/database-name": db.Name,
	}

	if !found {
		configuration := &dbforpostgresqlv1.FlexibleServersConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: db.Namespace,
				Labels:    desiredLabels,
			},
			Spec: desiredSpec,
		}

		if err := controllerutil.SetControllerReference(db, configuration, r.Scheme); err != nil {
			return fmt.Errorf("set controller reference on FlexibleServersConfiguration: %w", err)
		}

		logger.Info("creating FlexibleServersConfiguration for database",
			"configurationName", resourceName,
			"namespace", db.Namespace,
			"parameter", parameterName,
			"value", value,
		)

		if err := r.Create(ctx, configuration); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return fmt.Errorf("create FlexibleServersConfiguration %s/%s: %w", db.Namespace, resourceName, err)
		}
		return nil
	}

	var updated bool
	existing.Labels, updated = k8sutil.SyncSpecAndLabels(&existing.Spec, desiredSpec, existing.Labels, desiredLabels)

	if updated {
		logger.Info("updating FlexibleServersConfiguration to match Database",
			"configurationName", resourceName,
			"namespace", db.Namespace,
			"parameter", parameterName,
			"value", value,
		)
		if err := r.Update(ctx, &existing); err != nil {
			return fmt.Errorf("update FlexibleServersConfiguration %s/%s: %w", db.Namespace, resourceName, err)
		}
	}

	return nil
}
