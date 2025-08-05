resource "azapi_resource" "dis_apim_operator" {
  type      = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name      = "dis-apim-${var.dis_apim_service_name}"
  parent_id = var.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        dis-apim = {
          force = false
          path  = "./"
          postBuild = {
            substitute = {
              DISAPIM_SUBSCRIPTION_ID             = "${var.dis_apim_subscription_id}"
              DISAPIM_RESOURCE_GROUP              = "${var.dis_apim_resource_group_name}"
              DISAPIM_APIM_SERVICE_NAME           = "${var.dis_apim_service_name}"
              DISAPIM_TARGET_NAMESPACE            = "${var.dis_apim_target_namespace}"
              DISAPIM_WORKLOAD_IDENTITY_CLIENT_ID = "${azurerm_user_assigned_identity.disapim_identity.client_id}"
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
        url                   = "oci://altinncr.azurecr.io/manifests/infra/dis-apim"
        useWorkloadIdentity   = true
      }
      namespace                  = "flux-system"
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
