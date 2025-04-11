# Description

This GitHub Action is similar to `altinn/altinn-platform/actions/terraform/apply` but requires `altinn/altinn-platform/actions/terraform/init` to be run first. This allows you, for example, to run `terraform output` commands you might need before running the Plan.

Note: Similar to the other action, this action will not run unless the `altinn/altinn-platform/actions/terraform/plan-only` action has been executed first.


## Sample
```yaml
jobs:
  plan:
    ...
  deploy:
    name: Deploy
    needs: plan
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
      - name: Use the value from Terraform output
        shell: bash
        env:
          SOMETHING: ${{ steps.<...>.SOMETHING }}
        run: |
          echo $SOMETHING
      ...
      - name: Terraform Apply
        uses: Altinn/altinn-platform/actions/terraform/apply-only@main
        with:
          ...
```
