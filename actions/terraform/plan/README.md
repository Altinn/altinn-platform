# Description

This GitHub Action executes `terraform plan` against a specified environment and stores the plan file beside the Terraform state. In addition, this action runs `terraform fmt` and `terraform validate`, and includes the results of all three (`fmt`, `validate`, and `plan`) in the GitHub Action Summary. If this action is triggered by a pull request and the plan has changes, it will also post a comment with the summary.

## Plan file

- The action stores the plan file in container `tfstates` of the state storage account. The blob path is `<state path>.plans/<run id>.tfplan`.
- The action publishes only the SHA-256 checksum of the plan file as a GitHub artifact. The apply action uses it to verify the plan file.
- On `pull_request` events the action stores no plan file, because no job applies those plans. Every other event stores one. Set `store_plan_on_pull_request: "true"` if a job applies the plan in the same pull request run.
- The apply action deletes the plan file after a successful apply. If the delete fails, the job prints a warning, and the plan file or some of its versions stay. A plan file that no job applies also stays in the storage account.
- The actions use their own Azure CLI session. They do not change the Azure CLI session of the caller.

Requirements:

- The runner has the Azure CLI (`az`).
- The identity has the Storage Blob Data Owner role for the state path. The apply action deletes blob versions.
- The plan job and the apply job run in the same workflow run.
- Both jobs use the same version of these actions and the same `oidc_type`, `oidc_value` and `tf_state_name`.

## Log level

The `init` action sets the Terraform log level for the job. The plan and apply actions run `init` first.

- `tf_log_level` accepts `INFO`, `WARN` and `ERROR`, in upper or lower case. The default is `WARN`.
- At `WARN`, Terraform prints no `[INFO]` log lines, for example the lines with the Terraform version and the command arguments. The plan output, the job summary and the pull request comment do not change.
- At `INFO`, the log shows the command arguments, and these include `tf_args`. Do not put secret values in `tf_args`.
- The action refuses every other value, for example `DEBUG`, `TRACE` and `JSON`. The job log can be public, and these levels can print secret values.
- The action also refuses such a value in the environment variables `TF_LOG`, `TF_LOG_CORE`, `TF_LOG_PROVIDER`, `TF_LOG_CLOUD` and `TF_LOG_STACKS`, because Terraform reads them too.
- When the action refuses a value, the step fails with an error, and `init`, `plan` and `apply` run no Terraform command. The action also sets the refused variables to a safe level for the rest of the job.

Limits:

- The check stops mistakes. It does not stop a workflow that sets these variables after `init`, on its own steps, or in `env` on a step that calls `plan-only` or `apply-only`.
- To get debug logs, run Terraform in a place where the log is not public.

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