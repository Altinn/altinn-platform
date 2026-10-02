mock_provider "azurerm" {}
mock_provider "github" {}

variables {
  subscription_id = "00000000-0000-0000-0000-000000000001"
}

run "configured_repositories_have_one_identity_each" {
  command = plan

  assert {
    condition = toset(keys(module.syncroot_github_repo)) == toset([
      "altinn-auth", "altinn-dashboards-grafana", "altinn-platform",
      "altinn-platform-dis-proxies", "altinn-studio", "altinn-verification-dis-poc",
      "dialogporten-frontend-manifests", "dialogporten-manifests", "info.altinn.no",
      "pre-invoice-system"
    ])
    error_message = "Each repository must have exactly one publishing identity keyed by its repository name."
  }

  assert {
    condition     = toset(local.products_by_repo["altinn-auth"]) == toset(["access-management", "authorization", "authentication", "register", "resource-registry"])
    error_message = "Auth must have exactly the five requested syncroots, with no legacy accessmanagement entry."
  }
}

run "adding_an_earlier_product_keeps_repository_key" {
  command = plan
  variables {
    product_syncroot_source_repos = {
      abac              = { repo_name = "altinn-auth", environments = [], branches = ["main"] }
      access-management = { repo_name = "altinn-auth", environments = [], branches = ["main"] }
    }
  }
  assert {
    condition     = toset(keys(module.syncroot_github_repo)) == toset(["altinn-auth"])
    error_message = "Adding a product that sorts before existing products must not change the repository's module key."
  }
}

run "repository_identity_name_collisions_are_rejected" {
  command = plan
  variables {
    product_syncroot_source_repos = {
      infoportal = { repo_name = "info.altinn.no", environments = [], branches = ["main"] }
      other      = { repo_name = "info_altinn_no", environments = [], branches = ["main"] }
    }
  }
  expect_failures = [var.product_syncroot_source_repos]
}

run "shared_repository_rejects_different_branches" {
  command = plan
  variables {
    product_syncroot_source_repos = {
      access-management = { repo_name = "altinn-auth", environments = [], branches = ["main"] }
      authorization     = { repo_name = "altinn-auth", environments = [], branches = ["main", "fix/dis"] }
    }
  }
  expect_failures = [var.product_syncroot_source_repos]
}

run "shared_repository_rejects_different_environments" {
  command = plan
  variables {
    product_syncroot_source_repos = {
      access-management = { repo_name = "altinn-auth", environments = [], branches = ["main"] }
      authorization     = { repo_name = "altinn-auth", environments = ["prod"], branches = ["main"] }
    }
  }
  expect_failures = [var.product_syncroot_source_repos]
}

run "removing_original_product_keeps_repository_key" {
  command = plan
  variables {
    product_syncroot_source_repos = {
      resource-registry = { repo_name = "altinn-auth", environments = [], branches = ["main"] }
    }
  }
  assert {
    condition     = toset(keys(module.syncroot_github_repo)) == toset(["altinn-auth"])
    error_message = "The repository's module key must stay the same when only resource-registry remains."
  }
}

run "invalid_product_name_is_rejected" {
  command = plan
  variables {
    product_syncroot_source_repos = {
      "invalid/name" = { repo_name = "altinn-auth", environments = [], branches = ["main"] }
    }
  }
  expect_failures = [var.product_syncroot_source_repos]
}
