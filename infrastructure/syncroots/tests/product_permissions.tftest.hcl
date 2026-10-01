mock_provider "azurerm" {
  override_during = plan
  mock_resource "azurerm_user_assigned_identity" {
    defaults = {
      id           = "/subscriptions/00000000-0000-0000-0000-000000000001/resourceGroups/syncroots/providers/Microsoft.ManagedIdentity/userAssignedIdentities/Altinn-altinn-auth-accessmanagement-syncroot"
      client_id    = "00000000-0000-0000-0000-000000000002"
      principal_id = "00000000-0000-0000-0000-000000000003"
      tenant_id    = "00000000-0000-0000-0000-000000000004"
    }
  }
}
mock_provider "github" {}

variables {
  github_repo_name    = "altinn-auth"
  github_org_id       = "12345"
  github_environments = []
  github_branches     = ["main"]
  product_name        = "accessmanagement"
  subscription_id     = "00000000-0000-0000-0000-000000000001"
  resource_group_name = "syncroots"
}

run "single_product_permissions_are_unchanged" {
  command = plan
  module {
    source = "../modules/syncroot-github-repo"
  }
  assert {
    condition     = azurerm_role_assignment.altinncr_repo_writer.condition == "((!(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/content/write'}) AND !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/metadata/write'})) OR (@Request[Microsoft.ContainerRegistry/registries/repositories:name] StringStartsWith 'accessmanagement/'))"
    error_message = "Single-product repositories must keep their existing ACR write condition."
  }
}

run "shared_identity_can_publish_all_product_prefixes" {
  command = plan
  module {
    source = "../modules/syncroot-github-repo"
  }
  variables {
    additional_product_names = ["access-management", "authorization", "authentication", "register", "resource-registry"]
  }

  assert {
    condition     = azurerm_user_assigned_identity.syncroot_pusher.name == "Altinn-altinn-auth-accessmanagement-syncroot"
    error_message = "The existing publishing identity must be retained."
  }
  assert {
    condition     = github_actions_secret.azure_client_id.secret_name == "DIS_SYNCROOT_AZURE_CLIENT_ID" && github_actions_secret.azure_client_id.plaintext_value == "00000000-0000-0000-0000-000000000002"
    error_message = "The repository client ID secret must still point to the existing identity."
  }
  assert {
    condition     = toset(regexall("StringStartsWith '([^']+)'", azurerm_role_assignment.altinncr_repo_writer.condition)) == toset([["accessmanagement/"], ["access-management/"], ["authorization/"], ["authentication/"], ["register/"], ["resource-registry/"]])
    error_message = "Write access must include exactly the configured product prefixes, each with a trailing slash."
  }
  assert {
    condition     = length(regexall(" OR ", azurerm_role_assignment.altinncr_repo_writer.condition)) == 6
    error_message = "The product prefixes must be alternatives in the write condition."
  }
  assert {
    condition     = toset(keys(azurerm_federated_identity_credential.syncroot_pusher_branches)) == toset(["main"]) && toset(keys(azurerm_federated_identity_credential.syncroot_pusher_branches_immutable)) == toset(["main"]) && length(azurerm_federated_identity_credential.syncroot_pusher_envs) == 0 && length(azurerm_federated_identity_credential.syncroot_pusher_envs_immutable) == 0
    error_message = "Shared publishing must remain restricted to main, with no environment federation."
  }
}

run "invalid_additional_product_name_is_rejected" {
  command = plan
  module {
    source = "../modules/syncroot-github-repo"
  }
  variables {
    additional_product_names = ["resource/registry"]
  }
  expect_failures = [var.additional_product_names]
}
