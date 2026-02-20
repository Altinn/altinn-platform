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
	databaseNameLabelKey             = "dis.altinn.cloud/database-name"
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
	if db.Spec.EnableExtensions == nil {
		return r.clearOwnedManagedExtensionConfigurations(ctx, logger, db)
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

func (r *DatabaseReconciler) clearOwnedManagedExtensionConfigurations(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
) error {
	// Reconcile only the two extension configs this controller owns, identified by
	// deterministic names derived from the Database name. We intentionally do not
	// iterate all FlexibleServersConfiguration objects in the namespace.
	candidates := []struct {
		resourceName  string
		parameterName string
	}{
		{
			resourceName:  extensionsConfigResourceName(db.Name),
			parameterName: azureExtensionsConfigName,
		},
		{
			resourceName:  sharedPreloadLibrariesConfigResourceName(db.Name),
			parameterName: sharedPreloadLibrariesConfigName,
		},
	}

	for _, candidate := range candidates {
		key := types.NamespacedName{Name: candidate.resourceName, Namespace: db.Namespace}
		var configuration dbforpostgresqlv1.FlexibleServersConfiguration
		if err := r.Get(ctx, key, &configuration); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("get FlexibleServersConfiguration %s/%s: %w", db.Namespace, candidate.resourceName, err)
		}

		if !metav1.IsControlledBy(&configuration, db) {
			logger.Info(
				"skipping extension configuration that is not controlled by Database",
				"configurationName", candidate.resourceName,
				"namespace", db.Namespace,
			)
			continue
		}

		if err := r.updateFlexibleServerConfiguration(
			ctx,
			logger,
			db,
			&configuration,
			candidate.parameterName,
			"",
		); err != nil {
			return fmt.Errorf(
				"clear managed extension configuration %s/%s: %w",
				db.Namespace,
				candidate.resourceName,
				err,
			)
		}
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

	if !found {
		desiredSpec, desiredLabels := desiredFlexibleServerConfiguration(db, parameterName, value)

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

	return r.updateFlexibleServerConfiguration(ctx, logger, db, &existing, parameterName, value)
}

func desiredFlexibleServerConfiguration(
	db *storagev1alpha1.Database,
	parameterName string,
	value string,
) (dbforpostgresqlv1.FlexibleServersConfiguration_Spec, map[string]string) {
	desiredSpec := dbforpostgresqlv1.FlexibleServersConfiguration_Spec{
		AzureName: parameterName,
		Owner: &genruntime.KnownResourceReference{
			Name: db.Name,
		},
		Source: to.Ptr(configSourceUserOverride),
		Value:  to.Ptr(value),
	}

	desiredLabels := map[string]string{
		databaseNameLabelKey: db.Name,
	}

	return desiredSpec, desiredLabels
}

func (r *DatabaseReconciler) updateFlexibleServerConfiguration(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
	configuration *dbforpostgresqlv1.FlexibleServersConfiguration,
	parameterName string,
	value string,
) error {
	desiredSpec, desiredLabels := desiredFlexibleServerConfiguration(db, parameterName, value)

	var updated bool
	configuration.Labels, updated = k8sutil.SyncSpecAndLabels(
		&configuration.Spec,
		desiredSpec,
		configuration.Labels,
		desiredLabels,
	)

	if !updated {
		return nil
	}

	logger.Info("updating FlexibleServersConfiguration to match Database",
		"configurationName", configuration.Name,
		"namespace", db.Namespace,
		"parameter", parameterName,
		"value", value,
	)
	if err := r.Update(ctx, configuration); err != nil {
		return fmt.Errorf("update FlexibleServersConfiguration %s/%s: %w", db.Namespace, configuration.Name, err)
	}

	return nil
}
