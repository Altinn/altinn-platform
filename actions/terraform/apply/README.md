# Introduction

This GitHub Action downloads the Terraform plan file that the `altinn/altinn-platform/actions/terraform/plan` action stored beside the Terraform state. It verifies the plan file against the checksum from the plan job and executes `terraform apply` with it. This action fails unless the plan action has been executed first in the same workflow run.

After a successful apply the action deletes the plan file. If the delete fails, the job prints a warning. To apply again, re-run all jobs so that the plan job makes a new plan file. If the apply fails, the plan file stays and you can re-run the apply job.

If the action finds no checksum artifact, the plan job stored no plan file. The plan job ran on a `pull_request` event, or it uses another version of the actions.

See [Plan file](../plan/README.md#plan-file) for where the plan file is and what the actions need.

## Sample
```yaml
jobs:
  plan:
    name: Plan
    environment: prod
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Repository
        uses: actions/checkout@v4

      - name: Terraform Plan
        uses: altinn/altinn-platform/actions/terraform/plan@main
        with:
          working_directory: ${{ env.TF_PROJECT }}
          oidc_type: environment
          oidc_value: ${{ env.ENVIRONMENT }}
          arm_client_id: ${{ env.ARM_CLIENT_ID }}
          arm_subscription_id: ${{ env.ARM_SUBSCRIPTION_ID }}
          tf_state_name: ${{ env.TF_STATE_NAME }}
          gh_token: ${{ secrets.GITHUB_TOKEN }}

  deploy:
    name: Deploy
    environment: prod
    needs: plan
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Repository
        uses: actions/checkout@v4

      - name: Terraform Apply
        uses: altinn/altinn-platform/actions/terraform/apply@main
        with:
          working_directory: ${{ env.TF_PROJECT }}
          oidc_type: environment
          oidc_value: ${{ env.ENVIRONMENT }}
          arm_client_id: ${{ env.ARM_CLIENT_ID }}
          arm_subscription_id: ${{ env.ARM_SUBSCRIPTION_ID }}
          tf_state_name: ${{ env.TF_STATE_NAME }}

```