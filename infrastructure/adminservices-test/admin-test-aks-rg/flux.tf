resource "azurerm_kubernetes_cluster_extension" "flux_ext" {
  depends_on     = [azurerm_kubernetes_cluster.aks]
  name           = "flux-ext"
  cluster_id     = azurerm_kubernetes_cluster.aks.id
  extension_type = "microsoft.flux"
  configuration_settings = {
    "useKubeletIdentity"      = "true"
    "autoUpgradeMinorVersion" = "true"
    "multiTenancy.enforce"    = "true"
  }
}
