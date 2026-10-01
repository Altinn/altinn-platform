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

`altinn-auth` uses `access-management` as its identity product. `moved.tf` migrates
the former `accessmanagement` module address so existing GitHub secrets retain
their Terraform ownership. Renaming the Azure identity creates a new client ID;
Terraform updates the repository secret to match. The old artifact prefix loses
write access. Coordinate the publisher and namespace migration with `altinn-auth`
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
