resource "azapi_resource" "grafana-operator" {
  depends_on = [azapi_resource.linkerd]
  type       = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name       = "grafana-operator"
  parent_id  = var.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        traefik = {
          force = false
          path  = "./"
          postBuild = {
            substitute = {
              EXTERNAL_GRAFANA_URL : "${var.grafana_endpoint}"
              GRAFANA_ADMIN_APIKEY : "${var.token_grafana_operator}"
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
        url                   = "oci://altinncr.azurecr.io/manifests/infra/grafana-operator"
        useWorkloadIdentity   = true
      }
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
