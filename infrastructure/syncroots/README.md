# Syncroot publishing

`terraform.tfvars.json` maps product names to their publishing repositories and
allowed GitHub branches/environments. Product names may contain single hyphens
between alphanumeric segments and become ACR prefixes such as
`access-management/syncroot`.

Several products can share a repository. Set `repo_identity_products[repo_name]`
to an existing product in that repository. That product keeps its Terraform
address, Azure identity and `DIS_SYNCROOT_AZURE_*` repository secrets; its registry
write condition includes all products sharing the repository. All shared entries
must specify identical branches and environments. Changing the selected identity
product can replace the publishing identity and its credentials.

`altinn-auth` retains `accessmanagement` as its identity product and a legacy
artifact prefix because its publisher and the at22 bootstrap still use that name.
The new `access-management` prefix is available alongside it. Migrating the
existing namespace, Flux resources, RBAC and workload identities requires
coordinated changes in `altinn-auth` and `dis-way/core`.

After deployment, the workflow seeds missing environment tags with the default
syncroot. Adding an entry does not configure a product's publisher or bootstrap
it in a cluster.

Validate locally without cloud credentials (the workflow pins the Terraform version):

```sh
terraform init -backend=false
terraform validate
terraform test
```
