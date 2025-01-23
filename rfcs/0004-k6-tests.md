- Feature Name: `k6_tests_cluster`
- Start Date: 2025-01-09
- RFC PR: [altinn/altinn-platform#1211](https://github.com/Altinn/altinn-platform/pull/1211)
- Github Issue: [altinn/altinn-platform#1150](https://github.com/Altinn/altinn-platform/issues/1150)
- Product/Category: Monitoring
- State: **REVIEW**

# Summary
[summary]: #summary

This RFC proposes a Platform managed k8s cluster that developers can use to run K6 Tests from.
The cluster is multi-tenant - meaning multiple teams will share the same cluster but each team
will have it's own namespace.

The cluster can offer a variety of nodes with different specs to accomodate
different workloads. A mix of ["on-demand"](https://learn.microsoft.com/en-us/azure/aks/core-aks-concepts#vm-size-and-image) and
["spot"](https://learn.microsoft.com/en-us/azure/virtual-machines/spot-vms) node pools are available and developers can
select which nodes are needed depending on their needs. The node pools autoscale to accomodate
workload needs and are shared by multiple teams. There is, of course, the possibility of requesting dedicated
nodes if the need arises.

The solution also offers automatic test metrics storage in Azure Monitor Workspace (AMW) and visualization in Grafana by taking advantage of the
predefined Grafana dashboards from both k6 and Azure. More specific dashboards can be developed and
shared between the teams as well as further integrations (e.g. with app insights or log analytics).

Secret managed is implemented with SealedSecrets in cases where the tests need some sort of
authentication.

For extra feedback on the tests and the systems being tested we also support notifications via AlertManager and github commit status updates.

The interface for developers will be simple YAML file with the necessary configurations needed from them.
The k8s resources will be generated with Jsonnet based on the configs provided in the config file.

A Github Action that can be re-used by various teams will be maintained by the Platform team for simple integration and maintainability.

# Motivation
[motivation]: #motivation

The initial requirements came from Dagfinn as he needed a place to
store the metrics related to the performance tests he was running and also more performant nodes to
run the tests from as the nodes provided by Github (via Github Actions) were not performant
enough. It also seems like a good idea to standardize the way k6 tests are run throughout the company.
Ideally, we should make it easy for developers to write the tests themselves instead of relying on
a small group of people to write them all.

# Guide-level explanation
[guide-level-explanation]: #guide-level-explanation
> [!WARNING]
> Workflow is outdated but the general idea is still valid.
![General workflow](./img/k6-workflow-overview.png)

The expected workflow should be:
- Developers write down their k6 scripts.
- They create a team platform specific config file with basic information related to the tests such as
the entry point for the tests, node requirements, credentials needed, number of instances, etc.
- They push their code to their Github repo.
- We provide the automation via a Github Action that reads the config file and generate the boilerplate needed
to run the tests and deploy the manifests into the k8s cluster.
- The tests run,
- Developers check the Grafana dashboards / Github status checks if they are interested on the results or wait for
potential notifications from AlertManager.

# Reference-level explanation
[reference-level-explanation]: #reference-level-explanation

Onboarding a new Team requires some setup beforehand.
First, it's assumed that teams already have a [Service Principal](https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-azure)
that they are using to authenticate towards Azure from Github.

On our end, we need to create a namespace for the team, create the necessary Entra ID Groups, add the necessary members to the group and create the k8s RoleBindings. [Azure docs for an overview of the needed steps.](https://learn.microsoft.com/en-us/azure/aks/azure-ad-rbac)
Ideally, these will be done in a automated way but as of right now we have to do this manually.

There are 4 general groups defined:
- Cluster Admin: has the [`Azure Kubernetes Service Cluster Admin Role`](https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles/containers#azure-kubernetes-service-cluster-admin-role) to allow us to List cluster admin credential whenever we need to do anything in the cluster. It's also required by whatever ServicePrincipal we decide to use to manage resources in the cluster, e.g. to create the namespaces per team, create the role bindings, deploy services we might want to provide in cluster, etc. It's a [super-user access](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#user-facing-roles). In the future we might want to use this Group/Role only in exceptional cases and instead, create CustomRole with the permissions we know we need and another a CustomRole for the permissions needed by the Service Principal. And for us, we can use PIM when needed.

- Cluster User: has the [`Azure Kubernetes Service Cluster User Role`](https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles/containers#azure-kubernetes-service-cluster-user-role) to allow users (humans and service principals) to List cluster user credentials. This is what will allow them to interact with the cluster via kubectl for example.

- ${TEAM_NAME} Users - The object id will be used in the k8s RoleBinding to set the permissions that developers need to debug issues on the cluster - [example config](https://github.com/Altinn/altinn-platform/blob/d37e379417b1886f6d17816ba70bfae5ac664c32/infrastructure/adminservices-test/altinn-monitor-test-rg/k6_tests_rg_rbac.tf#L1-L31). This is a group per team.

- ${TEAM_SP} Users - The object id will be used in the k8s RoleBinding to set the permissions that service principals need to deploy k8s resources on the cluster - [example config](https://github.com/Altinn/altinn-platform/blob/d37e379417b1886f6d17816ba70bfae5ac664c32/infrastructure/adminservices-test/altinn-monitor-test-rg/k6_tests_rg_rbac.tf#L33-L63). This is a group per team. If a team has multiple repos, we can add the various SPs to the same group. (If they prefer to keep it separate, we can also follow the normal process as if it was a completely different team).

Once we've done our part, developers need to setup their own Github Workflow that references our GithubAction and fill out a config file that we use to generate the deployment manifests. The current implementation uses a simple yaml file as the config format and Jsonnet to create the needed manifests.

```
test_run:
  name: k6-enduser-search
  vus: 50
  duration: 10m
  parallelism: 10
  file_path: "/path/to/where/test/files/are/located"
```
The Github Action is a [composite action](https://docs.github.com/en/actions/sharing-automations/creating-actions/creating-a-composite-action) that runs a docker image with the needed tools and scripts to setup the environment, generate the needed k6 and k8s resources and deploys starts the test by deploying the manifests into the cluster.

Some of the steps include [creating a .tar file](https://grafana.com/docs/k6/latest/misc/archive/), creating a ConfigMap to hold the archive file, creating optional SealedSecrets with encrypted data and generating the actual [TestRun Custom Resource](https://grafana.com/docs/k6/latest/testing-guides/running-distributed-tests/#4-create-a-custom-resource). Other useful things such as adding labels, default test id values, etc. that are useful for integrating with other systems are also handled by the action.

An example of a TestRun config can be seen below.

```
apiVersion: k6.io/v1alpha1
kind: TestRun
metadata:
  name: k6-create-transmissions
  namespace: dialogporten
spec:
  arguments: --out experimental-prometheus-rw --vus=10 --duration=5m --tag testid=k6-create-transmissions_20250109T082811
  parallelism: 5
  script:
    configMap:
      name: k6-create-transmissions
      file: archive.tar
  runner:
    env:
    - name: K6_PROMETHEUS_RW_SERVER_URL
      value: "http://kube-prometheus-stack-prometheus.monitoring:9090/api/v1/write"
    - name: K6_PROMETHEUS_RW_TREND_STATS
      value: "avg,min,med,max,p(95),p(99),p(99.5),p(99.9),count"
    metadata:
      labels:
        k6-test: k6-create-transmissions
    resources:
      requests:
        memory: 200Mi
```

As the test is running, Grafana can be used to check the behavior in real time.

For developers that would like to have smoke tests implemented after every commit to main, it's possible use the [github api](https://docs.github.com/en/rest/commits/statuses?apiVersion=2022-11-28) to do it. For those who wish it, it's also possible to use [AlertManager](https://prometheus.io/docs/alerting/latest/configuration/#receiver) to generate notifications to systems such as Slack.

![Grafana K6 Prometheus Dashboard](https://grafana.com/api/dashboards/19665/images/14905/image)

## Infrastructure
The main infrastructure needed are a k8s clusters (for running the tests and other supporting services) and an Azure Monitor Workspace for storing the Prometheus metrics generated by the test runs.

Some of the main requirements from the cluster are: [the enablement of OIDC issuer and workload identity](https://learn.microsoft.com/en-us/azure/aks/workload-identity-deploy-cluster), which are needed for example to configure Prometheus to write metrics into the Azure Monitor Workspace. [Entra ID with Kubernetes RBAC](https://learn.microsoft.com/en-us/azure/aks/azure-ad-rbac?tabs=portal) so that we can define permissions per namespace and per user type/role. And the [deployment of multiple node pools](https://learn.microsoft.com/en-us/azure/aks/manage-node-pools) with different labels in order to be able to define [where specific workloads need to run on](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/).

The amount of node pools needed will vary depending on the use cases we end up supporting and it's a relatively easy process to add and remove node pools from the cluster. (TODO: I had some issues a few years ago where if we scaled a node pool to zero it would not scale up when workloads needed to be scheduled. There are mitigations for this but hopefully the nodes will scale up automatically now.) The process of adding the necessary config into the TestRun k8s manifest should be abstract from the users to avoid silly misconfigurations.

The cluster should also be configured in a while requires the least amount of maintenance possible, e.g. by [allowing automatic updates](https://learn.microsoft.com/en-us/azure/aks/auto-upgrade-cluster?tabs=azure-cli#cluster-auto-upgrade-channels).


To visualize the data  stored in the Azure Monitor Workspace, we need to add a azure_monitor_workspace_integrations block in the centralized monitoring azurerm_dashboard_grafana. A new datasource will then available in Grafana for data querying.

Azure also provides a few out-of-the-box dashboards that can be used to monitor the state of the cluster. We also import other OSS dashboards as needed; such as the [K6 operator dashboard](https://grafana.com/grafana/dashboards/19665-k6-prometheus/).

### Services
There are also a few services we need to maintain; mainly a Prometheus instance that is used as [the remote write target by the test pods](https://grafana.com/docs/k6/latest/results-output/real-time/prometheus-remote-write/) which then [forwards the metrics to the Azure Monitor Worspace](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-remote-write-managed-identity). Currently, the config is quite simple. The Prometheus instance was deployed via kube-prometheus-stack's Helm Chart together with AlertManager. Prometheus needs to be configured to use Workload Identity in order for it to be able to push metrics to the Azure Monitor Workspace. The rest of the prometheus configs tweaked so far were: Addition of externalLabels (likely not needed if we only use a single cluster), enableRemoteWriteReceiver to support receiving metrics via Remote Write from the test pods, a low retention period as the objective at the moment is only to keep the metrics long enough until they are remote writed to AMW (This might need to be tweaks depending on how we end up using AlertManager), configuration of the volumeClaimTemplate to select an appropriate disk type and size, and a remote write configuration block that points to the Azure Monitor Workspace. The K8s manifests also need some tweaks, mainly the ServiceAccount and Pod need some Workload Identity Labels and Annotations respectively.

The other major service we need is the [k6-operator](https://grafana.com/docs/k6/latest/testing-guides/running-distributed-tests/) which is responsible for actually running the tests based on the TestRun manifests being applied to the cluster. The k6 operator is also deployed via a Helm Chart.

The last service is [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) which can be used by developers that need to inject any sort of secrets into the cluster. Sealed Secrets allows for Secrets to be encrypted locally and commited to a Github repo. Only the controller running in the cluster is able to decrypt the secrets.



### Potential Use-Cases
The [Grafana K6 documentation](https://grafana.com/docs/k6/latest/testing-guides/automated-performance-testing/#model-the-scenarios-and-workload) has a lot of good information to get started.
- Smoke Tests: Validate that your script works and that the system performs adequately under minimal load.

- Soak Test: assess the reliability and performance of your system over extended periods.

- Average-load test: assess how your system performs under expected normal conditions.

- Stress test: assess how a system performs at its limits when load exceeds the expected average.

# Drawbacks
[drawbacks]: #drawbacks

# Rationale and alternatives
[rationale-and-alternatives]: #rationale-and-alternatives

The ideas is to share resources between teams and have a 'golden path' for testing.
There is currently a request to be able to assign costs to different teams that I haven't considered.
I'm not sure if tools such as kubecost can make these calculations for us or if we need to, for example,
dedicate specific node pools per team and rely on tags/labels on the nodes to assign cost to individual teams.
The initial design was to share resources as much as possible to keep costs down.

# Prior art
[prior-art]: #prior-art

TODO: Get an overview of what Dagfinn, Core? and other teams were doing previouly.

# Unresolved questions
[unresolved-questions]: #unresolved-questions


# Future possibilities
[future-possibilities]: #future-possibilities

- Simplify manifest generation. Most of the setup is boilerplate so we should be able to abstract most things.
- Add support to deploy the tests with a volume mount instead of a ConfigMap.
- Improve Dashboards experience, e.g. easy linking between resource usage (both for individual pods as nodes), tracing, logs, etc.
- Slack and/or Github integration so teams receive feedback of their test runs.
- Store pod logs and integrate the log solution from Microsoft with Grafana.
