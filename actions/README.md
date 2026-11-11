# Actions

Reusable GitHub Actions for the Altinn platform.

> [!WARNING]
> **The actions that other repositories consume are being moved to
> [`dis-way/actions`](https://github.com/dis-way/actions).**
> They have been duplicated there under the `altinn/` namespace and are
> deprecated here. Each deprecated action emits a `::warning::` annotation when
> it runs. Nothing has been removed yet — existing workflows keep working — but
> update `uses:` references to the new location when convenient.

## Migration status

| Action | Status | New location |
| --- | --- | --- |
| `terraform/plan` | Deprecated | `dis-way/actions/altinn/terraform/plan` |
| `terraform/apply` | Deprecated | `dis-way/actions/altinn/terraform/apply` |
| `terraform/init` | Deprecated | `dis-way/actions/altinn/terraform/init` |
| `terraform/write-terraform-summary` | Deprecated | `dis-way/actions/altinn/terraform/write-terraform-summary` |
| `flux/build-push-image` | Deprecated | `dis-way/actions/altinn/flux/build-push-image` |
| `flux/retag-image` | Deprecated | `dis-way/actions/altinn/flux/retag-image` |
| `flux/verify-syncroot` | Deprecated | `dis-way/actions/altinn/flux/verify-syncroot` |
| `flux/setup-flux-acr` | Deprecated | `dis-way/actions/altinn/flux/setup-flux-acr` |
| `terraform/plan-only` | **Stays here** | — |
| `terraform/apply-only` | **Stays here** | — |
| `generate-k6-manifests` | **Stays here** | — |
| `terraform/azure-app-token` | **Stays here** (unused) | — |

Only the actions consumed by repositories *outside* this one are moving —
`terraform/plan`, `terraform/apply`, `flux/build-push-image`,
`flux/retag-image` and `flux/verify-syncroot` — together with the three they
call internally (`terraform/init`, `terraform/write-terraform-summary`,
`flux/setup-flux-acr`).

`flux/setup-flux-acr` is deprecated but does not emit a runtime warning: it has
no direct consumers, so the annotation would only ever fire transitively and
would double up the warning already emitted by `flux/build-push-image` and
`flux/retag-image`.

## Why four actions stay

**`terraform/plan-only`** and **`terraform/apply-only`** are used only by this
repository's own [`k6tests-rg-deploy`](../.github/workflows/k6tests-rg-deploy.yml)
workflow. Nothing outside `altinn-platform` references them, so there is no
migration to do.

**`generate-k6-manifests`** cannot move cleanly. The `generate-k6-manifests`
binary is not built from the sources in this folder at action runtime — it is
compiled into `ghcr.io/altinn/altinn-platform/k6-action-image` by
[`infrastructure/images/k6-action/Dockerfile`](../infrastructure/images/k6-action/Dockerfile),
whose build context is this repository's root. The action's own `Dockerfile`
only layers `generate.sh` on top of that image. The generator also resolves
`ghcr.io/altinn/altinn-platform/k6-image` as the k6 runner and reads cluster
config baked into the image at `/actions/generate-k6-manifests/infra/`.

Moving the action without also moving `infrastructure/images/k6-action/` would
produce a repository where editing the Go sources changes nothing at runtime —
a trap rather than a migration. Its only consumer is
`Altinn/altinn-platform-validation-tests`.

**`terraform/azure-app-token`** has no consumers — not in this repository and
not in any other repository that references
`Altinn/altinn-platform/actions`. `dis-way/actions` already has its own copy.
Duplicating it would carry dead code across; it should be kept or deleted here
on its own merits.

## Migrating a workflow

Replace the `uses:` path and keep the `with:` block unchanged:

```diff
-  uses: Altinn/altinn-platform/actions/terraform/plan@<hash>
+  uses: dis-way/actions/altinn/terraform/plan@<hash>
```

> [!CAUTION]
> Do **not** repoint at `dis-way/actions/terraform/plan` (without `altinn/`).
> That is a different, independently developed action: it uses a different Azure
> storage account, resource group and subscription for Terraform state, and a
> different state-key format. Unknown inputs are only a warning in GitHub
> Actions, not an error, so a wrong path silently falls back to defaults and
> plans against an empty state.
