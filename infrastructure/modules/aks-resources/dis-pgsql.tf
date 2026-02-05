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
              DISPG_AZURE_SUBSCRIPTION_ID = "${var.subscription_id}"
              DISPG_AZURE_TENANT_ID = "${var.obs_tenant_id}" # same tenant as obs
              DISPG_DB_RESOURCE_GROUP = "${var.dis_resource_group_name}"
              DISPG_DB_VNET_NAME = "${var.dis_db_vnet_name}"
              DISPG_AKS_VNET_NAME = "${var.aks_workpool_vnet_name}"
              DISPG_AKS_RESOURCE_GROUP = "${var.aks_node_resource_group}"
              DISPG_WORKLOAD_IDENTITY_CLIENT_ID = "${var.dis_pgsql_uami_client_id}"
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
