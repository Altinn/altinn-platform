# dis-pgsql-operator

## Description

`dis-pgsql-operator` provides self-service PostgreSQL provisioning for app teams.
It reconciles DIS storage APIs into Azure PostgreSQL Flexible Server resources
through Azure Service Operator (ASO), and provisions app/owner access inside
PostgreSQL with Kubernetes Jobs.

## API Model

The operator exposes two storage APIs:

- `DatabaseServer` provisions and configures an Azure PostgreSQL Flexible Server.
- `Database` provisions a PostgreSQL database on a same-namespace
  `DatabaseServer`.

`Database.spec.server.name` selects the `DatabaseServer`. `Database.spec.name`
is the PostgreSQL database name and maps directly to the ASO
`FlexibleServersDatabase.spec.azureName` field.

Dedicated and multitenant layouts use the same APIs:

- Dedicated: one `Database` per `DatabaseServer`.
- Multitenant: many `Database` resources on one shared `DatabaseServer`.

## Connection ConfigMaps

Once a `Database` is fully ready (its Azure resources exist and access has been
provisioned), the operator publishes one **non-secret** ConfigMap per
`identityRef` access principal so consuming apps can read the connection
coordinates declaratively. Access principals declared as Entra `group`s or
`servicePrincipal`s do not get a ConfigMap (there is no `ApplicationIdentity`
to derive a consumer from).

The ConfigMap name is derivable before the database is deployed, from values
known at authoring time:

```
<database.metadata.name>-<identityRef.name>-dis-pgsql
```

The name is lowercased/sanitized to a valid DNS-1123 name. If it would exceed 63
characters it is truncated and given a deterministic hash suffix — in that case,
select the ConfigMap by labels rather than recomputing the name.

Data keys (CloudNativePG-style):

| key       | value                                                              |
|-----------|--------------------------------------------------------------------|
| `host`    | PostgreSQL server FQDN                                              |
| `port`    | `5432`                                                             |
| `dbname`  | database name                                                      |
| `user`    | the resolved managed-identity / Postgres role the app connects as  |
| `sslmode` | `require`                                                          |
| `uri`     | `postgresql://<user>@<host>:<port>/<dbname>?sslmode=require`        |

There is **no password / pgpass** key: authentication is Entra (Azure AD) token
based, so the ConfigMap holds no secrets.

Labels (the binding contract; a consumer or a kro resource graph can select on
these even when the name is hash-suffixed):

- `pgsql.dis.altinn.cloud/database`: the `Database` name
- `pgsql.dis.altinn.cloud/principal`: the `identityRef.name`
- `pgsql.dis.altinn.cloud/component`: `connection`

The ConfigMap is owned by the `Database`, so it is garbage-collected when the
`Database` is deleted, and removed when its principal is dropped from
`spec.access.principals`.

Apps can consume it directly with `envFrom`:

```yaml
envFrom:
  - configMapRef:
      name: <database>-<identityRef>-dis-pgsql
```

## Getting Started

### Prerequisites
- go version v1.24.6+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/dis-pgsql-operator:tag
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
make deploy IMG=<some-registry>/dis-pgsql-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples from `config/samples`:

```sh
kubectl apply -k config/samples/
```

The samples include both a dedicated layout and a multitenant layout.

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

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/dis-pgsql-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/dis-pgsql-operator/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v2-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
