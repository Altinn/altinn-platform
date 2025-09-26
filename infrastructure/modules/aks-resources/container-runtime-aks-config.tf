resource "azapi_resource" "container_runtime_aks_config" {
  type      = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name      = "container-runtime-aks-config"
  parent_id = var.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        aks-config-ama = {
          force                  = false
          path                   = "./ama/"
          prune                  = false
          retryIntervalInSeconds = 300
          syncIntervalInSeconds  = 300
          timeoutInSeconds       = 300
          wait                   = true
        },
        aks-config-metrics-server = {
          force                  = false
          path                   = "./metrics-server/"
          prune                  = false
          retryIntervalInSeconds = 300
          syncIntervalInSeconds  = 300
          timeoutInSeconds       = 300
          wait                   = true
        }
        aks-config-teams-rbac = {
          force = false
          path  = "./rbac/"
          postBuild = {
            substitute = {
              AKS_READ_EVERYTHING_AND_RESTART_GROUP_ID = "${var.developer_entra_id_group}"
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
        url                   = "oci://altinncr.azurecr.io/manifests/infra/container-runtime-aks-config"
        useWorkloadIdentity   = true
      }
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
