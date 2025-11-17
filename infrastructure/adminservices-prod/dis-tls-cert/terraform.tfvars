# Azure region where resources will be deployed
# Example values: norwayeast, westeurope, northeurope
location = "norwayeast"

# Azure subscription ID where resources will be deployed
# Replace with your actual subscription ID in GUID format
subscription_id = "a6e9ee7d-2b65-41e1-adfb-0c8c23515cf9"

# Note: Tags are now defined in localtags.tf and merged with submodule tag
# The tags variable is currently not used in the module

# Additional RBAC role assignments for the Key Vault
# Each entry requires a valid Key Vault RBAC role and principal ID
# Uncomment and modify as needed:
# azure_keyvault_additional_role_assignments = [
#   {
#     role_definition_name = "Key Vault Secrets Officer"
#     principal_id         = "00000000-0000-0000-0000-000000000000"
#   },
#   {
#     role_definition_name = "Key Vault Certificates Officer"
#     principal_id         = "11111111-1111-1111-1111-111111111111"
#   }
# ]
