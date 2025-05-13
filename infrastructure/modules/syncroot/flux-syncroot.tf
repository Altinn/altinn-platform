resource "azapi_resource" "flux_syncroot" {
  type      = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name      = "syncroot-${var.team_name}-${var.environment}"
  parent_id = var.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        syncroot = {
          force = false
          path  = "./${var.environment}"
          postBuild = {
            substitute = {
              DISABLE_IPV6 = "false"
            }
          }
          prune                  = false
          retryIntervalInSeconds = 300
          syncIntervalInSeconds  = 300
          timeoutInSeconds       = 600
          wait                   = true
        }
      }
      ociRepository = {
        insecure = false
        repositoryRef = {
          tag = var.environment
        }
        syncIntervalInSeconds = 300
        timeoutInSeconds      = 300
        url                   = "oci://altinncr.azurecr.io/${var.team_name}/syncroot"
        useWorkloadIdentity   = true
      }
      namespace                  = var.namespace != "" ? var.namespace : var.team_name
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
