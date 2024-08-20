# Description

This GitHub Action executes `terraform plan` against a specified environment and publishes the plan as a GitHub artifact. In addition, this action runs `terraform fmt` and `terraform validate`, and includes the results of all three (`fmt`, `validate`, and `plan`) in the GitHub Action Summary. If this action is triggered by a pull request, it will also post a comment with the summary.


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

```