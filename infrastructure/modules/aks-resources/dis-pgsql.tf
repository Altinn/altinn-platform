resource "azapi_resource" "dis_pgsql_operator" {
  depends_on = [azapi_resource.cert_manager]
  count      = var.enable_dis_pgsql_operator ? 1 : 0
  type       = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name       = "dis-pgsql"
  parent_id  = var.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        dis-pgsql = {
          force = false
          path  = "./"
          postBuild = {
            substitute = {
              DISPG_ISSUER_URL            = "${var.azurerm_kubernetes_cluster_oidc_issuer_url}"
              DISPG_TARGET_RESOURCE_GROUP = "${var.dis_pgsql_resource_group_id}"
              DISPG_UAMI_CLIENT_ID        = "${var.dis_pgsql_uami_client_id}"
            }
          }
          prune                  = false
          retryIntervalInSeconds = 300
          syncIntervalInSeconds  = 300
          timeoutInSeconds       = 300
          wait                   = true
        }
      }
      ociRepository = {
        insecure = false
        repositoryRef = {
          tag = var.flux_release_tag
        }
        syncIntervalInSeconds = 300
        timeoutInSeconds      = 300
        url                   = "oci://altinncr.azurecr.io/manifests/infra/dis-pgsql"
        useWorkloadIdentity   = true
      }
      namespace                  = "flux-system"
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
