APP_NAME ?= dis-identity-operator
IMG ?= controller:latest
CONTAINER_TOOL ?= docker
include ../../Makefile.common

##@ Identity-specific

ASO_CRD_VERSION ?= v2.13.0

# The envtest controller suite needs the Azure Service Operator CRDs on disk.
# This recipe-less rule augments the setup-envtest target from Makefile.common
# with an extra prerequisite (it does not override the recipe).
setup-envtest: aso-crds

.PHONY: aso-crds
aso-crds: ## Download the ASO CRDs locally if necessary.
	@mkdir -p $(LOCALBIN)/aso-crds
	curl -sSL https://github.com/Azure/azure-service-operator/releases/download/$(ASO_CRD_VERSION)/azureserviceoperator_customresourcedefinitions_$(ASO_CRD_VERSION).yaml -o $(LOCALBIN)/aso-crds/azureserviceoperator_customresourcedefinitions_$(ASO_CRD_VERSION).yaml
