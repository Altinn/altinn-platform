package database

// MaxLogicalDatabaseNameLength is the PostgreSQL identifier limit used by Azure
// PostgreSQL Flexible Server database names.
const MaxLogicalDatabaseNameLength = 63

// LogicalDatabaseName returns the operator's exact database name from spec.name.
func LogicalDatabaseName(name string) string {
	return name
}
