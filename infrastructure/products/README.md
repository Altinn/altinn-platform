## Introduction
![Solution](products/docs/architecture.drawio.svg)

## Initialization

comment out `backend "azurerm"` block in `main.tf` for initiating state locally. 
```terraform
terraform {
    ...

    # backend "azurerm" {
    #   use_azuread_auth = true
    # }
}
```

### Prerequisites:

* *az* CLI tool installed.
* Ensure that you has either Azure IAM role(s) current subscription:
    * *Owner*
    * Combination of *Contributor* and *User Access Administrator*.
* Ensure that you has permissions to create AD groups and app registrations.

### Configure Variables
* Make any necessary changes to *variables.tfvars* before deploying the project. Default configuration for *variables.tfvars* are:
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
| `ARM_STORAGE_ACCOUNT`     	| Name of the storage account. Can be interpreted from Terraform file *azure_arm.tf* in block `resource "azurerm_storage_account" "backend"`                                            	|
| `ARM_STORAGE_CONTAINER`   	| By default it should be *tfstates*, but it must have the same name as defined in block `resource "azurerm_storage_container" "container"`, can also be located in file *azure_arm* 	    |
| `ADMIN_GITHUB_OWNER`      	| GitHub organization or user. Should be the same name as defined in the *organization.yaml* field `admin.github.owner`                                                                 	|
| `ADMIN_GITHUB_REPOSITORY` 	| GitHub repository that should manage this project. Repository should be defined in *organization.yaml* field `admin.github.repository`                                                	|
| `VARIABLES`               	| Path to the *tf.vars* default *variables.tfvars*                                                                                                                                    	    |

Uncomment the `backend "azurerm"` block in *main.tf* to enable remote state storage for deployments:
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

