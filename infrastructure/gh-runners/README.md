# GitHub Runners

Deploys private GitHub Actions runners as Azure Container App Jobs in the `altinn-org-gh-runners` resource group.

## Resources

- One shared resource group (`altinn-org-gh-runners`)
- Per repository: a VNet, subnet, Key Vault, and Container App Job

## Adding a repository

1. Create a new file `<repo-name>.tf` following the pattern of an existing one
2. Assign a non-overlapping `/24` CIDR from the `172.17.128.0/16` range
3. Choose a `private_runners_prefix` of max 11 lowercase alphanumeric characters

## Permissions

Add Azure AD object IDs (users or groups) to `terraform.tfvars` under `container_apps_managers` to grant Contributor access to the resource group.

## Deployment

Triggered automatically via [altinn-org-gh-runners-deploy.yml](../../.github/workflows/altinn-org-gh-runners-deploy.yml) on changes to this directory or the shared module.

Required GitHub secrets/variables:

| Name | Type | Description |
|---|---|---|
| `GH_RUNNERS_APP_KEY` | Secret | GitHub App private key (base64-encoded PEM) |
| `GH_RUNNERS_APP_ID` | Variable | GitHub App ID |
| `GH_RUNNERS_APP_INSTALL_ID` | Variable | GitHub App installation ID |
