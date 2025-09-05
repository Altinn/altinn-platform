resource "azapi_resource" "linkerd" {
  depends_on = [azapi_resource.cert_manager]
  type       = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name       = "linkerd"
  parent_id  = var.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        linkerd = {
          force = false
          path  = "./"
          postBuild = {
            substitute = {
              DISABLE_IPV6           = "false"
              DEFAULT_INBOUND_POLICY = var.linkerd_default_inbound_policy
            }
          }
          prune                  = false
          retryIntervalInSeconds = 300
          syncIntervalInSeconds  = 300
          timeoutInSeconds       = 600
          wait                   = true
        },
        linkerd-post-deploy = {
          dependsOn = [
            "linkerd"
          ]
          force                  = true
          path                   = "./post-deploy/"
          prune                  = true
          retryIntervalInSeconds = 300
          # Set syncIntervalInSeconds to 1 year to not run regularly, will run on updates force=true
          syncIntervalInSeconds = 31557600
          timeoutInSeconds      = 300
          wait                  = true
        }
      }
      ociRepository = {
        insecure = false
        repositoryRef = {
          tag = var.flux_release_tag
        }
        syncIntervalInSeconds = 300
        timeoutInSeconds      = 300
        url                   = "oci://altinncr.azurecr.io/manifests/infra/linkerd"
        useWorkloadIdentity   = true
      }
      namespace                  = "flux-system"
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
