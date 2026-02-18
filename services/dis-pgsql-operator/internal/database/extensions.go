package database

import (
	"fmt"
	"sort"
	"strings"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
)

// We are following the docs here:
// https://github.com/MicrosoftDocs/azure-databases-docs/blob/main/articles/postgresql/extensions/includes/extensions-table.md

// Here for getting better reconcile error messages
// before we send a bad parameter to ASO/Azure
var allowedDatabaseExtensions = map[storagev1alpha1.DatabaseExtension]struct{}{
	storagev1alpha1.DatabaseExtensionHstore:           {},
	storagev1alpha1.DatabaseExtensionPgCron:           {},
	storagev1alpha1.DatabaseExtensionPgStatStatements: {},
	storagev1alpha1.DatabaseExtensionPgAudit:          {},
	storagev1alpha1.DatabaseExtensionUUIDOSSP:         {},
}

var extensionSharedPreloadLibraries = map[storagev1alpha1.DatabaseExtension]string{
	storagev1alpha1.DatabaseExtensionPgCron:           "pg_cron",
	storagev1alpha1.DatabaseExtensionPgStatStatements: "pg_stat_statements",
	storagev1alpha1.DatabaseExtensionPgAudit:          "pgaudit",
}

// ResolveExtensionSettings validates and normalizes extension settings into
// Azure configuration values for azure.extensions and shared_preload_libraries.
func ResolveExtensionSettings(
	extensions []storagev1alpha1.DatabaseExtension,
) (string, string, error) {
	enabledExtensions := make(map[string]struct{}, len(extensions))
	preloadLibraries := make(map[string]struct{})

	for i := range extensions {
		extensionName := strings.TrimSpace(string(extensions[i]))
		if extensionName == "" {
			return "", "", fmt.Errorf("extension value must not be empty")
		}

		extension := storagev1alpha1.DatabaseExtension(extensionName)
		if _, ok := allowedDatabaseExtensions[extension]; !ok {
			return "", "", fmt.Errorf("unsupported extension %q", extensionName)
		}

		enabledExtensions[extensionName] = struct{}{}

		if preloadLibrary, ok := extensionSharedPreloadLibraries[extension]; ok {
			preloadLibraries[preloadLibrary] = struct{}{}
		}
	}

	enabled := make([]string, 0, len(enabledExtensions))
	for extension := range enabledExtensions {
		enabled = append(enabled, extension)
	}
	sort.Strings(enabled)

	preload := make([]string, 0, len(preloadLibraries))
	for library := range preloadLibraries {
		preload = append(preload, library)
	}
	sort.Strings(preload)

	return strings.Join(enabled, ","), strings.Join(preload, ","), nil
}
