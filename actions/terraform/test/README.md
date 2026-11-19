# Test project

The workflow `.github/workflows/terraform-actions-test.yml` uses this Terraform project to test the plan and apply actions in a real run. The project has one `terraform_data` resource. It creates nothing in Azure except its own state.

`plan-file-delete-test.sh` tests the delete of the plan file with a stub of the Azure CLI. The same workflow runs it. It makes no call to Azure.
