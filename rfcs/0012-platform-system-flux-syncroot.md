- Feature Name: platform_system_flux_syncroot
- Start Date: 2026-04-17
- RFC PR: [altinn/altinn-platform#0000](https://github.com/altinn/altinn-platform/pull/0000)
- Github Issue: [altinn/altinn-platform#0000](https://github.com/altinn/altinn-platform/issues/0000)
- Product/Category: CI/CD
- State: **REVIEW**

# Summary
[summary]: #summary

Provision the `platform-system` syncroot using the same team syncroot model already used for all product teams: Terraform azapi creates a single `OCIRepository` + `Kustomization` once, and the platform team manages all infrastructure components (cert-manager, linkerd, traefik, kyverno, external-secrets, operators, etc.) by publishing to the `platform-system` OCI artifact. This replaces the current approach of managing each infrastructure component as a separate Terraform `azapi_resource` `fluxconfiguration` ARM resource, eliminating a class of reliability problems caused by ARM API timeouts that cause Terraform to destroy and recreate running components.

# Motivation
[motivation]: #motivation

Platform infrastructure components — linkerd, traefik, cert-manager, various operators — are currently bootstrapped by Terraform using the Azure `azapi` provider to create `Microsoft.KubernetesConfiguration/fluxConfigurations` ARM resources. This approach has a fundamental operational problem:

**ARM apply operations are blocking and have a fixed timeout.** When the Azure control plane takes longer than expected to report readiness (due to slow node scheduling, image pulls, or transient API latency), Terraform marks the `fluxconfiguration` resource as failed. On the next plan/apply, Terraform concludes the resource must be destroyed and recreated. This takes down the running component — linkerd proxies drain, traefik stops serving ingress, operators lose their watches — during what should have been a no-op reconciliation.

Additionally, the `fluxconfiguration` ARM resource wraps Flux source and `Kustomization` CRDs inside an opaque Azure resource, making it harder to inspect, debug, or override behaviour using standard Flux tooling.

Flux `Kustomization` supports a `dependsOn` field for expressing ordering between components — for example, ensuring cert-manager is healthy before deploying an operator that requires its CRDs. When components are managed as `fluxconfiguration` ARM resources, this field is not available. Dependencies must instead be encoded as `depends_on` in Terraform, which only controls the order of ARM API calls during a Terraform run. It provides no ongoing guarantee: if a component is redeployed outside of Terraform, the dependency ordering is lost entirely.

The syncroot pattern is already proven and in production: most platform infrastructure components (cert-manager, linkerd, traefik, kyverno, external-secrets, otel-operator, azure-service-operator, dis-vault, and others) are already deployed this way, as are all product team deployments (`product-dialogporten`, `product-dis`, `product-infoportal`). The remaining components still managed via Terraform `azapi` `fluxconfiguration` should be migrated to complete the transition.

By completing this migration, we gain:

- **No destroy/recreate cycles**: Flux Kustomizations are applied in-cluster and reconcile asynchronously. A slow image pull or API hiccup does not cause Terraform to tear down a running component.
- **Native dependency ordering**: Flux `Kustomization` supports `dependsOn`, allowing components to declare ordering constraints (e.g. cert-manager before dependent operators) that are enforced continuously, not just during a Terraform run.
- **Standard Flux tooling**: `flux get`, `flux reconcile`, `flux suspend/resume` work as expected. No ARM API indirection.
- **Drift detection**: Flux continuously reconciles desired state, catching manual changes or partial failures without a Terraform run.
- **Consistency**: The platform team uses the same deployment model it provides to other teams, making it easier to reason about and operate.

# Guide-level explanation
[guide-level-explanation]: #guide-level-explanation

Every product team has a syncroot provisioned by Terraform azapi: one `OCIRepository` + `Kustomization` created once per cluster, after which the team owns the OCI artifact and deploys their products by publishing to it. This RFC applies the same model to the platform team itself — Terraform azapi provisions the `platform-system` syncroot once, and the platform team deploys all infrastructure components by publishing to the `platform-system` OCI artifact. There is no Terraform involvement in day-to-day component deployments.

**Adding a new infrastructure component**

Add the new component's `OCIRepository` and `Kustomization` (or `HelmRelease`) manifests to the `platform-system` OCI artifact and push a new tag. The root syncroot in `flux-system` will reconcile them into the `platform-system` namespace:

```
platform-system/
  cert-manager/
    ocirepository.yaml
    kustomization.yaml
  linkerd/
    ocirepository.yaml
    kustomization.yaml
  my-new-operator/          # new component
    ocirepository.yaml
    kustomization.yaml
```

Once the new artifact tag is pushed and the `OCIRepository` reconciles, Flux will deploy the component within its next interval. No Terraform run, no ARM API call, no waiting for a blocking apply to time out.

**Updating an existing component**

Update the relevant manifests and merge. Flux reconciles the diff. No pipeline trigger required.

**Observing deployment state**

Use standard Flux tooling — the same tooling used for application team deployments:

```
flux get kustomizations -n flux-system
flux reconcile kustomization platform-system
```

**Existing team members** should migrate components from `azapi fluxconfiguration` to the syncroot incrementally, one component at a time, verifying reconciliation before removing the Terraform resource.

**New team members** should treat the `platform-system` syncroot path as the default home for any infrastructure component they are asked to deploy — the same way application teams treat their own syncroot paths.

# Reference-level explanation
[reference-level-explanation]: #reference-level-explanation

## Syncroot structure

The structure already in production is a two-level hierarchy:

**Level 1 — root syncroot** (`flux-system` namespace): A single `OCIRepository` + `Kustomization` pair provisioned by Terraform azapi — the same mechanism used to provision syncroots for all product teams. This is done once per cluster. The artifact it points to contains the definitions for all child `OCIRepository` and `Kustomization` resources. Adding a new infrastructure component means adding its resources to this artifact and pushing a new tag; no Terraform change is required.

```
flux-system
  ocirepository/platform-system        # pulls the platform-system artifact
  kustomization/platform-system-...    # reconciles child resources into platform-system ns
```

**Level 2 — per-component resources** (`platform-system` namespace): Each infrastructure component has its own `OCIRepository` pointing to a component-specific artifact at an independent version, and a `Kustomization` (or `HelmRelease`) that reconciles it. Components currently deployed this way include: `cert-manager`, `linkerd`, `traefik`, `kyverno`, `kyverno-policies`, `external-secrets-operator`, `otel-operator`, `otel-collector`, `opencost`, `azure-service-operator`, `dis-vault`, `dis-vault-operator`, `dis-identity-operator`, `dis-identity-sync`, `dis-apim-operator`, `tls-issuer`.

```
platform-system
  ocirepository/cert-manager           # at_ring1@sha256:...
  kustomization/cert-manager-...

  ocirepository/linkerd                # at_ring1@sha256:...
  kustomization/linkerd-linkerd
  kustomization/linkerd-linkerd-crds
  kustomization/linkerd-linkerd-post-deploy

  ocirepository/traefik                # at_ring1@sha256:...
  kustomization/traefik-traefik
  ...
```

Product teams follow the same pattern in their own namespaces (`product-dialogporten`, `product-dis`, `product-infoportal`), using their own `OCIRepository` and `Kustomization` resources scoped to their namespace.

## Current state: azapi fluxconfiguration

Today, the `platform-system` syncroot itself and its child configurations (linkerd, traefik, operators, etc.) are created as `Microsoft.KubernetesConfiguration/fluxConfigurations` ARM resources via the Terraform `azapi` provider. Terraform's `azapi_resource` applies these synchronously: it calls the ARM API and waits for the resource to reach a terminal provisioning state before returning.

The failure mode is:

1. Terraform calls ARM to create or update a `fluxconfiguration`.
2. The component takes longer than expected to become healthy (slow image pull, node pressure, transient ARM API latency).
3. Terraform's operation times out; the resource is left in a non-terminal state from Terraform's perspective.
4. On the next `terraform apply`, Terraform decides the resource needs to be replaced (`-/+`), issuing a delete followed by a create.
5. The running component (linkerd, traefik, etc.) is destroyed and recreated, causing an outage.

This RFC removes the per-component `fluxconfiguration` ARM resources and replaces them with a single `platform-system` syncroot provisioned by Terraform azapi — exactly as team syncroots are provisioned today. All infrastructure components are then deployed as `OCIRepository` + `Kustomization` resources within the `platform-system` OCI artifact, managed entirely by the platform team without further Terraform involvement.

## Secret and variable management

> **This section describes a partially resolved design.** The mechanism for deploy-time variable substitution is known; how to reliably provision and sequence the values that feed into it across 200 clusters is the primary open question for this RFC. See [Unresolved questions](#unresolved-questions).

Today, secrets and configuration variables are stored as GitHub Actions secrets/variables and injected into Terraform at workflow runtime, which then post-renders them into `fluxconfiguration` manifests. This mechanism disappears in the new model. The intended replacement is Flux's `postBuild.substituteFrom`.

### OCI artifact templating

The platform deploys approximately 200 clusters. A single OCI artifact per component is published with `${VAR}` placeholders throughout all manifests. Flux resolves these at reconcile time per cluster:

```yaml
spec:
  postBuild:
    substituteFrom:
      - kind: ConfigMap
        name: platform-system-vars    # cluster identity and static config
        namespace: platform-system
      - kind: Secret
        name: platform-system-secrets # sensitive values from Key Vault via ESO
        namespace: platform-system
```

### What is decided

- **Static cluster vars** (cluster name, environment, region, resource group, non-sensitive resource names) are deployed as a `ConfigMap` (`platform-system-vars`) via a small dedicated Terraform `azapi` `fluxconfiguration`. A ConfigMap applies instantly with no health check complexity and will not trigger the timeout/destroy-recreate failure mode that affects full infrastructure components. This remains a Terraform-managed concern.
- **Key Vault** is provisioned by Terraform. Outputs from other Terraform deployments (connection strings, client IDs, etc.) are written directly to Key Vault — no Kubernetes API push.
- **ESO** is deployed as a child Kustomization in `platform-system`, pulling Key Vault secrets into a `Secret` in the `platform-system` namespace for use by `postBuild.substituteFrom`.
- **Runtime secrets** for workloads in other namespaces (`linkerd`, `traefik`, `cert-manager`, etc.) require a separate mechanism since a Secret in `platform-system` is not accessible to pods in other namespaces. ESO's `ClusterExternalSecret` is the candidate, but which components need this and how they are organised is not yet defined.

### What is not yet decided

See [Unresolved questions](#unresolved-questions) for the full list. The core open problems are bootstrap ordering, var ownership across components, and runtime secret access across namespaces.

## OCI artifact signing

Since all infrastructure components are deployed from OCI artifacts, signing those artifacts with cosign provides an important supply chain security control: Flux will refuse to apply any artifact that cannot be verified against the platform's public key. This means a tampered artifact, a compromised registry entry, or a manually pushed image cannot be reconciled onto a cluster.

Flux's `OCIRepository` supports cosign verification natively:

```yaml
spec:
  verify:
    provider: cosign
    secretRef:
      name: cosign-pub-key
```

Artifacts are signed as part of the CI pipeline (release-please PR merge → build → sign → push). Combined with the PR-based audit trail this gives end-to-end supply chain integrity: the change was approved via PR, the artifact was produced and signed by CI, and Flux verifies the signature before applying it.

This applies equally to the root `platform-system` artifact and to each per-component artifact. RFC 0009 (cosign-oci) covers the signing infrastructure in detail.

## Interaction with existing features

- **Terraform-managed Azure resources**: Resources that require Terraform (e.g. initial resource group creation, managed identities, role assignments) remain in Terraform. Terraform no longer manages `fluxconfiguration` resources for individual components, and no longer pushes values to the Kubernetes API. Terraform outputs are written to Key Vault.
- **RBAC and workload identity**: Any managed identity or role assignment required by a component must be provisioned before the Flux reconciliation runs. This is handled either by a `dependsOn` in the child Kustomization or by a preceding Terraform step that completes before the Flux sync interval fires.
- **GitHub Actions secrets/variables**: Sensitive values are migrated to Azure Key Vault (surfaced via ESO). Non-sensitive cluster-specific values move to the per-cluster `platform-system-vars` ConfigMap. The OCI artifact itself is cluster-agnostic and uses `${VAR}` placeholders throughout.

## Reconciliation and dependency ordering

Child Kustomizations can declare `dependsOn` references to express ordering constraints. For example, a component that requires a Key Vault to exist first should declare:

```yaml
spec:
  dependsOn:
    - name: platform-system-vault
```

## Health checks

Each child Kustomization should include `healthChecks` pointing to the resources it manages, so that Flux can report accurate readiness before dependent components proceed.

## Corner cases

- **Environment-specific overrides**: Use Kustomize patches in the environment-specific path to override values (e.g. SKU, replica count) without duplicating the base manifests.
- **Bootstrap order**: The `platform-system` syncroot itself must be bootstrapped before it can manage child components. The bootstrap process is handled once per cluster by the platform team.
- **Removal of components**: Deleting a child Kustomization from the syncroot path will cause Flux to prune the corresponding resources if `prune: true` is set. Care must be taken to verify there are no dependents before removing a component.

# Drawbacks
[drawbacks]: #drawbacks

- **Increased coupling**: All infrastructure components share a single reconciliation path. A broken manifest can block reconciliation of unrelated components unless child Kustomizations are sufficiently isolated.
- **Flux as a hard dependency**: Teams lose the ability to deploy infrastructure independently of Flux. Any Flux outage or misconfiguration blocks all infrastructure updates.
- **Migration effort**: Existing infrastructure deployed via pipelines or manual steps requires migration work before it benefits from this approach.

# Rationale and alternatives
[rationale-and-alternatives]: #rationale-and-alternatives

**Why this design?**

The syncroot pattern is already the established deployment model for application teams in the platform. Adopting it for platform infrastructure means the platform team eats its own cooking: using and validating the same mechanism it provides to others. Operators already know the tools. No new concepts are introduced.

**Alternatives considered**

- *Keep using azapi fluxconfiguration*: No migration effort, but the destroy/recreate problem remains. Every Terraform run that experiences a timeout risks an outage for linkerd, traefik, or other critical components. The ARM API indirection also makes it harder to use Flux CLI tooling to debug or intervene.
- *Increase Terraform azapi timeouts*: Mitigates some timeout failures but does not eliminate the fundamental issue. ARM resources are still replaced if Terraform loses track of their state, and longer timeouts slow down all CI runs even when nothing is wrong.
- *Per-team Flux syncroots via azapi*: Each team manages their own `fluxconfiguration`. Still subject to the same timeout/recreate problem, and fragments the operational model.
- *Argo CD*: Functionally similar to Flux for this use case. The platform already uses Flux, so switching would introduce unnecessary complexity.

**Impact of not doing this**

The destroy/recreate problem continues. Operators are periodically woken up to investigate why linkerd or traefik disappeared during a routine Terraform run. Trust in the deployment pipeline erodes.

# Prior art
[prior-art]: #prior-art

- **Existing team syncroots**: Terraform azapi provisions a syncroot (`OCIRepository` + `Kustomization`) per team. Teams then publish Flux manifests to their OCI artifact to deploy their products. This RFC applies the same two-step model to platform infrastructure: the root syncroot is provisioned once, and the platform team manages component deployments by publishing to the `platform-system` OCI artifact.
- **RFC 0009 — Cosign OCI**: Establishes artifact signing for OCI images in the platform. This RFC directly benefits from that work: by moving all infrastructure component deployments to OCI artifacts, every deployment can be covered by cosign verification. Flux's native cosign support means signature verification is enforced at reconcile time with no additional tooling.
- **RFC 0008 — Release Please**: All deployments are driven by PRs with release-please managing versioning. This provides a clear audit trail — who approved and merged a change is recorded in Git history — making the "who triggered this deployment" question already solved.
- **RFC 0001 — Pull-based CD**: Established the GitOps pattern for application deployments using Flux. This RFC extends the same pattern to infrastructure components managed by the platform team itself.
- **Flux multi-tenancy model**: The Flux project documents a pattern where a single root Kustomization aggregates per-team or per-component child Kustomizations, which is directly applicable here.

# Unresolved questions
[unresolved-questions]: #unresolved-questions

## Primary blocker: secret and variable injection

Secret and variable injection is the biggest unsolved design problem for this migration. The questions below must be resolved before components that depend on injected values can be migrated.

**Bootstrap ordering**
The `platform-system-vars` ConfigMap (from Terraform azapi) and ESO (from the syncroot) must both be ready before any component that uses `postBuild.substituteFrom` can reconcile successfully. Flux will retry on failure, but on a fresh cluster the ordering is:
1. Terraform provisions cluster + Key Vault + writes secrets to Key Vault
2. Terraform deploys `platform-system-vars` ConfigMap via azapi fluxconfiguration
3. Root syncroot deploys ESO into `platform-system`
4. ESO creates `platform-system-secrets` Secret
5. Remaining components reconcile with both ConfigMap and Secret available

Step 3 requires the root syncroot to be bootstrapped first. Step 4 requires ESO's own `ClusterSecretStore` to be configured with the Key Vault URL — which itself may be a `${VAR}` from the ConfigMap. The exact sequence and any gaps need to be validated on a real cluster before the migration proceeds.

**Var ownership and inventory**
There is no current inventory of which vars each component needs, which are sensitive (Key Vault) vs. static (ConfigMap), and which are available at cluster bootstrap time vs. provisioned later by other Terraform deployments. This must be mapped out per component before migration.

**Runtime secrets across namespaces**
Workloads in `linkerd`, `traefik`, `cert-manager` and other namespaces cannot read Secrets from `platform-system` at runtime. It is not yet clear which components need runtime secret access (as opposed to deploy-time substitution), and whether `ClusterExternalSecret` is the right mechanism or whether deploy-time substitution into HelmRelease values covers all cases.

**Key Vault topology**
Should there be one Key Vault per cluster or one per environment? Per-cluster gives tighter blast-radius isolation. Per-environment reduces the number of vaults to manage at 200-cluster scale. This decision affects how Terraform provisions vaults and how ESO `ClusterSecretStore` resources are configured.

## Migration

- What is the safe migration sequence for removing a `fluxconfiguration` ARM resource while its child Flux resources continue running? Deleting the ARM resource may also delete the in-cluster `OCIRepository` and `Kustomization` unless the component is first re-adopted by the `platform-system` syncroot.

## Other

- Should the `platform-system` syncroot use a dedicated service account with least-privilege RBAC, or the default Flux service account?
- Should health check failures in one child Kustomization block the reconciliation of sibling components, or should failures be isolated?

# Future possibilities
[future-possibilities]: #future-possibilities

- **Progressive delivery for infrastructure**: Once all infrastructure flows through the syncroot, it becomes feasible to introduce staged rollouts (e.g. deploy to `dev` → validate → promote to `prod`) using Flux image automation or a promotion controller.
- **Policy enforcement**: A shared syncroot is a natural integration point for policy engines such as Kyverno or OPA Gatekeeper — policies applied at the syncroot level would cover all infrastructure components automatically.
- **Observability**: Centralizing deployments makes it easier to build a unified dashboard showing the reconciliation status of all infrastructure across all environments.
- **Self-service infrastructure**: With a well-defined syncroot structure, teams could submit pull requests to add their own infrastructure components without needing platform-team involvement for each deployment.
