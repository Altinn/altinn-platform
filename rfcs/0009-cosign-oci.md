- Feature Name: `cosign-oci-gitops`
- Start Date: 2026-02-25
- RFC PR: [altinn/altinn-platform#3151](https://github.com/altinn/altinn-platform/pull/3151)
- Github Issue: N/A
- Product/Category: Platform / SRE
- State: **REVIEW** (possible states are: **REVIEW**, **ACCEPTED** and **REJECTED**)

# Summary
[summary]: #summary

Secure the GitOps supply chain by publishing Kubernetes manifests (plain YAML / Kustomize) as OCI artifacts into Azure Container Registry (ACR) and enforcing that Flux only deploys artifacts that are keyless Cosign signed by our GitHub Actions OIDC identity. Environment promotion is performed by retagging a known immutable digest to a deployment group tag (`at_ring1`, `at_ring2`, `tt_ring1`, `tt_ring2`, `prod_ring1`, `prod_ring2`) and re-signing it, without rebuilding the artifact.

# Motivation
[motivation]: #motivation

Our current GitOps setup lacks a cryptographic guarantee that what Flux deploys is what CI produced. An attacker (or misconfiguration) could push arbitrary manifests to ACR and have them deployed. This RFC closes that gap by:

- Making ACR the **single distribution channel** for manifests (no separate "config repo" for artifacts).
- Keeping an **immutable digest history** — once published, a `sha-<shortsha>` tag never changes.
- Tying **promotion** (moving a deployment group tag to a new digest) to a cryptographic signature anchored to the GitHub Actions OIDC identity.
- Letting Flux **reject unsigned or tampered artifacts** before they are applied to the cluster.

Expected outcome: only manifests built and signed by a trusted GitHub Actions workflow on a trusted ref can reach any environment.

# Guide-level explanation
[guide-level-explanation]: #guide-level-explanation

## Key concepts

**OCI artifact** — a manifest directory packaged as an OCI image layer and stored in ACR alongside container images. Flux's `OCIRepository` source type knows how to pull and unpack these.

**Immutable tag** — `sha-<shortsha>` — written once per commit, never moved.

**Deployment group tag** — `at_ring1`, `at_ring2`, `tt_ring1`, `tt_ring2`, `prod_ring1`, `prod_ring2` — a mutable pointer to a digest. Moving this tag is promotion.

**Keyless Cosign signing** — instead of a long-lived key pair, the CI job obtains a short-lived certificate from Sigstore's Fulcio CA, bound to the GitHub OIDC token. The certificate encodes the workflow identity. Signatures are stored as OCI objects in the same registry.

**Flux verification** — `OCIRepository.spec.verify` instructs Flux to call Cosign before trusting any pulled artifact. If verification fails, the source is not marked ready and nothing is deployed.

## How a team member should think about this

1. **Merging to main is not the same as deploying.** A merge creates an immutable artifact (`sha-*`) but does not move any environment tag.
2. **Promotion is an explicit, auditable action.** A workflow (manual or automated) retags the digest to a deployment group tag and signs it. Flux picks up the change within its poll interval.
3. **Flux enforces the policy.** Even if someone manually pushes to ACR, Flux will refuse to deploy it unless it carries a valid Cosign signature matching the trusted issuer and subject.
4. **Signatures are digest-anchored.** Moving a deployment group tag to a new digest invalidates the old signature for that tag. The new digest must be independently signed before Flux will deploy it.

## Example: promoting `at_ring1` to a new version

```
# 1. CI pushes immutable artifact on merge
flux push artifact oci://altinncr.azurecr.io/manifests/myapp:sha-abc1234 ...

# 2. Promote workflow retags
flux tag artifact oci://altinncr.azurecr.io/manifests/myapp:sha-abc1234 \
  --tag at_ring1

# 3. Promote workflow signs
cosign sign --yes altinncr.azurecr.io/manifests/myapp:at_ring1

# 4. Flux detects the new digest on :at_ring1, verifies signature, applies manifests
```

# Reference-level explanation
[reference-level-explanation]: #reference-level-explanation

## Artifact layout in ACR

```
altinncr.azurecr.io/manifests/<app>:sha-<shortsha>   # immutable
altinncr.azurecr.io/manifests/<app>:<group>           # mutable, deployment group tag
```

Deployment group tags: `at_ring1`, `at_ring2`, `tt_ring1`, `tt_ring2`, `prod_ring1`, `prod_ring2`.

## End-to-end flow

### 1) Pull Request
CI runs validation (kustomize build, kubeconform, policy checks). No artifacts are published or deployment group tags moved.

### 2) release-please
On merge to the default branch, release-please updates the changelog and opens/merges a release PR. This triggers the build step.

### 3) Build & push OCI artifact
CI packages the manifest directory using `flux push artifact`, pushes an immutable tag, and immediately signs it with keyless Cosign.

### 4) Retag to environment
A promotion workflow (manual `workflow_dispatch` or automated) retags the digest to a deployment group tag using `flux tag artifact`. This does not rebuild the artifact.

### 5) Sign (keyless Cosign)
The promotion workflow signs the deployment group tag. Cosign fetches an OIDC token from GitHub Actions, obtains a short-lived certificate from Fulcio, signs the manifest digest, and stores the signature in ACR.

> Signing the deployment group tag is safe even though it is mutable: the signature is anchored to the digest. If the tag is later moved to a different digest, the old signature does not validate for the new digest.

## Required permissions

**GitHub Actions workflows** must have:

```yaml
permissions:
  id-token: write   # required for keyless Cosign OIDC token
  contents: read
```

**Azure / ACR** — workflows authenticate via Azure Workload Identity Federation (OIDC) using `azure/login`, then `az acr login`. No long-lived secrets stored in GitHub.

**Flux** — pulls from ACR using Azure Workload Identity. The Flux controller's pod identity is federated with an Azure managed identity that has `AcrPull` on the registry.

## Reference implementation

### A) GitHub Actions — publish, retag, and sign

```yaml
name: publish-manifests

on:
  workflow_dispatch:
    inputs:
      env_tag:
        description: "Deployment group tag to move"
        required: true
        type: choice
        options: [at_ring1, at_ring2, tt_ring1, tt_ring2, prod_ring1, prod_ring2]

permissions:
  id-token: write
  contents: read

env:
  ACR_LOGIN_SERVER: altinncr.azurecr.io
  OCI_REPO: manifests/<app>      # change per app
  MANIFEST_PATH: ./kustomize     # change if needed

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Login to ACR
        run: az acr login --name "${ACR_LOGIN_SERVER%%.*}"

      - name: Install Flux CLI
        run: curl -s https://fluxcd.io/install.sh | sudo bash

      - name: Install Cosign
        uses: sigstore/cosign-installer@v3

      - name: Push immutable OCI artifact
        id: push
        run: |
          SHORT_SHA="${GITHUB_SHA::7}"
          IMM_TAG="sha-${SHORT_SHA}"
          IMM_REF="${ACR_LOGIN_SERVER}/${OCI_REPO}:${IMM_TAG}"

          flux push artifact "oci://${IMM_REF}" \
            --path="${MANIFEST_PATH}" \
            --source="${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}" \
            --revision="${GITHUB_SHA}"

          echo "IMM_REF=${IMM_REF}" >> $GITHUB_OUTPUT

      - name: Keyless cosign sign immutable tag
        env:
          COSIGN_EXPERIMENTAL: "1"
        run: |
          cosign sign --yes "${{ steps.push.outputs.IMM_REF }}"

      - name: Retag immutable -> deployment group tag
        run: |
          flux tag artifact "oci://${{ steps.push.outputs.IMM_REF }}" \
            --tag "${{ inputs.env_tag }}"

      - name: Keyless cosign sign deployment group tag
        env:
          COSIGN_EXPERIMENTAL: "1"
        run: |
          cosign sign --yes \
            "${ACR_LOGIN_SERVER}/${OCI_REPO}:${{ inputs.env_tag }}"
```

`flux tag artifact` retags by digest using the Flux CLI's native ACR provider support. `cosign sign` stores the signature alongside the artifact in ACR.

### B) Flux — OCIRepository watching deployment group tag with verification

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: <app>-manifests-at-ring1
  namespace: flux-system
spec:
  interval: 2m
  url: oci://altinncr.azurecr.io/manifests/<app>
  ref:
    tag: at_ring1
  verify:
    provider: cosign
    matchOIDCIdentity:
      - issuer: "^https://token\\.actions\\.githubusercontent\\.com$"
        subject: "^https://github\\.com/<ORG>/<REPO>/.github/workflows/publish-manifests\\.yml@refs/heads/main$"
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: <app>-at-ring1
  namespace: flux-system
spec:
  interval: 5m
  sourceRef:
    kind: OCIRepository
    name: <app>-manifests-at-ring1
  path: "./"
  prune: true
  wait: true
```

Repeat for `at_ring2`, `tt_ring1`, `tt_ring2`, `prod_ring1`, `prod_ring2` (changing `ref.tag` and resource names).

**Verification semantics:**
- `issuer` must match the GitHub Actions OIDC issuer exactly.
- `subject` must match the identity embedded in the signing certificate. The subject encodes the repo, workflow path, and ref. You can choose how strictly to constrain it:

| Level | Subject regex | Allows |
|-------|--------------|--------|
| Loose | `^https://github\.com/<ORG>/<REPO>/.*$` | Any workflow, any branch |
| Medium | `^https://github\.com/<ORG>/<REPO>/.*@refs/heads/main$` | Any workflow, main branch only |
| Strict | `^https://github\.com/<ORG>/<REPO>/\.github/workflows/publish-manifests\.yml@refs/heads/main$` | One workflow file, main branch only |

The stricter the subject, the stronger the supply chain policy. The example YAML above uses the strict form, which is the recommended level for production environments.

## Promotion models

**Option 1 — Manual (`workflow_dispatch`):** a team member selects the deployment group tag.

**Option 2 — Automated pipeline with approvals:** a separate workflow gates higher environments (e.g. `prod`) behind required reviewers using GitHub Environments. Signs only after all gates pass.

## Rollback

Rollback is retagging a deployment group tag back to an earlier immutable digest and re-signing it. Because immutable tags (`sha-*`) are never deleted, any previously deployed digest is still available in ACR.

```bash
# Move at_ring1 back to a known-good digest
flux tag artifact oci://altinncr.azurecr.io/manifests/<app>:sha-<previous-sha> \
  --tag at_ring1

# Re-sign the tag at the previous digest
cosign sign --yes altinncr.azurecr.io/manifests/<app>:at_ring1
```

Flux will detect the tag now points to a different (older) digest, verify the new signature, and reconcile to the previous state. No special rollback command is needed — it is the same promotion mechanism in reverse.

> Because immutable tags are signed at build time, the `sha-*` digest already has a valid signature. The `cosign sign` step above issues a new signature binding the deployment group tag to that same digest.

## Operational checks

**In-cluster:**
```bash
kubectl -n flux-system get ocirepository <app>-manifests-at-ring1 -o yaml
# Look for Ready condition and SourceVerified condition
```

**In CI (optional pre-promotion verify):**
```bash
COSIGN_EXPERIMENTAL=1 cosign verify \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --certificate-identity-regexp "https://github.com/<ORG>/<REPO>.*" \
  altinncr.azurecr.io/manifests/<app>:at_ring1
```

# Drawbacks
[drawbacks]: #drawbacks

- **Operational complexity:** teams must understand OCI artifact semantics, deployment group tags, and the sign-then-deploy model. Misconfigured subjects or issuers will cause Flux to refuse deployments.
- **Flux pull latency:** Flux polls on `interval`; there is no push-based trigger. A 2-minute interval means up to 2 minutes between promotion and deployment start.
- **ACR storage costs:** signatures and attestations are stored as additional OCI objects alongside each artifact. For high-frequency deployments this can add up.
- **Keyless signing requires network access to Sigstore:** the CI job must reach Fulcio and Rekor. Outages in Sigstore infrastructure would block signing.
- **Subject regex must be maintained:** as workflow files are renamed or branches change, the `subject` regex in `OCIRepository` manifests must be updated, or Flux will reject valid artifacts.

# Rationale and alternatives
[rationale-and-alternatives]: #rationale-and-alternatives

**Why OCI artifacts over a config repo (Git-based GitOps)?**
A separate Git config repo requires write access from CI, introduces merge conflicts on concurrent promotions, and provides no cryptographic binding between the artifact and the workflow that produced it. OCI + Cosign gives us immutability and provenance in a single system.

**Why keyless Cosign over a long-lived signing key?**
Long-lived keys must be stored as secrets, rotated, and revoked if compromised. Keyless signing delegates trust to the OIDC identity (the GitHub Actions workflow), which is already the access boundary we control. There is no key material to leak.

**Why deployment group tags over digest-pinning in Flux?**
Digest-pinning in Flux requires updating the `OCIRepository` manifest for every promotion — effectively recreating the config repo problem. Env tags let the workflow move the pointer while Flux watches a stable tag name.

**Alternatives not chosen:**
- **Helm charts as the distribution unit:** more complexity for teams that author plain YAML or Kustomize overlays.
- **Notation (CNCF) instead of Cosign:** Notation is not yet supported by Flux's `verify` field. Revisit when Flux adds support.
- **Hardware-backed keys (KMS):** possible as a future hardening step, but adds operational overhead for initial rollout.

**Impact of not doing this:** the current setup has no cryptographic guarantee on what Flux deploys. A compromised ACR credential or a misconfigured workflow could silently deploy tampered manifests.

# Prior art
[prior-art]: #prior-art

- **Flux OCI documentation** — Flux has first-class support for `OCIRepository` sources and Cosign verification since Flux v2.0. The approach follows the patterns documented by the Flux project.
- **Sigstore / Cosign** — widely adopted in the container/Kubernetes ecosystem (e.g., Chainguard, Google, GitHub's own artifact signing). The keyless flow is considered best practice for ephemeral CI identities.
- **SLSA (Supply-chain Levels for Software Artifacts)** — this RFC implements concepts aligned with SLSA Level 2/3: provenance attached to artifacts, signed by an automated build system.
- **GitHub's artifact attestations** — GitHub's own `actions/attest-build-provenance` uses the same Sigstore stack. Our approach is compatible and could be extended to include SLSA provenance attestations.

# Unresolved questions
[unresolved-questions]: #unresolved-questions

- **Alerting ownership:** who configures and owns alerts for `OCIRepository` verification failures?

# Future possibilities
[future-possibilities]: #future-possibilities

- **SLSA provenance attestations:** extend the CI workflow to attach a signed SLSA provenance attestation to each immutable artifact using `cosign attest`, enabling `cosign verify-attestation` checks in policy engines.
- **Notation support in Flux:** once Flux adds Notation verification support, we could evaluate migrating to the CNCF Notary v2 stack for compatibility with the broader CNCF ecosystem.
- **Automated promotion and canary releases:** automate ring progression (`at_ring1` → `at_ring2` → … → `prod_ring2`) with test gates and manual approval for prod. The ring structure maps naturally to canary: promote `prod_ring1` first, observe metrics, then `prod_ring2`. **Flagger** is a candidate for driving metric-gated progression and automatic rollback between rings.
- **Policy engine integration:** feed Flux verification events into Kyverno policies for additional admission-time checks beyond what Flux enforces at the source level.
- **Multi-registry mirroring:** mirror signed artifacts to a secondary registry for disaster recovery, with digest preservation and re-verification after copy.
