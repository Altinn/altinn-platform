package controller

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	azureExtensionsConfigName          = dbUtil.ServerParameterAzureExtensions
	sharedPreloadLibrariesConfigName   = dbUtil.ServerParameterSharedPreloadLibraries
	configSourceUserOverride           = "user-override"
	databaseNameLabelKey               = "dis.altinn.cloud/database-name"
	configurationKindLabelKey          = "dis.altinn.cloud/configuration-kind"
	configurationKindServerParameter   = "server-parameter"
	serverParametersReadyConditionType = "ServerParametersReady"
)

func extensionsConfigResourceName(dbName string) string {
	return fmt.Sprintf("%s-extensions", dbName)
}

func sharedPreloadLibrariesConfigResourceName(dbName string) string {
	return fmt.Sprintf("%s-shared-preload-libraries", dbName)
}

func serverParameterConfigResourceName(dbName, parameterName string) string {
	const maxResourceNameLen = 253

	hash := sha1.Sum([]byte(parameterName))
	suffix := fmt.Sprintf("-server-param-%x", hash[:5])
	maxBaseNameLen := maxResourceNameLen - len(suffix)

	baseName := dbName
	if len(baseName) > maxBaseNameLen {
		baseName = baseName[:maxBaseNameLen]
	}

	return baseName + suffix
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

func (r *DatabaseReconciler) ensurePostgresServerParameters(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
) error {
	serverParameters, err := dbUtil.ResolveServerParameters(db.Spec.ServerType, db.Spec.ServerParams)
	if err != nil {
		return fmt.Errorf("resolve server parameters: %w", err)
	}

	extraLabels := map[string]string{
		configurationKindLabelKey: configurationKindServerParameter,
	}

	desiredResources := make(map[string]string, len(serverParameters))
	for i := range serverParameters {
		parameter := serverParameters[i]
		resourceName := serverParameterConfigResourceName(db.Name, parameter.Name)
		desiredResources[resourceName] = parameter.Name

		if err := r.ensureFlexibleServerConfigurationWithLabels(
			ctx,
			logger,
			db,
			resourceName,
			parameter.Name,
			parameter.Value,
			extraLabels,
		); err != nil {
			return fmt.Errorf("ensure %q configuration: %w", parameter.Name, err)
		}
	}

	if err := r.clearOwnedManagedServerParameterConfigurations(ctx, logger, db, desiredResources); err != nil {
		return err
	}

	return r.updateServerParameterStatusFromASO(ctx, db, desiredResources)
}

func (r *DatabaseReconciler) clearOwnedManagedServerParameterConfigurations(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
	desiredResources map[string]string,
) error {
	var configurations dbforpostgresqlv1.FlexibleServersConfigurationList
	if err := r.List(
		ctx,
		&configurations,
		client.InNamespace(db.Namespace),
		client.MatchingLabels(map[string]string{
			databaseNameLabelKey:      db.Name,
			configurationKindLabelKey: configurationKindServerParameter,
		}),
	); err != nil {
		return fmt.Errorf("list FlexibleServersConfiguration resources for server parameters: %w", err)
	}

	for i := range configurations.Items {
		configuration := &configurations.Items[i]
		if !metav1.IsControlledBy(configuration, db) {
			logger.Info(
				"skipping server parameter configuration that is not controlled by Database",
				"configurationName", configuration.Name,
				"namespace", db.Namespace,
			)
			continue
		}

		if _, ok := desiredResources[configuration.Name]; ok {
			continue
		}

		logger.Info("deleting stale server parameter configuration",
			"configurationName", configuration.Name,
			"namespace", db.Namespace,
			"parameter", configuration.Spec.AzureName,
		)
		if err := r.Delete(ctx, configuration); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete stale FlexibleServersConfiguration %s/%s: %w", db.Namespace, configuration.Name, err)
		}
	}

	return nil
}

