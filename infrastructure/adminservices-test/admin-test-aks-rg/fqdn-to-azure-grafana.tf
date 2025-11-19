resource "azapi_resource" "fqdn_to_azure_grafana" {
  type      = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name      = "fqdn-to-azure-grafana"
  parent_id = module.aks.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        redirect-grafana-fqdn-to-azure-grafana = {
          force = false
          path  = "./fqdn-to-azure-grafana/"
          postBuild = {
            substitute = {
              REDIRECT_GRAFANA_FROM_FQDN = "grafana.altinn.cloud"
              REDIRECT_GRAFANA_TO_FQDN   = "altinn-grafana-test-b2b8dpdkcvfuhfd3.eno.grafana.azure.com"
            }
          }
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
        url                   = "oci://altinncr.azurecr.io/manifests/infra/grafana-operator"
        useWorkloadIdentity   = true
      }
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
