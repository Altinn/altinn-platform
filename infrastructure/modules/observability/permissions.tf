resource "azurerm_role_assignment" "otel_collector_metrics_publisher" {
  scope                = local.dcr_id
  principal_id         = azuread_service_principal.sp.object_id
  role_definition_name = "Monitoring Metrics Publisher"
}
