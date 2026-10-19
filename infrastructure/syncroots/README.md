# Syncroot publishing

`terraform.tfvars.json` maps product names to their publishing repositories and
allowed GitHub branches/environments. Product names may contain single hyphens
between alphanumeric segments and become ACR prefixes such as
`access-management/syncroot`.

Products are grouped by `repo_name`. Each repository has one Terraform module,
publishing identity and set of `DIS_SYNCROOT_AZURE_*` secrets, with write access
to all its product prefixes. Shared entries must specify identical branches; their environments are combined
on the shared identity. Adding or removing a product keeps the repository's identity and
credentials; removing its last product removes the publishing resources.

Each identity federates only on the OIDC subject format GitHub uses for its
repository. Repositories created, renamed or transferred after 2026-07-15, and
repositories that opt in, get the immutable format with the org and repo ids
(`repo:Altinn@<org id>/<repo>@<repo id>:...`); older ones keep the name-based format
(`repo:Altinn/<repo>:...`). Every plan reads the current format from
`GET /repos/{owner}/{repo}/actions/oidc/customization/sub`, which needs the Actions
(read) permission on the GitHub App. When a repository switches format, run this
workflow so its identity swaps credentials; publishing from that repository fails until
then. Entra allows at most 20 credentials per identity, one per environment and branch.
An identity near that limit may need a second apply after a switch, because the new
credentials can be created before the old ones are deleted.

Identity names use the organization and repository, such as
`Altinn-altinn-auth-syncroot`. This replaces all existing product-based publishing
modules. Tear down those modules before creating the replacements: both use the
same GitHub secret names, so later deletion of old secrets would remove the new
credentials. Coordinate Auth's publisher and namespace changes with `altinn-auth`
and `dis-way/core` before publishing or switching the at22 bootstrap.

After deployment, the workflow seeds missing environment tags with the default
syncroot. Adding an entry does not configure a product's publisher or bootstrap
it in a cluster.

Validate locally without cloud credentials (the workflow pins the Terraform version):

```sh
terraform init -backend=false
terraform validate
terraform test
```
