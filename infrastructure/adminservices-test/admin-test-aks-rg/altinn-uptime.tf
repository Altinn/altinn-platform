resource "azapi_resource" "altinn_uptime" {
  depends_on = [module.aks_resources]
  type       = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name       = "altinn-uptime"
  parent_id  = module.aks.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        altinn-uptime = {
          force                  = false
          path                   = "./"
          prune                  = true
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
          tag = "latest"
        }
        syncIntervalInSeconds = 300
        timeoutInSeconds      = 300
        url                   = "oci://altinncr.azurecr.io/manifests/infra/altinn-uptime"
        useWorkloadIdentity   = true
      }
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
