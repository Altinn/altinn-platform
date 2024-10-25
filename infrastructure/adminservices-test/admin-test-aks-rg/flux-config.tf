resource "kubectl_manifest" "flux_config" {
  depends_on = [azurerm_kubernetes_cluster_extension.flux_ext]
  yaml_body  = <<YAML
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: OCIRepository
metadata:
  name: flux-config
  namespace: flux-system
spec:
  interval: 5m
  url: oci://altinncr.azurecr.io/manifests/config
  provider: azure
  ref:
    tag: admin-test
YAML
}

resource "kubectl_manifest" "flux_config_kustomize" {
  depends_on = [azurerm_kubernetes_cluster_extension.flux_ext]
  yaml_body  = <<YAML
apiVersion: kustomize.toolkit.fluxcd.io/v1beta2
kind: Kustomization
metadata:
  name: flux-config
  namespace: flux-system
spec:
  sourceRef:
    kind: OCIRepository
    name: flux-config
  interval: 5m
  retryInterval: 5m
  path: ./
  prune: true
  wait: true
  timeout: 2m
YAML
}
