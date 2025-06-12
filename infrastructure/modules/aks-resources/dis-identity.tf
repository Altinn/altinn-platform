resource "azapi_resource" "dis_identity_operator" {
  count = var.enable_dis_identity_operator ? 1 : 0
  type      = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name      = "dis-identity-operator"
  parent_id = var.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        dis-identity-operator = {
          dependsOn = [
            "azure-service-operator-aso"
          ]
          force                  = false
          path                   = "./default"
          postBuild = {
            substitute = {
              DISID_ISSUER_URL = "${var.azurerm_kubernetes_cluster_oidc_issuer_url}"
              DISID_TARGET_RESOURCE_GROUP = "${var.azurerm_dis_identity_resource_group_id}"
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
        url                   = "oci://altinncr.azurecr.io/manifests/infra/dis-identity-operator"
        useWorkloadIdentity   = true
      }
      namespace                  = "flux-system"
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
