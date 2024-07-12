# Altinn Platform

## Altinn Products
To configure or add identity federation (GitHub to Azure), Azure IAM, and handle Terraform state for a product in Altinn, modify the `products.yaml` file and create a pull request.

### Identity Federation
For identity federation to work in GitHub actions, federation must either be done in context of branch `main` or from the GitHub environments listed in the table below. You can introduce new environments by making some changes to `terraform.tfvars` in `infrastructure/products`.

| GitHub Environment | Azure Subscription |
| - | - |
| dev | Dev |
| test | Test |
| at21 | Test |
| at22 | Test |
| at23 | Test |
| at24 | Test |
| at25 | Test |
| yt01 | Test |
| staging | Staging |
| tt02 | Staging |
| prod | Prod |

### IAM
Products in altinn will have be configured as following:

| Group Name | Permissions |
| - | - |
| Altinn Product {Team Name} Reader : {Workspace} | Azure IAM Reader |
| Altinn Product {Team Name} Developer : {Workspace} | Azure IAM Contributor |
| Altinn Product {Team Name} Admin : {Workspace} | Azure IAM User Access Administrator, Contributor, Storage Blob Owner (Terraform State files) |

The owner of these groups will be set manually by platform, typically the owner will be the architect of the product.

### Terraform
Each product have access to a storage account for persisting the state of their Terraform projects and identity federation must be used to access the Terraform state files. Federated Applications can read other all state files but are able to write to their own. The storage account container is configured with Azure ABAC rules to control write permissions for the different applications. The state file path are: `github.com/<github owner>/<repo>/<environments | branch>/<branch name | environment name>/<state file name>`. Only members of the administrator group can unlock dead leases in the container and migrate Terraform state files.

Additionally, Terraform templates for CI and CD builds are available in `templates/terraform`:

`Plan`: Used for CI builds. It runs terraform validate, fmt, init, and plan, generates a report, and publishes it in the job summary or as a comment on a pull request (if any). The plan is also published as an artifact that can be reviewed before applying it.

`Apply`: Initializes the Terraform state, attempts to download an artifact that matches the given parameters. If no artifact is found, it will plan and apply itself. For the best results, we recommend running the plan before apply so that a maintainer can review the plan first.