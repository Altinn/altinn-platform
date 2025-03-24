package azure

type DiagnosticsType string

const (
	// DiagnosticsIdAzureMonitor - Azure Monitor diagnostics settings.
	DiagnosticsIdAzureMonitor DiagnosticsType = "azuremonitor"
	// DiagnosticsIdApplicationInsights - Application Insights diagnostics settings.
	DiagnosticsIdApplicationInsights DiagnosticsType = "applicationinsights"
)
