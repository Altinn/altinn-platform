# AMA Metrics Settings Configuration

This directory contains the Azure Monitor Agent (AMA) settings configuration that controls which metrics are collected from the Kubernetes cluster.

The configuration enables/disables various metric collectors (kubelet, cadvisor, kubestate, etc.) and sets scrape intervals and namespaces for pod annotation-based scraping.