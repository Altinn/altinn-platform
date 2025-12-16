# Node Recording Rules for Linux
# Note: These rules are Linux-specific. Windows nodes would require separate rule groups.
resource "azurerm_monitor_alert_prometheus_rule_group" "node_recording_rules_linux" {
  count = var.enable_aks_monitoring ? 1 : 0

  name                = "NodeRecordingRulesRuleGroup-linux"
  location            = var.location
  resource_group_name = local.rg.name
  cluster_name        = local.cluster_name
  description         = "Node Exporter recording rules for Prometheus - Linux"
  rule_group_enabled  = true
  interval            = "PT1M"
  scopes              = [local.amw.id, var.azurerm_kubernetes_cluster_id]

  tags = merge(var.localtags, {
    submodule = "observability"
  })

  depends_on = [
    azurerm_monitor_data_collection_rule.amw
  ]

  rule {
    enabled    = true
    record     = "instance:node_num_cpu:sum"
    expression = <<-EOT
      count without (cpu, mode) (
        node_cpu_seconds_total{job="node",mode="idle"}
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "instance:node_cpu_utilisation:rate5m"
    expression = <<-EOT
      1 - avg without (cpu) (
        sum without (mode) (rate(node_cpu_seconds_total{job="node", mode=~"idle|iowait|steal"}[5m]))
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "instance:node_load1_per_cpu:ratio"
    expression = <<-EOT
      (
        node_load1{job="node"}
        / instance:node_num_cpu:sum{job="node"}
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "instance:node_memory_utilisation:ratio"
    expression = <<-EOT
      1 - (
        (
          node_memory_MemAvailable_bytes{job="node"}
          or
          (
            node_memory_Buffers_bytes{job="node"} +
            node_memory_Cached_bytes{job="node"} +
            node_memory_MemFree_bytes{job="node"} +
            node_memory_Slab_bytes{job="node"}
          )
        )
        / node_memory_MemTotal_bytes{job="node"}
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "instance:node_vmstat_pgmajfault:rate5m"
    expression = "rate(node_vmstat_pgmajfault{job=\"node\"}[5m])"
  }

  rule {
    enabled    = true
    record     = "instance_device:node_disk_io_time_seconds:rate5m"
    expression = "rate(node_disk_io_time_seconds_total{job=\"node\", device!=\"\"}[5m])"
  }

  rule {
    enabled    = true
    record     = "instance_device:node_disk_io_time_weighted_seconds:rate5m"
    expression = "rate(node_disk_io_time_weighted_seconds_total{job=\"node\", device!=\"\"}[5m])"
  }

  rule {
    enabled    = true
    record     = "instance:node_network_receive_bytes_excluding_lo:rate5m"
    expression = <<-EOT
      sum without (device) (
        rate(node_network_receive_bytes_total{job="node", device!="lo"}[5m])
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "instance:node_network_transmit_bytes_excluding_lo:rate5m"
    expression = <<-EOT
      sum without (device) (
        rate(node_network_transmit_bytes_total{job="node", device!="lo"}[5m])
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "instance:node_network_receive_drop_excluding_lo:rate5m"
    expression = <<-EOT
      sum without (device) (
        rate(node_network_receive_drop_total{job="node", device!="lo"}[5m])
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "instance:node_network_transmit_drop_excluding_lo:rate5m"
    expression = <<-EOT
      sum without (device) (
        rate(node_network_transmit_drop_total{job="node", device!="lo"}[5m])
      )
    EOT
  }
}

# Kubernetes Recording Rules for Linux
# Kubernetes Recording Rules for Linux
# Note: These rules are Linux-specific. Windows nodes would require separate rule groups.
resource "azurerm_monitor_alert_prometheus_rule_group" "kubernetes_recording_rules_linux" {
  count = var.enable_aks_monitoring ? 1 : 0

  name                = "KubernetesRecordingRulesRuleGroup-linux"
  location            = var.location
  resource_group_name = local.rg.name
  cluster_name        = local.cluster_name
  description         = "Kubernetes recording rules for Prometheus - Linux"
  rule_group_enabled  = true
  interval            = "PT1M"
  scopes              = [local.amw.id, var.azurerm_kubernetes_cluster_id]

  tags = merge(var.localtags, {
    submodule = "observability"
  })

  depends_on = [
    azurerm_monitor_data_collection_rule.amw
  ]

  rule {
    enabled    = true
    record     = "node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate"
    expression = <<-EOT
      sum by (cluster, namespace, pod, container) (
        irate(container_cpu_usage_seconds_total{job="cadvisor", image!=""}[5m])
      ) * on (cluster, namespace, pod) group_left(node) topk by (cluster, namespace, pod) (
        1, max by(cluster, namespace, pod, node) (kube_pod_info{node!=""})
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "node_namespace_pod_container:container_memory_working_set_bytes"
    expression = <<-EOT
      container_memory_working_set_bytes{job="cadvisor", image!=""}
      * on (namespace, pod) group_left(node) topk by(namespace, pod) (
        1, max by(namespace, pod, node) (kube_pod_info{node!=""})
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "node_namespace_pod_container:container_memory_rss"
    expression = <<-EOT
      container_memory_rss{job="cadvisor", image!=""}
      * on (namespace, pod) group_left(node) topk by(namespace, pod) (
        1, max by(namespace, pod, node) (kube_pod_info{node!=""})
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "node_namespace_pod_container:container_memory_cache"
    expression = <<-EOT
      container_memory_cache{job="cadvisor", image!=""}
      * on (namespace, pod) group_left(node) topk by(namespace, pod) (
        1, max by(namespace, pod, node) (kube_pod_info{node!=""})
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "node_namespace_pod_container:container_memory_swap"
    expression = <<-EOT
      container_memory_swap{job="cadvisor", image!=""}
      * on (namespace, pod) group_left(node) topk by(namespace, pod) (
        1, max by(namespace, pod, node) (kube_pod_info{node!=""})
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "cluster:namespace:pod_memory:active:kube_pod_container_resource_requests"
    expression = <<-EOT
      kube_pod_container_resource_requests{resource="memory",job="kube-state-metrics"}
      * on (namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (
        (kube_pod_status_phase{phase=~"Pending|Running"} == 1)
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "namespace_memory:kube_pod_container_resource_requests:sum"
    expression = <<-EOT
      sum by (namespace, cluster) (
        sum by (namespace, pod, cluster) (
          max by (namespace, pod, container, cluster) (
            kube_pod_container_resource_requests{resource="memory",job="kube-state-metrics"}
          ) * on(namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (
            kube_pod_status_phase{phase=~"Pending|Running"} == 1
          )
        )
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "cluster:namespace:pod_cpu:active:kube_pod_container_resource_requests"
    expression = <<-EOT
      kube_pod_container_resource_requests{resource="cpu",job="kube-state-metrics"}
      * on (namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (
        (kube_pod_status_phase{phase=~"Pending|Running"} == 1)
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "namespace_cpu:kube_pod_container_resource_requests:sum"
    expression = <<-EOT
      sum by (namespace, cluster) (
        sum by (namespace, pod, cluster) (
          max by (namespace, pod, container, cluster) (
            kube_pod_container_resource_requests{resource="cpu",job="kube-state-metrics"}
          ) * on(namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (
            kube_pod_status_phase{phase=~"Pending|Running"} == 1
          )
        )
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "cluster:namespace:pod_memory:active:kube_pod_container_resource_limits"
    expression = <<-EOT
      kube_pod_container_resource_limits{resource="memory",job="kube-state-metrics"}
      * on (namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (
        (kube_pod_status_phase{phase=~"Pending|Running"} == 1)
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "namespace_memory:kube_pod_container_resource_limits:sum"
    expression = <<-EOT
      sum by (namespace, cluster) (
        sum by (namespace, pod, cluster) (
          max by (namespace, pod, container, cluster) (
            kube_pod_container_resource_limits{resource="memory",job="kube-state-metrics"}
          ) * on(namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (
            kube_pod_status_phase{phase=~"Pending|Running"} == 1
          )
        )
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "cluster:namespace:pod_cpu:active:kube_pod_container_resource_limits"
    expression = <<-EOT
      kube_pod_container_resource_limits{resource="cpu",job="kube-state-metrics"}
      * on (namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (
        (kube_pod_status_phase{phase=~"Pending|Running"} == 1)
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "namespace_cpu:kube_pod_container_resource_limits:sum"
    expression = <<-EOT
      sum by (namespace, cluster) (
        sum by (namespace, pod, cluster) (
          max by (namespace, pod, container, cluster) (
            kube_pod_container_resource_limits{resource="cpu",job="kube-state-metrics"}
          ) * on(namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (
            kube_pod_status_phase{phase=~"Pending|Running"} == 1
          )
        )
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "namespace_workload_pod:kube_pod_owner:relabel"
    expression = <<-EOT
      max by (cluster, namespace, workload, pod) (
        label_replace(
          label_replace(
            kube_pod_owner{job="kube-state-metrics", owner_kind="ReplicaSet"},
            "replicaset", "$1", "owner_name", "(.*)"
          ) * on(replicaset, namespace) group_left(owner_name) topk by(replicaset, namespace) (
            1, max by (replicaset, namespace, owner_name) (
              kube_replicaset_owner{job="kube-state-metrics"}
            )
          ),
          "workload", "$1", "owner_name", "(.*)"
        )
      )
    EOT
    labels = {
      workload_type = "deployment"
    }
  }

  rule {
    enabled    = true
    record     = "namespace_workload_pod:kube_pod_owner:relabel"
    expression = <<-EOT
      max by (cluster, namespace, workload, pod) (
        label_replace(
          kube_pod_owner{job="kube-state-metrics", owner_kind="DaemonSet"},
          "workload", "$1", "owner_name", "(.*)"
        )
      )
    EOT
    labels = {
      workload_type = "daemonset"
    }
  }

  rule {
    enabled    = true
    record     = "namespace_workload_pod:kube_pod_owner:relabel"
    expression = <<-EOT
      max by (cluster, namespace, workload, pod) (
        label_replace(
          kube_pod_owner{job="kube-state-metrics", owner_kind="StatefulSet"},
          "workload", "$1", "owner_name", "(.*)"
        )
      )
    EOT
    labels = {
      workload_type = "statefulset"
    }
  }

  rule {
    enabled    = true
    record     = "namespace_workload_pod:kube_pod_owner:relabel"
    expression = <<-EOT
      max by (cluster, namespace, workload, pod) (
        label_replace(
          kube_pod_owner{job="kube-state-metrics", owner_kind="Job"},
          "workload", "$1", "owner_name", "(.*)"
        )
      )
    EOT
    labels = {
      workload_type = "job"
    }
  }

  rule {
    enabled    = true
    record     = ":node_memory_MemAvailable_bytes:sum"
    expression = <<-EOT
      sum(
        node_memory_MemAvailable_bytes{job="node"}
        or
        (
          node_memory_Buffers_bytes{job="node"} +
          node_memory_Cached_bytes{job="node"} +
          node_memory_MemFree_bytes{job="node"} +
          node_memory_Slab_bytes{job="node"}
        )
      ) by (cluster)
    EOT
  }

  rule {
    enabled    = true
    record     = "cluster:node_cpu:ratio_rate5m"
    expression = <<-EOT
      sum(rate(node_cpu_seconds_total{job="node",mode!="idle",mode!="iowait",mode!="steal"}[5m])) by (cluster)
      / count(sum(node_cpu_seconds_total{job="node"}) by (cluster, instance, cpu)) by (cluster)
    EOT
  }
}

# UX Recording Rules for Azure Portal Integration
# UX Recording Rules for Linux
# Note: These rules are Linux-specific and enable Azure Portal monitoring blade integration.
resource "azurerm_monitor_alert_prometheus_rule_group" "ux_recording_rules_linux" {
  count = var.enable_aks_monitoring ? 1 : 0

  name                = "UXRecordingRulesRuleGroup-linux"
  location            = var.location
  resource_group_name = local.rg.name
  cluster_name        = local.cluster_name
  description         = "UX Recording Rules for Linux - Enables Azure Portal monitoring blade"
  rule_group_enabled  = true
  interval            = "PT1M"
  scopes              = [local.amw.id, var.azurerm_kubernetes_cluster_id]

  tags = merge(var.localtags, {
    submodule = "observability"
  })

  depends_on = [
    azurerm_monitor_data_collection_rule.amw
  ]

  rule {
    enabled    = true
    record     = "ux:pod_cpu_usage:sum_irate"
    expression = <<-EOT
      (sum by (namespace, pod, cluster, microsoft_resourceid) (irate(container_cpu_usage_seconds_total{container != "", pod != "", job = "cadvisor"}[5m])))
      * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
      (max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (kube_pod_info{pod != "", job = "kube-state-metrics"}))
    EOT
  }

  rule {
    enabled    = true
    record     = "ux:controller_cpu_usage:sum_irate"
    expression = "sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (ux:pod_cpu_usage:sum_irate)"
  }

  rule {
    enabled    = true
    record     = "ux:pod_workingset_memory:sum"
    expression = <<-EOT
      (sum by (namespace, pod, cluster, microsoft_resourceid) (container_memory_working_set_bytes{container != "", pod != "", job = "cadvisor"}))
      * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
      (max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (kube_pod_info{pod != "", job = "kube-state-metrics"}))
    EOT
  }

  rule {
    enabled    = true
    record     = "ux:controller_workingset_memory:sum"
    expression = "sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (ux:pod_workingset_memory:sum)"
  }

  rule {
    enabled    = true
    record     = "ux:pod_rss_memory:sum"
    expression = <<-EOT
      (sum by (namespace, pod, cluster, microsoft_resourceid) (container_memory_rss{container != "", pod != "", job = "cadvisor"}))
      * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
      (max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (kube_pod_info{pod != "", job = "kube-state-metrics"}))
    EOT
  }

  rule {
    enabled    = true
    record     = "ux:controller_rss_memory:sum"
    expression = "sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (ux:pod_rss_memory:sum)"
  }

  rule {
    enabled    = true
    record     = "ux:pod_container_count:sum"
    expression = <<-EOT
      sum by (node, created_by_name, created_by_kind, namespace, cluster, pod, microsoft_resourceid) (
        (sum by (container, pod, namespace, cluster, microsoft_resourceid) (kube_pod_container_info{container != "", pod != "", container_id != "", job = "kube-state-metrics"})
        or sum by (container, pod, namespace, cluster, microsoft_resourceid) (kube_pod_init_container_info{container != "", pod != "", container_id != "", job = "kube-state-metrics"}))
        * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
        (max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (kube_pod_info{pod != "", job = "kube-state-metrics"}))
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "ux:controller_container_count:sum"
    expression = "sum by (node, created_by_name, created_by_kind, namespace, cluster, microsoft_resourceid) (ux:pod_container_count:sum)"
  }

  rule {
    enabled    = true
    record     = "ux:pod_container_restarts:max"
    expression = <<-EOT
      max by (node, created_by_name, created_by_kind, namespace, cluster, pod, microsoft_resourceid) (
        (max by (container, pod, namespace, cluster, microsoft_resourceid) (kube_pod_container_status_restarts_total{container != "", pod != "", job = "kube-state-metrics"})
        or sum by (container, pod, namespace, cluster, microsoft_resourceid) (kube_pod_init_status_restarts_total{container != "", pod != "", job = "kube-state-metrics"}))
        * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
        (max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (kube_pod_info{pod != "", job = "kube-state-metrics"}))
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "ux:controller_container_restarts:max"
    expression = "max by (node, created_by_name, created_by_kind, namespace, cluster, microsoft_resourceid) (ux:pod_container_restarts:max)"
  }

  rule {
    enabled    = true
    record     = "ux:pod_resource_limit:sum"
    expression = <<-EOT
      (sum by (cluster, pod, namespace, resource, microsoft_resourceid) (max by (cluster, microsoft_resourceid, pod, container, namespace, resource) (kube_pod_container_resource_limits{container != "", pod != "", job = "kube-state-metrics"}))
      unless (count by (pod, namespace, cluster, resource, microsoft_resourceid) (kube_pod_container_resource_limits{container != "", pod != "", job = "kube-state-metrics"})
      != on (pod, namespace, cluster, microsoft_resourceid) group_left() sum by (pod, namespace, cluster, microsoft_resourceid) (kube_pod_container_info{container != "", pod != "", job = "kube-state-metrics"})))
      * on (namespace, pod, cluster, microsoft_resourceid) group_left (node, created_by_kind, created_by_name) (kube_pod_info{pod != "", job = "kube-state-metrics"})
    EOT
  }

  rule {
    enabled    = true
    record     = "ux:controller_resource_limit:sum"
    expression = "sum by (cluster, namespace, created_by_name, created_by_kind, node, resource, microsoft_resourceid) (ux:pod_resource_limit:sum)"
  }

  rule {
    enabled    = true
    record     = "ux:controller_pod_phase_count:sum"
    expression = <<-EOT
      sum by (cluster, phase, node, created_by_kind, created_by_name, namespace, microsoft_resourceid) (
        (kube_pod_status_phase{job="kube-state-metrics",pod!=""}
        or (label_replace((count(kube_pod_deletion_timestamp{job="kube-state-metrics",pod!=""}) by (namespace, pod, cluster, microsoft_resourceid)
        * count(kube_pod_status_reason{reason="NodeLost", job="kube-state-metrics"} == 0) by (namespace, pod, cluster, microsoft_resourceid)), "phase", "terminating", "", "")))
        * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
        (max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (kube_pod_info{job="kube-state-metrics",pod!=""}))
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "ux:cluster_pod_phase_count:sum"
    expression = "sum by (cluster, phase, node, namespace, microsoft_resourceid) (ux:controller_pod_phase_count:sum)"
  }

  rule {
    enabled    = true
    record     = "ux:node_cpu_usage:sum_irate"
    expression = "sum by (instance, cluster, microsoft_resourceid) ((1 - irate(node_cpu_seconds_total{job=\"node\", mode=\"idle\"}[5m])))"
  }

  rule {
    enabled    = true
    record     = "ux:node_memory_usage:sum"
    expression = <<-EOT
      sum by (instance, cluster, microsoft_resourceid) (
        (node_memory_MemTotal_bytes{job = "node"} - node_memory_MemFree_bytes{job = "node"} - node_memory_cached_bytes{job = "node"} - node_memory_buffers_bytes{job = "node"})
      )
    EOT
  }

  rule {
    enabled    = true
    record     = "ux:node_network_receive_drop_total:sum_irate"
    expression = "sum by (instance, cluster, microsoft_resourceid) (irate(node_network_receive_drop_total{job=\"node\", device!=\"lo\"}[5m]))"
  }

  rule {
    enabled    = true
    record     = "ux:node_network_transmit_drop_total:sum_irate"
    expression = "sum by (instance, cluster, microsoft_resourceid) (irate(node_network_transmit_drop_total{job=\"node\", device!=\"lo\"}[5m]))"
  }
}

# Outputs (optional)
output "node_recording_rules_id" {
  description = "ID of the Node Recording Rules rule group"
  value       = var.enable_aks_monitoring ? azurerm_monitor_alert_prometheus_rule_group.node_recording_rules_linux[0].id : null
}

output "kubernetes_recording_rules_id" {
  description = "ID of the Kubernetes Recording Rules rule group"
  value       = var.enable_aks_monitoring ? azurerm_monitor_alert_prometheus_rule_group.kubernetes_recording_rules_linux[0].id : null
}

output "ux_recording_rules_id" {
  description = "ID of the UX Recording Rules rule group"
  value       = var.enable_aks_monitoring ? azurerm_monitor_alert_prometheus_rule_group.ux_recording_rules_linux[0].id : null
}
