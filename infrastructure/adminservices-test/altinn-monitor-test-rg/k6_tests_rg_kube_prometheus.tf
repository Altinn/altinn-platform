resource "helm_release" "prometheus_operator_crds" {
  name       = "prometheus-operator-crds"
  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "prometheus-operator-crds"
  version    = "16.0.1"
}

resource "helm_release" "kube_prometheus_stack" {
  depends_on       = [helm_release.prometheus_operator_crds]
  name             = "kube-prometheus-stack"
  namespace        = "monitoring"
  create_namespace = true
  repository       = "https://prometheus-community.github.io/helm-charts"
  chart            = "kube-prometheus-stack"
  skip_crds        = true
  version          = "66.3.1"

  values = [<<-EOT
              crds:
                enabled: false
              alertmanager:
                enabled: true
              grafana:
                enabled: false
              prometheus:
                enabled: true
                prometheusSpec:
                  externalLabels:
                    cluster: "${azurerm_kubernetes_cluster.k6tests.name}"
                  priorityClassName: "system-cluster-critical"
                  retention: 1d
                  storageSpec:
                    volumeClaimTemplate:
                      spec:
                        resources:
                          requests:
                            storage: 5Gi
              EOT
  ]
}
