resource "kubectl_manifest" "flux_traefik_ocirepo" {
  depends_on = [azurerm_kubernetes_cluster_extension.flux_ext]
  yaml_body  = <<YAML
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: OCIRepository
metadata:
  name: traefik
  namespace: flux-system
spec:
  provider: azure
  interval: 5m
  url: oci://altinncr.azurecr.io/manifests/infra/traefik
  ref:
    tag: ${var.flux_release_tag}
YAML
}

resource "kubectl_manifest" "flux_traefik_kustomization" {
  depends_on = [kubectl_manifest.flux_traefik_ocirepo]
  yaml_body  = <<YAML
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: traefik
  namespace: flux-system
spec:
  sourceRef:
    kind: OCIRepository
    name: traefik
  interval: 5m
  retryInterval: 5m
  path: ./
  prune: true
  wait: true
  timeout: 5m
  patches:
    - patch: |-
        apiVersion: helm.toolkit.fluxcd.io/v2
        kind: HelmRelease
        metadata:
          name: altinn-traefik
        spec:
          values:
            service:
              spec:
                externalTrafficPolicy: Local
              ipFamilyPolicy: PreferDualStack
              ipFamilies:
                - IPv4
                - IPv6
              annotations:
                service.beta.kubernetes.io/azure-load-balancer-resource-group: ${azurerm_kubernetes_cluster.aks.node_resource_group}
                service.beta.kubernetes.io/azure-load-balancer-ipv4: ${azurerm_public_ip.pip4.ip_address}
                service.beta.kubernetes.io/azure-load-balancer-ipv6: ${azurerm_public_ip.pip6.ip_address}
      target:
        kind: HelmRelease
        name: altinn-traefik
YAML
}