func (r *DatabaseReconciler) updateServerParameterStatusFromASO(
	ctx context.Context,
	db *storagev1alpha1.Database,
	desiredResources map[string]string,
) error {
	previousStatus := db.Status.DeepCopy()
	serverParameterErrors := make([]storagev1alpha1.DatabaseServerParameterError, 0)
	pending := false

	for resourceName, parameterName := range desiredResources {
		var configuration dbforpostgresqlv1.FlexibleServersConfiguration
		if err := r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: db.Namespace}, &configuration); err != nil {
			if apierrors.IsNotFound(err) {
				pending = true
				continue
			}
			return fmt.Errorf("get FlexibleServersConfiguration %s/%s: %w", db.Namespace, resourceName, err)
		}

		conditionStatus, reason, message, hasReady := readyConditionInfo(configuration.Status.Conditions)
		if !hasReady || conditionStatus == metav1.ConditionUnknown {
			pending = true
			continue
		}

		if conditionStatus == metav1.ConditionTrue {
			continue
		}

		serverParameterErrors = append(serverParameterErrors, storagev1alpha1.DatabaseServerParameterError{
			Name:    parameterName,
			Reason:  reason,
			Message: message,
		})
	}

	// Keep status output deterministic across reconciles to avoid noisy updates from map iteration order.
	sort.Slice(serverParameterErrors, func(i, j int) bool {
		return serverParameterErrors[i].Name < serverParameterErrors[j].Name
	})

	db.Status.ServerParameterErrors = serverParameterErrors
	switch {
	case len(serverParameterErrors) > 0:
		meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
			Type:    serverParametersReadyConditionType,
			Status:  metav1.ConditionFalse,
			Reason:  "ApplyFailed",
			Message: summarizeServerParameterErrors(serverParameterErrors),
		})
	case pending:
		meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
			Type:    serverParametersReadyConditionType,
			Status:  metav1.ConditionFalse,
			Reason:  "Reconciling",
			Message: "Waiting for server parameter configurations to become ready.",
		})
	default:
		meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
			Type:    serverParametersReadyConditionType,
			Status:  metav1.ConditionTrue,
			Reason:  "Ready",
			Message: "All server parameter configurations are ready.",
		})
	}

	if !equality.Semantic.DeepEqual(previousStatus, &db.Status) {
		if err := r.Status().Update(ctx, db); err != nil {
			return fmt.Errorf("update Database status with server parameter results: %w", err)
		}
	}

	if len(serverParameterErrors) > 0 {
		return fmt.Errorf("one or more server parameters failed to apply; see status.serverParameterErrors")
	}

	return nil
}

func summarizeServerParameterErrors(errors []storagev1alpha1.DatabaseServerParameterError) string {
	// Keep the condition message compact; full details remain in status.serverParameterErrors.
	const maxErrorsInSummary = 3
	if len(errors) == 0 {
		return ""
	}

	summary := make([]string, 0, maxErrorsInSummary)
	for i := range errors {
		if i == maxErrorsInSummary {
			break
		}

		msg := errors[i].Message
		if msg == "" {
			msg = "configuration rejected by Azure"
		}
		summary = append(summary, fmt.Sprintf("%s: %s", errors[i].Name, msg))
	}

	if len(errors) > maxErrorsInSummary {
		// Indicate truncation while preserving how many additional failures exist.
		return fmt.Sprintf("%s (and %d more)", strings.Join(summary, "; "), len(errors)-maxErrorsInSummary)
	}

	return strings.Join(summary, "; ")
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
	return r.ensureFlexibleServerConfigurationWithLabels(
		ctx,
		logger,
		db,
		resourceName,
		parameterName,
		value,
		nil,
	)
}

// ensureFlexibleServerConfigurationWithLabels is the generalized implementation.
// ensureFlexibleServerConfiguration is kept as a convenience wrapper for call sites
// that do not need additional labels.
func (r *DatabaseReconciler) ensureFlexibleServerConfigurationWithLabels(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
	resourceName string,
	parameterName string,
	value string,
	extraLabels map[string]string,
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
		desiredSpec, desiredLabels := desiredFlexibleServerConfiguration(db, parameterName, value, extraLabels)

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

	return r.updateFlexibleServerConfigurationWithLabels(ctx, logger, db, &existing, parameterName, value, extraLabels)
}

func desiredFlexibleServerConfiguration(
	db *storagev1alpha1.Database,
	parameterName string,
	value string,
	extraLabels map[string]string,
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
	for key, labelValue := range extraLabels {
		desiredLabels[key] = labelValue
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
	return r.updateFlexibleServerConfigurationWithLabels(
		ctx,
		logger,
		db,
		configuration,
		parameterName,
		value,
		nil,
	)
}

func (r *DatabaseReconciler) updateFlexibleServerConfigurationWithLabels(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
	configuration *dbforpostgresqlv1.FlexibleServersConfiguration,
	parameterName string,
	value string,
	extraLabels map[string]string,
) error {
	desiredSpec, desiredLabels := desiredFlexibleServerConfiguration(db, parameterName, value, extraLabels)

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
