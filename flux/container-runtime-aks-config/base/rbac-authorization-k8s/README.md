# RBAC Authorization Configuration

This directory contains the RBAC configuration that grants read access to all Kubernetes resources and restart capabilities to an Entra ID group.

The configuration requires the `${AKS_READ_EVERYTHING_AND_RESTART_GROUP_ID}` environment variable to be set with the Entra ID group object ID.

The ClusterRole allows reading all resources across core, apps, batch, and extensions API groups, with additional permissions to update deployments and delete pods for restart operations.