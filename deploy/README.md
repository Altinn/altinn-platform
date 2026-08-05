# deploy

Kubernetes manifests this repo ships to the clusters.

CI publishes each subfolder to the container registry
(`altinncr.azurecr.io/dis/...`) as a Flux OCI artifact. Flux running on the
clusters pulls the artifact and applies it. So: edit a file here, run the
matching publish workflow, and the change reaches the clusters a few minutes
later.

Only the manifests live here. The CI identity allowed to push them is set up
separately in `infrastructure/syncroots/`.

## Folders

| Folder | Published as | Applied on | What it is |
|---|---|---|---|
| `common/` | `dis/syncroot` | the DIS cluster in each environment | the `product-dis` namespace and DIS apps (e.g. dis-console) |
| `admin/` | `dis/syncroot-admin` | the adminservices cluster | the shared dis-console database server and its admin identity |
| `templates/` | one artifact per subfolder | stamped onto a cluster by Terraform | manifests with `${PLACEHOLDER}` values |

### common/ and admin/

`base/` holds shared resources. Each environment folder is what Flux applies in
that environment. A cluster pulls the artifact tag for its environment and
applies the folder with the same name — e.g. the `at22` cluster pulls tag
`at22` and applies `common/at22`.

### templates/

These are **not** applied as-is — the files contain `${PLACEHOLDER}` values.
The `dis-pgsql-tenant-access` Terraform module fills in the per-cluster values
and tells Flux to apply the result. Use this when each cluster needs the same
manifests with different values.

- `dis-console-tenant-db-template/` — applied on the **admin** cluster. Gives a
  tenant its own database on the shared server.
- `dis-console-tenant-app-template/` — applied on the **tenant** cluster. Runs
  dis-console there, pointed at that database.

## Ship a change

1. Edit the files.
2. Run the publish workflow for the target environment (GitHub Actions):
   - `common/` → **Publish DIS Syncroot Artifact**
   - `admin/` and `templates/` → **Publish DIS Admin Syncroot Artifacts**
3. Flux applies it on the clusters within ~5 minutes.

## Conventions

- Plain YAML, no inline comments. Put the "why" in this README or the pull request.
- The environment is the artifact tag: `at22 at23 at24 tt02 yt01 prod` for
  `common/`, `test` for `admin/` and `templates/`.
