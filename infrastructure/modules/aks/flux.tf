resource "azurerm_kubernetes_cluster_extension" "flux" {
  name           = "flux"
  cluster_id     = azurerm_kubernetes_cluster.aks.id
  extension_type = "microsoft.flux"
  configuration_settings = {
    "useKubeletIdentity"      = "true"
    "autoUpgradeMinorVersion" = "true"
    "multiTenancy.enforce"    = var.enable_multi_tenancy
  }
}
