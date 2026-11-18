# Description

This GitHub Action writes the results of `terraform fmt`, `terraform validate` and `terraform plan` to the GitHub Action Summary. On a pull request it also posts the summary as a comment, but only when the plan has changes.

The `Plan File` row shows where the plan action stored the plan file, or that it did not store one on a pull request run. See [Plan file](../plan/README.md#plan-file).
