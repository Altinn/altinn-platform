resource "azapi_resource" "blackbox_exporter" {
  depends_on = [module.aks_resources]
  type       = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name       = "blackbox-exporter"
  parent_id  = module.aks.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        blackbox-exporter = {
          force                  = false
          path                   = "./base/"
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
          tag = var.flux_release_tag
        }
        syncIntervalInSeconds = 300
        timeoutInSeconds      = 300
        url                   = "oci://altinncr.azurecr.io/manifests/infra/blackbox-exporter"
        useWorkloadIdentity   = true
      }
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
