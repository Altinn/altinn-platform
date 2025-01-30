resource "kubectl_manifest" "grafana_altinn_cloud_middleware" {
  depends_on = [kubectl_manifest.flux_traefik_kustomization]
  yaml_body  = <<YAML
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: redirect-to-central-grafana
  namespace: traefik
spec:
  redirectRegex:
    permanent: true
    regex: ^http(|s)://(.*)grafana.(.*)altinn.(no|cloud)(.*)
    replacement:  https://altinn-grafana-test-b2b8dpdkcvfuhfd3.eno.grafana.azure.com$${5}
YAML
}

resource "kubectl_manifest" "grafana_altinn_cloud_ingressroute" {
  depends_on = [kubectl_manifest.grafana_altinn_cloud_middleware]
  yaml_body  = <<YAML
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: redirect-to-central-grafana
  namespace: traefik
spec:
  entryPoints:
  - https
  routes:
  - kind: Rule
    match: Host(`grafana.altinn.cloud`) || Host(`dev.grafana.altinn.cloud`) && PathPrefix(`/`)
    middlewares:
    - name: redirect-to-central-grafana
      namespace: traefik
    services:
    - kind: TraefikService
      name: noop@internal
YAML
}
