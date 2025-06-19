resource "azapi_resource" "grafana_manifests" {
  count      = var.grafana_endpoint != null && var.grafana_endpoint != "" ? 1 : 0
  depends_on = [azapi_resource.linkerd]
  type       = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name       = "grafana-manifests"
  parent_id  = var.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        grafana-manifests = {
          force                  = false
          path                   = "./dashboards/"
          prune                  = false
          retryIntervalInSeconds = 300
          syncIntervalInSeconds  = 300
          timeoutInSeconds       = 300
          wait                   = true
          postBuild = {
            substitute = var.grafana_dashboard_release_branch != "" ? {
              RELEASE_BRANCH = var.grafana_dashboard_release_branch
            } : {}
          }
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
        url                   = "oci://altinncr.azurecr.io/manifests/infra/grafana-manifests"
        useWorkloadIdentity   = true
      }
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
