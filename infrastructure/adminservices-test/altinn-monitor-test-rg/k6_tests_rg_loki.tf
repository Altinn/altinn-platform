resource "azurerm_storage_account" "loki" {
  name                            = "k6clusterlokisa"
  resource_group_name             = azurerm_resource_group.k6tests_rg.name
  location                        = azurerm_resource_group.k6tests_rg.location
  account_tier                    = "Standard"
  account_replication_type        = "LRS"
  allow_nested_items_to_be_public = false
}

resource "azurerm_storage_container" "loki_chunks" {
  name                  = "k6cluster-loki-chunks"
  storage_account_id    = azurerm_storage_account.loki.id
  container_access_type = "private"
}

resource "azurerm_storage_container" "loki_ruler" {
  name                  = "k6cluster-loki-ruler"
  storage_account_id    = azurerm_storage_account.loki.id
  container_access_type = "private"
}

resource "azuread_application" "loki" {
  display_name     = "adminservicestest-k6tests-loki"
  sign_in_audience = "AzureADMyOrg"
}

resource "azuread_service_principal" "loki" {
  client_id = azuread_application.loki.client_id
}

resource "azuread_application_federated_identity_credential" "loki" {
  application_id = azuread_application.loki.id
  display_name   = "adminservicestest-k6tests-loki"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = azurerm_kubernetes_cluster.k6tests.oidc_issuer_url
  subject        = "system:serviceaccount:monitoring:loki"
}

resource "azurerm_role_assignment" "storage_blob_data_contributor" {
  scope                = azurerm_storage_account.loki.id
  role_definition_name = "Storage Blob Data Contributor"
  principal_id         = azuread_service_principal.loki.object_id
}

resource "helm_release" "loki" {
  depends_on = [
    helm_release.kube_prometheus_stack,
    azuread_application.loki,
  ]
  lint             = true
  name             = "loki"
  namespace        = "monitoring"
  create_namespace = false
  repository       = "https://grafana.github.io/helm-charts"
  chart            = "loki-distributed"
  version          = "0.80.6"

  values = [
    "${templatefile(
      "${path.module}/k6_tests_rg_loki_values.tftpl",
      {
        account_name          = "${azurerm_storage_account.loki.name}",
        chunks_container_name = "${azurerm_storage_container.loki_chunks.name}",
        ruler_container_name  = "${azurerm_storage_container.loki_ruler.name}",
        client_id             = "${azuread_application.loki.client_id}",
      }
    )}"
  ]
}
