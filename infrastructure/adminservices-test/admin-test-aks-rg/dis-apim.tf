resource "kubectl_manifest" "flux_dis_apim_ocirepo" {
  depends_on = [
    module.infra_resources,
  ]
  yaml_body = <<YAML
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: OCIRepository
metadata:
  name: dis-apim
  namespace: flux-system
spec:
  provider: azure
  interval: 5m
  url: oci://altinncr.azurecr.io/ghcr.io/altinn/altinn-platform/kustomize/dis-apim-operator
  ref:
    tag: ${var.flux_release_tag}
YAML
}

resource "kubectl_manifest" "flux_dis_apim_dev_kustomization" {
  depends_on = [
    kubectl_manifest.flux_dis_apim_ocirepo,
  ]
  yaml_body = <<YAML
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: dis-apim
  namespace: flux-system
spec:
  sourceRef:
    kind: OCIRepository
    name: dis-apim
  interval: 5m
  targetNamespace: dis-apim-operator-system
  retryInterval: 5m
  path: ./default
  prune: true
  wait: true
  timeout: 10m
  images:
    - name: controller
      newName: ghcr.io/altinn/altinn-platform/dis-apim-operator
      newTag: ${var.flux_release_tag}
  patches:
    - target:
        kind: ServiceAccount
        name: controller-manager
      patch: |-
        - op: add
          path: /metadata/annotations
          value:
            azure.workload.identity/client-id: "${azuread_service_principal.dis_apim_sp.client_id}"
    - target:
        kind: Deployment
      patch: |-
        - op: add
          path: /spec/template/spec/containers/0/env
          value:
            - name: DISAPIM_SUBSCRIPTION_ID
              value: ${var.subscription_id}
            - name: DISAPIM_RESOURCE_GROUP
              value: "altinn-apim-test-rg"
            - name: DISAPIM_APIM_SERVICE_NAME
              value: "altinn-apim-test-apim"
        - op: add
          path: /spec/template/metadata/labels/azure.workload.identity~1use
          value: "true"
YAML
}
