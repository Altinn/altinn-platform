# AMA Metrics Prometheus Configuration

This directory contains the Prometheus configuration for Azure Monitor Agent (AMA) to scrape Traefik metrics from the `altinn-traefik-metrics` service in the `traefik` namespace.

The configuration filters metrics to only include specific Traefik metrics and routes them to the centralized monitoring account in Azure Monitor.