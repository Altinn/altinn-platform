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
