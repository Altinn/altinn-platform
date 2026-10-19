mock_provider "azurerm" {
  override_during = plan
  mock_resource "azurerm_user_assigned_identity" {
    defaults = {
      id           = "/subscriptions/00000000-0000-0000-0000-000000000001/resourceGroups/syncroots/providers/Microsoft.ManagedIdentity/userAssignedIdentities/Altinn-altinn-auth-syncroot"
      client_id    = "00000000-0000-0000-0000-000000000002"
      principal_id = "00000000-0000-0000-0000-000000000003"
      tenant_id    = "00000000-0000-0000-0000-000000000004"
    }
  }
}
mock_provider "github" {}

variables {
  github_repo_name        = "altinn-auth"
  github_org_id           = "12345"
  github_sub_claim_prefix = "repo:Altinn/altinn-auth"
  github_environments     = []
  github_branches         = ["main"]
  product_names           = ["access-management"]
  subscription_id         = "00000000-0000-0000-0000-000000000001"
  resource_group_name     = "syncroots"
}

run "single_product_permissions_are_unchanged" {
  command = plan
  module {
    source = "../modules/syncroot-github-repo"
  }
  assert {
    condition     = azurerm_role_assignment.altinncr_repo_writer.condition == "((!(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/content/write'}) AND !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/metadata/write'})) OR (@Request[Microsoft.ContainerRegistry/registries/repositories:name] StringStartsWith 'access-management/'))"
    error_message = "Single-product repositories must keep their existing ACR write condition."
  }
  assert {
    condition     = azurerm_user_assigned_identity.syncroot_pusher.name == "Altinn-altinn-auth-syncroot" && azurerm_role_assignment.altinncr_pusher.name == "a72d1840-6d0a-56ef-acf7-5832d4ce4326" && azurerm_role_assignment.altinncr_repo_writer.name == "e068ed94-e773-5133-86c7-5a9375cff232"
    error_message = "Identity and role assignment names must depend on the repository, even for a single product."
  }
}

run "shared_identity_can_publish_all_product_prefixes" {
  command = plan
  module {
    source = "../modules/syncroot-github-repo"
  }
  variables {
    product_names = ["access-management", "authorization", "authentication", "register", "resource-registry"]
  }

  assert {
    condition     = azurerm_user_assigned_identity.syncroot_pusher.name == "Altinn-altinn-auth-syncroot"
    error_message = "The publishing identity must use the repository name."
  }
  assert {
    condition     = github_actions_secret.azure_client_id.secret_name == "DIS_SYNCROOT_AZURE_CLIENT_ID" && github_actions_secret.azure_client_id.plaintext_value == "00000000-0000-0000-0000-000000000002"
    error_message = "The repository client ID secret must point to its publishing identity."
  }
  assert {
    condition     = toset(regexall("StringStartsWith '([^']+)'", azurerm_role_assignment.altinncr_repo_writer.condition)) == toset([["access-management/"], ["authorization/"], ["authentication/"], ["register/"], ["resource-registry/"]])
    error_message = "Write access must include exactly the configured product prefixes, each with a trailing slash."
  }
  assert {
    condition     = length(regexall(" OR ", azurerm_role_assignment.altinncr_repo_writer.condition)) == 5
    error_message = "The product prefixes must be alternatives in the write condition."
  }
  assert {
    condition     = toset(keys(azurerm_federated_identity_credential.syncroot_pusher_branches)) == toset(["main"]) && length(azurerm_federated_identity_credential.syncroot_pusher_branches_immutable) == 0 && length(azurerm_federated_identity_credential.syncroot_pusher_envs) == 0 && length(azurerm_federated_identity_credential.syncroot_pusher_envs_immutable) == 0
    error_message = "Shared publishing must remain restricted to main, with no environment federation."
  }
}

