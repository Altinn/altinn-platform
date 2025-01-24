resource "kubectl_manifest" "flux_whoami_ocirepo" {
  depends_on = [kubectl_manifest.flux_traefik_kustomization]
  yaml_body  = <<YAML
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: OCIRepository
metadata:
  name: whoami
  namespace: flux-system
spec:
  provider: azure
  interval: 5m
  url: oci://altinncr.azurecr.io/manifests/infra/whoami
  ref:
    tag: ${var.flux_release_tag}
YAML
}

resource "kubectl_manifest" "flux_whoami_kustomization" {
  depends_on = [kubectl_manifest.flux_whoami_ocirepo]
  yaml_body  = <<YAML
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: whoami
  namespace: flux-system
spec:
  sourceRef:
    kind: OCIRepository
    name: whoami
  interval: 5m
  retryInterval: 5m
  path: ./
  prune: true
  wait: true
  timeout: 5m
YAML
}
