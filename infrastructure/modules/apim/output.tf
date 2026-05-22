output "apim_id" {
  value = azurerm_api_management.apim.id
}

output "apim_service_name" {
  value = azurerm_api_management.apim.name
}

output "apim_rg_name" {
  value = azurerm_resource_group.apim_rg.name
}

output "apim_default_logger_id" {
  value = azurerm_api_management_logger.apimlogger.id
}
