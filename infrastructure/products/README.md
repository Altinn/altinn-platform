
## Altinn Products
To configure or add identity federation (GitHub to Azure), Azure IAM, and handle Terraform state for a product in Altinn, modify the products.yaml file located at the repo's root and create a pull request.

## Solution

![products/docs/architecture.drawio.svg](https://raw.githubusercontent.com/Altinn/altinn-platform/main/infrastructure/products/docs/architecture.drawio.svg)

### Identity Federation
For identity federation to work in GitHub Actions, federation must either be done in the context of the branch `main` or from the GitHub environments listed in the table below. You can introduce new environments by making some changes to `terraform.tfvars` in `infrastructure/products`.

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
Products in Altinn should be configured as follows:

| Group Name | Permissions |
| - | - |
| Altinn Product {Team Name} Reader : {Workspace} | Azure IAM Reader |
| Altinn Product {Team Name} Developer : {Workspace} | Azure IAM Contributor |
| Altinn Product {Team Name} Admin : {Workspace} | Azure IAM User Access Administrator, Contributor, Storage Blob Owner (Terraform State files) |

The owner of these groups will be set manually by the platform; typically, the owner will be the architect of the product.

### Terraform
Each product has access to a storage account for persisting the state of their Terraform projects, and identity federation must be used to access the Terraform state files. Federated applications can read all state files but can write only to their own. The storage account container is configured with Azure ABAC rules to control write permissions for different applications. The state file paths are: github.com/<github owner>/<repo>/<environments | branch>/<branch name | environment name>/<state file name>. Only members of the administrator group can unlock dead leases in the container and migrate Terraform state files.

Additionally, Terraform templates for CI and CD builds are available in templates/terraform:

Plan: Used for CI builds. It runs terraform validate, fmt, init, and plan, generates a report, and publishes it in the job summary or as a comment on a pull request (if any). The plan is also published as an artifact that can be reviewed before applying it.

Apply: Initializes the Terraform state, and attempts to download an artifact that matches the given parameters. If no artifact is found, it will plan and apply itself. For best results, we recommend running the plan before applying, so that a maintainer can review the plan first.

## Initialization

Comment out the backend "azurerm" block in main.tf for initiating state locally:

```terraform
terraform {
    ...

    # backend "azurerm" {
    #   use_azuread_auth = true
    # }
}
```

### Prerequisites:
* The az CLI tool must be installed.
* Ensure that you have one of the following Azure IAM roles in the current subscription:
  * Owner
  * A combination of Contributor and User Access Administrator.
* Ensure that you have permissions to create AD groups and app registrations.

### Configure Variables
* Make any necessary changes to variables.tfvars before deploying the project. The default configuration for variables.tfvars is:

```terraform
environments = [
  {
    name = "dev"
    workspaces = [
      {
        arm_subscription = "dev"
        names            = ["dev"]
      },
      {
        arm_subscription = "test"
        names            = ["at21", "at22", "at23", "at24", "at25"]
      }
    ]
  },
  {
    name = "prod"
    workspaces = [
      {
        arm_subscription = "staging"
        names            = ["tt02"]
      },
      {
        arm_subscription = "prod"
        names            = ["prod"]
      }
    ]
  }
]
```

### State Migration
In *Makefile* make adjustments to variables, if any:

| Variables                 	| Description                                                                                                                                                                               |
|---------------------------	|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------  |
| `ARM_STORAGE_ACCOUNT`     	| Name of the storage account. Can be interpreted from the Terraform file azure_arm.tf in the block resource "azurerm_storage_account" "backend"`                                            	|
| `ARM_STORAGE_CONTAINER`   	| By default, it should be tfstates, but it must have the same name as defined in the block resource "azurerm_storage_container" "container". This can also be located in the file azure_arm |
| `ADMIN_GITHUB_OWNER`      	| GitHub organization or user. Should be the same name as defined in the organization.yaml field admin.github.owner |
| `ADMIN_GITHUB_REPOSITORY` 	| GitHub repository that should manage this project. The repository should be defined in the organization.yaml field admin.github.repository |
| `VARIABLES`               	| Path to the tf.vars file; default is variables.tfvars |

Uncomment the `backend "azurerm"` block in `main.tf` to enable remote state storage for deployments:

```terraform
terraform {
    ...

    backend "azurerm" {
      use_azuread_auth = true
    }
}
```

```bash
# Migrate states to the cloud
make tf_migrate

# For terraform apply locally, can be executed after state has been migrated
make tf_apply

# For Terraform plan locally, can be executed after state has been migrated
```
