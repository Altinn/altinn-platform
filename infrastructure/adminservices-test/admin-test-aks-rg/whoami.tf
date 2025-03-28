resource "azapi_resource" "whoami" {
  depends_on = [azurerm_kubernetes_cluster_extension.flux_ext]
  type       = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name       = "whoami"
  parent_id  = azurerm_kubernetes_cluster.aks.id
  body = {
    properties = {
      kustomizations = {
        whoami = {
          force                  = false
          path                   = "./"
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
        url                   = "oci://altinncr.azurecr.io/manifests/infra/whoami"
        useWorkloadIdentity   = true
      }
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
