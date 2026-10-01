# Syncroot publishing

`terraform.tfvars.json` maps product names to their publishing repositories and
allowed GitHub branches/environments. Product names may contain single hyphens
between alphanumeric segments and become ACR prefixes such as
`access-management/syncroot`.

Several products can share a repository. Set `repo_identity_products[repo_name]`
to an existing product in that repository. That product owns the Terraform
module, Azure identity and `DIS_SYNCROOT_AZURE_*` repository secrets; its registry
write condition includes all products sharing the repository. All shared entries
must specify identical branches and environments. Changing the selected identity
product can replace the publishing identity and its credentials.

`altinn-auth` uses `access-management` as its identity product. Tear down the old
`accessmanagement` module before applying this configuration. Both modules use
the same GitHub secret names, so deleting the old secrets after creating the new
ones would remove the new credentials. The new Azure identity has a new client
ID and grants access to the configured product prefixes. Coordinate the teardown,
publisher and namespace changes with `altinn-auth` and `dis-way/core` before
publishing or switching the at22 bootstrap.

After deployment, the workflow seeds missing environment tags with the default
syncroot. Adding an entry does not configure a product's publisher or bootstrap
it in a cluster.

Validate locally without cloud credentials (the workflow pins the Terraform version):

```sh
terraform init -backend=false
terraform validate
terraform test
```
