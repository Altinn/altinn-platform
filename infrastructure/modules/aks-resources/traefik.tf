resource "azapi_resource" "traefik" {
  depends_on = [azapi_resource.linkerd]
  type       = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name       = "traefik"
  parent_id  = var.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        traefik = {
          force = false
          path  = "./"
          postBuild = {
            substitute = {
              AKS_SYSP00L_IP_PREFIX_0 : "${var.subnet_address_prefixes.aks_syspool[0]}"
              AKS_SYSP00L_IP_PREFIX_1 : "${var.subnet_address_prefixes.aks_syspool[1]}"
              AKS_WORKPOOL_IP_PREFIX_0 : "${var.subnet_address_prefixes.aks_workpool[0]}"
              AKS_WORKPOOL_IP_PREFIX_1 : "${var.subnet_address_prefixes.aks_workpool[1]}"
              AKS_NODE_RG : "${var.aks_node_resource_group}"
              PUBLIC_IP_V4 : "${var.pip4_ip_address}"
              PUBLIC_IP_V6 : "${var.pip6_ip_address}"
              # EXTERNAL_TRAFFIC_POLICY: Cluster (Local is default in traefik oci)
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
        url                   = "oci://altinncr.azurecr.io/manifests/infra/traefik"
        useWorkloadIdentity   = true
      }
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
