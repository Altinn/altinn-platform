ARM_STORAGE_ACCOUNT=altinnterraformstorage02
ARM_STORAGE_CONTAINER=tfstates
ADMIN_GITHUB_OWNER=altinn
ADMIN_GITHUB_REPOSITORY=altinn-platform
VARIABLES=variables.tfvars
AZ_SUBSCRIPTION=45177a0a-d27e-490f-9f23-b4726de8ccc1

TF=terraform
TF_BACKEND_FLAGS=-backend-config="storage_account_name=$(ARM_STORAGE_ACCOUNT)" -backend-config="container_name=$(ARM_STORAGE_CONTAINER)" -backend-config="key=github.com/$(ADMIN_GITHUB_OWNER)/$(ADMIN_GITHUB_REPOSITORY)/environments/prod/teams.tfstate"
TF_VARIABLES_FLAGS=-var-file=$(VARIABLES)

tf_migrate:
	$(TF) init -migrate-state $(TF_BACKEND_FLAGS)

tf_init:
	${TF} init $(TF_BACKEND_FLAGS)

tf_plan:
	$(TF) plan $(TF_VARIABLES_FLAGS)

tf_apply:
	$(TF) apply $(TF_VARIABLES_FLAGS)

tf_destroy:
	$(TF) destroy $(TF_BACKEND_FLAGS)

az_set_subscription:
	az account set --subscription "$(AZ_SUBSCRIPTION)"

clean_local_state:
	@rm -f *.tfstate *.tfstate.backup *.terraform
