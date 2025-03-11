resource "azurerm_application_insights" "appinsights" {
  name                = "${var.name_prefix}-appinsights"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  application_type    = "web"
}

resource "azurerm_api_management_logger" "apimlogger" {
  name                = "${var.name_prefix}-apimlogger"
  api_management_name = azurerm_api_management.admin_test_apim.name
  resource_group_name = azurerm_resource_group.rg.name

  application_insights {
    instrumentation_key = azurerm_application_insights.appinsights.instrumentation_key
  }
}

resource "azurerm_api_management_diagnostic" "apimdiagnostic" {
  identifier               = "${var.name_prefix}-apimdiagnostic"
  resource_group_name      = azurerm_resource_group.rg.name
  api_management_name      = azurerm_api_management.admin_test_apim.name
  api_management_logger_id = azurerm_api_management_logger.apimlogger.id

  sampling_percentage       = 0.0
  always_log_errors         = true
  log_client_ip             = true
  verbosity                 = "information"
  http_correlation_protocol = "W3C"

  frontend_request {
    body_bytes = 32
    headers_to_log = [
      "content-type",
      "accept",
      "origin",
    ]
  }

  frontend_response {
    body_bytes = 32
    headers_to_log = [
      "content-type",
      "content-length",
      "origin",
    ]
  }

  backend_request {
    body_bytes = 32
    headers_to_log = [
      "content-type",
      "accept",
      "origin",
    ]
  }

  backend_response {
    body_bytes = 32
    headers_to_log = [
      "content-type",
      "content-length",
      "origin",
    ]
  }
}