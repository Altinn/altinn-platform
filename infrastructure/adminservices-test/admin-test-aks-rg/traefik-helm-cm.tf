resource "kubectl_manifest" "traefik_namespace" {
  depends_on = [azurerm_kubernetes_cluster_extension.flux_ext]
  yaml_body  = <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: traefik
YAML
}

resource "kubectl_manifest" "traefik_helm_values_configmap" {
  depends_on = [azurerm_kubernetes_cluster_extension.flux_ext]
  yaml_body  = <<YAML
apiVersion: v1
kind: ConfigMap
metadata:
  name: traefik-helm-values
  namespace: traefik
data:
  values.yaml: |
    service:
      annotations:
        service.beta.kubernetes.io/azure-load-balancer-resource-group: ${azurerm_kubernetes_cluster.aks.node_resource_group}
        service.beta.kubernetes.io/azure-load-balancer-ipv4: ${azurerm_public_ip.pip4.ip_address}
        service.beta.kubernetes.io/azure-load-balancer-ipv6: ${azurerm_public_ip.pip6.ip_address}
YAML
}
