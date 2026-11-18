# Description

This GitHub Action is similar to `altinn/altinn-platform/actions/terraform/plan` but requires `altinn/altinn-platform/actions/terraform/init` to be run first. This allows you, for example, to run `terraform output` commands you might need before running the Plan.

The action stores the plan file in the same way as the plan action. See [Plan file](../plan/README.md#plan-file). If `init` uses another storage account than the default, give the same `tf_arm_storage_account_name` to this action.


## Sample
```yaml
jobs:
  plan:
    name: Plan
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Repository
        uses: actions/checkout@v4
      ...
      - name: Terraform Init
        uses: Altinn/altinn-platform/actions/terraform/init@main
        with:
          ...
      - name: Get something from Terraform output
        shell: bash
        run: |
          echo "SOMETHING=$(terraform output -raw something)" >> "$GITHUB_OUTPUT"
      ...
      - name: Terraform Plan
        uses: Altinn/altinn-platform/actions/terraform/plan-only@main
        with:
          ...
```