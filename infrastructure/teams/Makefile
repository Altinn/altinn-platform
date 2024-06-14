ARM_STORAGE_ACCOUNT=staltinntfstate001
ARM_STORAGE_CONTAINER=tfstates
ADMIN_GITHUB_OWNER=altinn
ADMIN_GITHUB_REPOSITORY=altinn-platform
VARIABLES=variables.tfvars

TF=terraform
TF_BACKEND_FLAGS=-backend-config="storage_account_name=$(ARM_STORAGE_ACCOUNT)" -backend-config="container_name=$(ARM_STORAGE_CONTAINER)" -backend-config="key=github.com/$(ADMIN_GITHUB_OWNER)/$(ADMIN_GITHUB_REPOSITORY)"
TF_VARIABLES_FLAGS=-var-file=$(VARIABLES)

tf_migrate:
	$(TF) init -migrate-state $(TF_BACKEND_FLAGS)

tf_plan:
	$(TF) plan $(TF_VARIABLES_FLAGS)

tf_apply:
	$(TF) apply $(TF_VARIABLES_FLAGS)