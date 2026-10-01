mock_provider "azurerm" {}
mock_provider "github" {}

variables {
  subscription_id = "00000000-0000-0000-0000-000000000001"
}

run "configured_repositories_have_one_identity_each" {
  command = plan

  assert {
    condition = toset(keys(module.syncroot_github_repo)) == toset([
      "access-management", "arbeidsflate", "core", "dialogporten", "dis",
      "disproxies", "infoportal", "monitoring", "preinvoicingsystem", "studio"
    ])
    error_message = "Each repository must have one publishing identity, with the Auth module renamed to access-management."
  }

  assert {
    condition = toset([
      for product, repo in var.product_syncroot_source_repos : product if repo.repo_name == "altinn-auth"
    ]) == toset(["access-management", "authorization", "authentication", "register", "resource-registry"])
    error_message = "Auth must have exactly the five requested syncroots, with no legacy accessmanagement entry."
  }
}

run "shared_repository_requires_identity_selection" {
  command = plan
  variables {
    repo_identity_products = {}
  }
  expect_failures = [var.product_syncroot_source_repos]
}

run "identity_product_must_belong_to_repository" {
  command = plan
  variables {
    repo_identity_products = { "altinn-auth" = "studio" }
  }
  expect_failures = [var.product_syncroot_source_repos]
}

run "identity_product_must_exist" {
  command = plan
  variables {
    repo_identity_products = { "altinn-auth" = "missing" }
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

run "single_product_does_not_need_identity_selection" {
  command = plan
  variables {
    repo_identity_products = {}
    product_syncroot_source_repos = {
      resource-registry = { repo_name = "altinn-auth", environments = [], branches = ["main"] }
    }
  }
  assert {
    condition     = toset(keys(module.syncroot_github_repo)) == toset(["resource-registry"])
    error_message = "A single product with a hyphenated name must work without an explicit identity selection."
  }
}

run "invalid_product_name_is_rejected" {
  command = plan
  variables {
    repo_identity_products = {}
    product_syncroot_source_repos = {
      "invalid/name" = { repo_name = "altinn-auth", environments = [], branches = ["main"] }
    }
  }
  expect_failures = [var.product_syncroot_source_repos]
}
