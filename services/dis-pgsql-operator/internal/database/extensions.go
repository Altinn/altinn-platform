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
var allowedDatabaseServerExtensions = map[storagev1alpha1.DatabaseServerExtension]struct{}{
	storagev1alpha1.DatabaseServerExtensionHstore:           {},
	storagev1alpha1.DatabaseServerExtensionPgCron:           {},
	storagev1alpha1.DatabaseServerExtensionPgStatStatements: {},
	storagev1alpha1.DatabaseServerExtensionPgAudit:          {},
	storagev1alpha1.DatabaseServerExtensionUUIDOSSP:         {},
}

var extensionSharedPreloadLibraries = map[storagev1alpha1.DatabaseServerExtension]string{
	storagev1alpha1.DatabaseServerExtensionPgCron:           "pg_cron",
	storagev1alpha1.DatabaseServerExtensionPgStatStatements: "pg_stat_statements",
	storagev1alpha1.DatabaseServerExtensionPgAudit:          "pgaudit",
}

// ResolveExtensionSettings validates and normalizes extension settings into
// Azure configuration values for azure.extensions and shared_preload_libraries.
func ResolveExtensionSettings(
	extensions []storagev1alpha1.DatabaseServerExtension,
) (string, string, error) {
	enabledExtensions := make(map[string]struct{}, len(extensions))
	preloadLibraries := make(map[string]struct{})

	for i := range extensions {
		extensionName := strings.TrimSpace(string(extensions[i]))
		if extensionName == "" {
			return "", "", fmt.Errorf("extension value must not be empty")
		}

		extension := storagev1alpha1.DatabaseServerExtension(extensionName)
		if _, ok := allowedDatabaseServerExtensions[extension]; !ok {
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
