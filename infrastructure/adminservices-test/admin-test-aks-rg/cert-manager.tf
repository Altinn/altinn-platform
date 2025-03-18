resource "kubectl_manifest" "flux_cwert_manger_namespace" {
  depends_on = [azurerm_kubernetes_cluster_extension.flux_ext]
  yaml_body  = <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: cert-manager
YAML
}


resource "kubectl_manifest" "flux_cert_manager_helm_repository" {
  depends_on = [kubectl_manifest.flux_cwert_manger_namespace]
  yaml_body  = <<YAML
apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: cert-manager
  namespace: cert-manager
spec:
  interval: 24h
  url: https://charts.jetstack.io
YAML
}

resource "kubectl_manifest" "flux_cert_manager_helm_release" {
  depends_on = [kubectl_manifest.flux_cert_manager_helm_repository]
  yaml_body  = <<YAML
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: cert-manager
  namespace: cert-manager
spec:
  interval: 30m
  chart:
    spec:
      chart: cert-manager
      version: "${var.cert_manager_version}"
      sourceRef:
        kind: HelmRepository
        name: cert-manager
        namespace: cert-manager
      interval: 12h
  values:
    installCRDs: true
YAML
}