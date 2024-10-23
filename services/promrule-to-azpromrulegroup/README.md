# promrule-to-azpromrulegroup
This controller is intended to sync PrometheusRule CRs created by Pyrra (and not picked up by the Managed Prometheus) with Azure PrometheusRuleGroups.

```mermaid
    sequenceDiagram
    participant User
    participant K8s
    participant Pyrra
    participant ThisController
    participant Azure
    User->>K8s: Deploy a Pyrra CR
    K8s->>Pyrra: A new SLO CR has been created
    Pyrra->>K8s: Create PrometheusRule CR
    K8s ->> ThisController: A new PrometheusRule CR has been created
    ThisController ->> K8s: Fetch the object

    alt Object is marked for deletion
        alt Object has our finalizer
            ThisController->> Azure: Delete PrometheusRuleGroup(s)
            alt Successfully deleted all PrometheusRuleGroup(s)
                ThisController->> ThisController: Remove finalizer from Object
                ThisController->> K8s: Update Object
            else Did not Successfully delete all PrometheusRuleGroup(s)
                ThisController->> ThisController: Requeue the request
            end
        else Object does not have our finalizer
            ThisController->> ThisController: Stop the reconciliation
        end
    else Object is not marked for deletion
        alt Object has been marked with our finalizer
            alt Object has our annotations
                ThisController->> ThisController: Fetch metadata stored in the Annotations
                ThisController->> ThisController: Regenerate the ARM Template based on the PrometheusRule
                alt The new template and the old are the same
                    ThisController->> Azure: Re-apply the template in case manual changes have been made to the PrometheusRuleGroup(s)
                else The new and old templates are not the same
                    alt Resources have been deleted from PrometheusRule
                        ThisController->> Azure: Delete the corresponding PrometheusRuleGroup(s)
                    else Resources have been added to the PrometheusRule
                        ThisController->> Azure: Apply the new ARM template
                    end
                end
            else Object does not have our annotations
                ThisController ->> ThisController: Generate ARM template from PrometheusRule
                ThisController ->> ThisController: Get the name(s) of the resources to be created
                ThisController ->> Azure: Deploy the ARM template
                alt ARM deployment was Successful
                    ThisController->> ThisController: Add metadata as Annotations to the CR
                    ThisController->> K8s: Update CR
                else ARM deployment was not Successful
                    ThisController->>ThisController: Requeue
                end
            end
        else Object has not been marked with our finalizer
            ThisController ->> ThisController: Add our finalizer to the object
            ThisController ->> K8s: Update Object
        end
    end
```
## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Getting Started

### Prerequisites
- go version v1.22.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/promrule-to-azpromrulegroup:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/promrule-to-azpromrulegroup:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/promrule-to-azpromrulegroup:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/promrule-to-azpromrulegroup/<tag or branch>/dist/install.yaml
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
