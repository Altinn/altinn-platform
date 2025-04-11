package azure

// DiagnosticsType Internal enum that holds the allowed values for diagnosticsId in the APIM api
type DiagnosticsType string

const (
	// DiagnosticsIdAzureMonitor - Azure Monitor diagnostics settings id.
	DiagnosticsIdAzureMonitor DiagnosticsType = "azuremonitor"
	// DiagnosticsIdApplicationInsights - Application Insights diagnostics settings id.
	DiagnosticsIdApplicationInsights DiagnosticsType = "applicationinsights"
)
