resource "kubectl_manifest" "metrics_server_configmap" {
  depends_on     = [azurerm_kubernetes_cluster.aks]
  yaml_body = <<YAML
apiVersion: v1
kind: ConfigMap
metadata:
  name: metrics-server-config
  namespace: kube-system
  labels:
    kubernetes.io/cluster-service: "true"
    addonmanager.kubernetes.io/mode: EnsureExists
data:
  NannyConfiguration: |-
    apiVersion: nannyconfig/v1alpha1
    kind: NannyConfiguration
    baseCPU: 100m
    cpuPerNode: 1m
    baseMemory: 100Mi
    memoryPerNode: 8Mi
YAML
}
