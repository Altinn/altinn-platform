resource "azapi_resource" "traefik" {
  depends_on = [azurerm_kubernetes_cluster_extension.flux_ext]
  type       = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name       = "traefik"
  parent_id  = azurerm_kubernetes_cluster.aks.id
  body = {
    properties = {
      kustomizations = {
        traefik = {
          force = false
          path  = "./"
          postBuild = {
            substitute = {
              AKS_SYSP00L_IP_PREFIX_0 : "${var.subnet_address_prefixes["aks_syspool"][0]}"
              AKS_SYSP00L_IP_PREFIX_1 : "${var.subnet_address_prefixes["aks_syspool"][1]}"
              AKS_WORKPOOL_IP_PREFIX_0 : "${var.subnet_address_prefixes["aks_workpool"][0]}"
              AKS_WORKPOOL_IP_PREFIX_1 : "${var.subnet_address_prefixes["aks_workpool"][1]}"
              AKS_NODE_RG : "${azurerm_kubernetes_cluster.aks.node_resource_group}"
              PUBLIC_IP_V4 : "${azurerm_public_ip.pip4.ip_address}"
              PUBLIC_IP_V6 : "${azurerm_public_ip.pip6.ip_address}"
              # EXTERNAL_TRAFFIC_POLICY: Cluster (Local is default)
            }
          }
          prune                  = false
          retryIntervalInSeconds = 300
          syncIntervalInSeconds  = 300
          timeoutInSeconds       = 300
          wait                   = true
        }
      }
      namespace = "flux-system"
      ociRepository = {
        insecure = false
        repositoryRef = {
          tag = var.flux_release_tag
        }
        syncIntervalInSeconds = 300
        timeoutInSeconds      = 300
        url                   = "oci://altinncr.azurecr.io/manifests/infra/traefik"
        useWorkloadIdentity   = true
      }
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
