# Introduction
There are three containg terraform operations: *init*, *plan*, and *apply*. Each of these can seamlessly be integrated with any Altinn repository if they are enrolled in the identity federation setup. As identity federation is configured for branch (just main) and environments *dev*, *at21*, *at22*, *at23*, *at24*, *at25*, *tt02*, and *prod*, any deployments to other environments or branches will not work. The Terraform state will be stored in a centralized storage account. The path to the state will as following: `github.com/altinn/<repository name><branch | environment>/<branch name | environment name>/<state name>`.

Terraform init templates initialize the state. This job must be called before either plan or apply. It has the following arguments and is set using the keyword `with` when using the template:
```yaml
  working_directory: Project containing the main.tf file

  ### Identity federation parameters
  oidc_type: should either be environment or branch. Environment if using GitHub environment, branch main if not using GitHub environments. Required.
  oidc_value: should either be branch or name of the environment. Required.

  # Azure Parameters
  arm_client_id: Should be ${{ vars.ARM_CLIENT_ID }}. Required
  arm_subscription_id: Should be ${{ vars.ARM_SUBSCRIPTION_ID }}. Required
  arm_tenant_id: Should be ${{ vars.ARM_TENANT_ID }}. Can be ignored
  arm_resource_group_name: Resource group name of the storage account. Can be ignored 
  arm_storage_account_name: Name of storage account. Can be ignored

  ## Terraform Parameters
  tf_state_name: Defaults to tfstate. Must be set to a unique name if having more Terraform projects in a repository
  tf_version: Defaults to 1.7.4
  tf_log_level: Defaults to INFO
``` 

Terraform plan and apply has following parameters. The plan template will post a comment on the github aciton job.
```yaml
  working_directory: Project containing the main.tf file

  # Azure Parameters
  arm_client_id: Should be ${{ vars.ARM_CLIENT_ID }}. Required.
  arm_subscription_id: Should be ${{ vars.ARM_SUBSCRIPTION_ID }}. Required.
  arm_tenant_id: Should be ${{ vars.ARM_TENANT_ID }}. Required.

  # Terraform parameters
  tf_args: command flags <plan / apply> $tf_args 
  tf_version: Defaults to 1.7.4
```

## Templates

```yaml
name: Deploy Template

on:
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      log_level:
        required: true
        description: "Terraform Log Level"
        default: INFO
        type: choice
        options:
          - TRACE
          - DEBUG
          - INFO
          - WARN
          - ERROR

# Must be present for token exchange
permissions:
  id-token: write
  contents: write

jobs:
  at:
    name: AT
    uses: "./.github/workflows/terraform-apply-template.yaml"
    secrets: inherit
    strategy:
      fail-fast: false
      matrix:
        environment: [at21, at22, at23, at24, at25]
    with:
      environment: ${{ matrix.environment }}
      log_level: ${{ inputs.log_level }}

  tt02:
    uses: "./.github/workflows/terraform-apply-template.yaml"
    name: tt02
    needs: at
    secrets: inherit
    with:
      environment: tt02
      log_level: ${{ inputs.log_level }}

  prod_plan:
    uses: "./.github/workflows/terraform-plan-template.yaml"
    name: prod_plan
    needs: tt02
    secrets: inherit
    with:
      environment: prod
      log_level: ${{ inputs.log_level }}

  prod:
    uses: "./.github/workflows/terraform-apply-template.yaml"
    name: prod
    needs: prod_plan
    secrets: inheriP
    with:
      environment: prod
      log_level: ${{ inputs.log_level }}
```

## Terraform Apply Template

```yaml
# File: "./.github/workflows/apply-template.yaml"
on:
  workflow_call:
    inputs:
      environment:
        required: true
        type: string
      log_level:
        required: false
        type: string

env:
  TERRAFORM_VERSION: 1.7.4

jobs:
  release:
    environment: ${{ inputs.environment }}
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Repository
        uses: actions/checkout@v4
      - name: Terraform Initialize
        uses: altinn/altinn-platform/actions/terraform/init@v1
        with:
          working_directory: ./infrastructure # Path to directory containg main.tf
          oidc_type: environment # Token exhchange type branch or github environment. Recommended to use environment
          oidc_value: ${{ inputs.environment }} # Name of the github environment or branch. 
          arm_client_id: ${{ vars.ARM_CLIENT_ID }} # Azure app regg client ID
          arm_subscription_id: ${{ vars.ARM__SUBSCRIPTION_ID }}
          tf_state_name: tfstate # If you have multiple terraform projects per repo, make sure this has a unique name. The name is scoped to the repository 

      - name: Terraform Apply
        uses: altinn/altinn-platform/actions/terraform/apply@v1
        with:
          working_directory: ./infrastructure
          arm_client_id: ${{ vars.ARM_CLIENT_ID }}
          arm_subscription_id: ${{ vars.ARM_SUBSCRIPTION_ID }}
          tf_args: -var-file="variables.${{ inputs.environment }}.tfvars"
```

## Terraform Plan Template

```yaml
# File: "./.github/workflows/apply-template.yaml"
on:
  workflow_call:
    inputs:
      environment:
        required: true
        type: string
      log_level:
        required: false
        type: string

env:
  TERRAFORM_VERSION: 1.7.4

jobs:
  release:
    environment: ${{ inputs.environment }}
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Repository
        uses: actions/checkout@v4

      - name: Terraform Initialize
        uses: altinn/altinn-platform/actions/terraform/init@v1
        with:
          working_directory: ./infrastructure # Path to directory containg main.tf
          oidc_type: environment # Token exhchange type branch or github environment. Recommended to use environment
          oidc_value: ${{ inputs.environment }} # Name of the github environment or branch. 
          arm_client_id: ${{ vars.ARM_CLIENT_ID }} # Azure app regg client ID
          arm_subscription_id: ${{ vars.ARM__SUBSCRIPTION_ID }}
          tf_state_name: tfstate # If you have multiple terraform projects per repo, make sure this has a unique name. The name is scoped to the repository 

      - name: Terraform Plan
        uses: altinn/altinn-platform/actions/terraform/plan@v1
        with:
          working_directory: ./infrastructure
          arm_client_id: ${{ vars.ARM_CLIENT_ID }}
          arm_subscription_id: ${{ vars.ARM_SUBSCRIPTION_ID }}
          tf_args: -var-file="variables.${{ inputs.environment }}.tfvars"
```
