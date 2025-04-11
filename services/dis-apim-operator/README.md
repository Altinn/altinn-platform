# dis-apim-operator
// TODO(user): Add simple overview of use/purpose

## Description
// TODO(user): An in-depth paragraph about your project and overview of use

### Overview
The dis-apim-operator is a Kubernetes operator that manages the deployment of APIs in Azure API Management (APIM) using custom resources (CRs). 

It provides a way to define and manage APIs, API versions, API diagnostics, Backends, and policies in a declarative manner.

It leverages the [Azure/azure-sdk-for-go](https://github.com/Azure/azure-sdk-for-go) to interact with the Azure API Management service and perform operations such as creating, updating, and deleting APIs and their associated resources.

### Features

#### Custom Diagnostics settings for API

It is possible to configure custom diagnostics settings for the API. This is done by defining the diagnostics section for the API Version.

If the logs should be sent to another Applications Insights than the default one the owner of the APIM must [define a custom logger in the APIM](https://learn.microsoft.com/en-us/azure/api-management/api-management-howto-app-insights?tabs=rest).

Once the custom logger is created the owner of the APIM must inform the owner of the API about the name of the logger (in most cases the name of the application insights).

The LoggerName is then specified in the diagnostics section of the API version, the operator will the lookup the loggerID based on the name. This is done using the api [Logger - List By Service](https://learn.microsoft.com/en-us/rest/api/apimanagement/logger/list-by-service?view=rest-apimanagement-2024-05-01&tabs=HTTP) with the [azure-sdk-for-go](https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/resourcemanager/apimanagement/armapimanagement/logger_client.go) This is to not having to define the long loggerID in the CR.

## Getting Started

### Prerequisites
- go version v1.22.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/dis-apim-operator:tag
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
make deploy IMG=<some-registry>/dis-apim-operator:tag
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
make build-installer IMG=<some-registry>/dis-apim-operator:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/dis-apim-operator/<tag or branch>/dist/install.yaml
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2024 altinn.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

