package database

import (
	"strings"

	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
)

const maxLogicalDatabaseNameLength = 63

// DeriveLogicalDatabaseName builds a stable PostgreSQL database name from the
// LogicalDatabase spec fields that identify the tenant and purpose.
func DeriveLogicalDatabaseName(tenantID, tenantEnvironment, databaseKey string) string {
	source := strings.Join([]string{
		strings.TrimSpace(tenantID),
		strings.TrimSpace(tenantEnvironment),
		strings.TrimSpace(databaseKey),
	}, "-")

	sanitized := naming.EnsureLowerAlphaPrefix(naming.SanitizeLowerHyphen(source), "db")
	if len(sanitized) <= maxLogicalDatabaseNameLength {
		return sanitized
	}

	hash := naming.StableSHA256Hex(sanitized)[:8]
	return naming.WithHashSuffixOnOverflow(sanitized, maxLogicalDatabaseNameLength, hash, "db")
}