run "name_based_repo_federates_only_on_name_based_subjects" {
  command = plan
  module {
    source = "../modules/syncroot-github-repo"
  }
  variables {
    github_environments = ["at22"]
  }
  assert {
    condition     = azurerm_federated_identity_credential.syncroot_pusher_envs["at22"].subject == "repo:Altinn/altinn-auth:environment:at22" && azurerm_federated_identity_credential.syncroot_pusher_branches["main"].subject == "repo:Altinn/altinn-auth:ref:refs/heads/main"
    error_message = "A repo on name-based subjects must federate on the name-based subject for each environment and branch."
  }
  assert {
    condition     = length(azurerm_federated_identity_credential.syncroot_pusher_envs_immutable) == 0 && length(azurerm_federated_identity_credential.syncroot_pusher_branches_immutable) == 0
    error_message = "A repo on name-based subjects must not get immutable credentials."
  }
}

run "immutable_repo_federates_only_on_immutable_subjects" {
  command = plan
  module {
    source = "../modules/syncroot-github-repo"
  }
  override_data {
    target = data.github_repository.syncroot_source
    values = {
      repo_id = 854445147
    }
  }
  variables {
    github_environments     = ["at22"]
    github_sub_claim_prefix = "repo:Altinn@12345/altinn-auth@854445147"
  }
  assert {
    condition     = azurerm_federated_identity_credential.syncroot_pusher_envs_immutable["at22"].subject == "repo:Altinn@12345/altinn-auth@854445147:environment:at22" && azurerm_federated_identity_credential.syncroot_pusher_branches_immutable["main"].subject == "repo:Altinn@12345/altinn-auth@854445147:ref:refs/heads/main"
    error_message = "A repo on immutable subjects must federate on the immutable subject for each environment and branch."
  }
  assert {
    condition     = azurerm_federated_identity_credential.syncroot_pusher_envs_immutable["at22"].name == "Altinn-altinn-auth-env-at22-immutable" && azurerm_federated_identity_credential.syncroot_pusher_branches_immutable["main"].name == "Altinn-altinn-auth-ref-main-immutable"
    error_message = "Immutable credentials must keep their existing names so repos already on immutable subjects keep them."
  }
  assert {
    condition     = length(azurerm_federated_identity_credential.syncroot_pusher_envs) == 0 && length(azurerm_federated_identity_credential.syncroot_pusher_branches) == 0
    error_message = "A repo on immutable subjects must not get name-based credentials."
  }
}

run "unknown_subject_prefix_is_rejected" {
  command = plan
  module {
    source = "../modules/syncroot-github-repo"
  }
  variables {
    github_sub_claim_prefix = "repo:Altinn/altinn-auth:context"
  }
  expect_failures = [azurerm_user_assigned_identity.syncroot_pusher]
}

run "changing_products_keeps_identity_and_role_names" {
  command = plan
  module {
    source = "../modules/syncroot-github-repo"
  }
  variables {
    product_names = ["abac", "resource-registry"]
  }
  assert {
    condition     = azurerm_user_assigned_identity.syncroot_pusher.name == "Altinn-altinn-auth-syncroot" && azurerm_role_assignment.altinncr_pusher.name == "a72d1840-6d0a-56ef-acf7-5832d4ce4326" && azurerm_role_assignment.altinncr_repo_writer.name == "e068ed94-e773-5133-86c7-5a9375cff232"
    error_message = "Adding an earlier product and removing the original product must not change identity or role assignment names."
  }
  assert {
    condition     = toset(regexall("StringStartsWith '([^']+)'", azurerm_role_assignment.altinncr_repo_writer.condition)) == toset([["abac/"], ["resource-registry/"]])
    error_message = "Write access must follow the current products without retaining a removed product's prefix."
  }
}

run "invalid_product_name_is_rejected" {
  command = plan
  module {
    source = "../modules/syncroot-github-repo"
  }
  variables {
    product_names = ["resource/registry"]
  }
  expect_failures = [var.product_names]
}

run "empty_products_are_rejected" {
  command = plan
  module {
    source = "../modules/syncroot-github-repo"
  }
  variables {
    product_names = []
  }
  expect_failures = [var.product_names]
}
