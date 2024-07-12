ARM_STORAGE_ACCOUNT=altinnterraformstorage02
ARM_STORAGE_CONTAINER=tfstates
ADMIN_GITHUB_OWNER=altinn
ADMIN_GITHUB_REPOSITORY=altinn-platform
#VARIABLES=variables.tfvars
AZ_SUBSCRIPTION=d43d5057-8389-40d5-88c4-04db9275cbf2

TF=terraform
TF_BACKEND_FLAGS=-backend-config="storage_account_name=$(ARM_STORAGE_ACCOUNT)" -backend-config="container_name=$(ARM_STORAGE_CONTAINER)" -backend-config="key=github.com/$(ADMIN_GITHUB_OWNER)/$(ADMIN_GITHUB_REPOSITORY)/environments/prod/products.tfstate"

tf_migrate:
	$(TF) init -migrate-state $(TF_BACKEND_FLAGS)

tf_init:
	${TF} init $(TF_BACKEND_FLAGS)

az_set_subscription:
	az account set --subscription "$(AZ_SUBSCRIPTION)"

clean_local_state:
	@rm -f *.tfstate *.tfstate.backup
