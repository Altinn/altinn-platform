mock_provider "azurerm" {}
mock_provider "github" {}

# GitHub reports each repo's OIDC subject format through data.github_rest_api. Mock every
# repo these tests plan on the name-based form; the module tests cover both forms.
override_data {
  target = data.github_rest_api.oidc_subject["altinn-auth"]
  values = {
    code = 200
    body = "{\"use_default\":true,\"use_immutable_subject\":false,\"sub_claim_prefix\":\"repo:Altinn/altinn-auth\"}"
  }
}

override_data {
  target = data.github_rest_api.oidc_subject["altinn-dashboards-grafana"]
  values = {
    code = 200
    body = "{\"use_default\":true,\"use_immutable_subject\":false,\"sub_claim_prefix\":\"repo:Altinn/altinn-dashboards-grafana\"}"
  }
}

override_data {
  target = data.github_rest_api.oidc_subject["altinn-platform"]
  values = {
    code = 200
    body = "{\"use_default\":true,\"use_immutable_subject\":false,\"sub_claim_prefix\":\"repo:Altinn/altinn-platform\"}"
  }
}

override_data {
  target = data.github_rest_api.oidc_subject["altinn-platform-dis-proxies"]
  values = {
    code = 200
    body = "{\"use_default\":true,\"use_immutable_subject\":false,\"sub_claim_prefix\":\"repo:Altinn/altinn-platform-dis-proxies\"}"
  }
}

override_data {
  target = data.github_rest_api.oidc_subject["altinn-profile"]
  values = {
    code = 200
    body = "{\"use_default\":true,\"use_immutable_subject\":false,\"sub_claim_prefix\":\"repo:Altinn/altinn-profile\"}"
  }
}

override_data {
  target = data.github_rest_api.oidc_subject["altinn-studio"]
  values = {
    code = 200
    body = "{\"use_default\":true,\"use_immutable_subject\":false,\"sub_claim_prefix\":\"repo:Altinn/altinn-studio\"}"
  }
}

override_data {
  target = data.github_rest_api.oidc_subject["altinn-verification-dis-poc"]
  values = {
    code = 200
    body = "{\"use_default\":true,\"use_immutable_subject\":false,\"sub_claim_prefix\":\"repo:Altinn/altinn-verification-dis-poc\"}"
  }
}

override_data {
  target = data.github_rest_api.oidc_subject["dialogporten-frontend-manifests"]
  values = {
    code = 200
    body = "{\"use_default\":true,\"use_immutable_subject\":false,\"sub_claim_prefix\":\"repo:Altinn/dialogporten-frontend-manifests\"}"
  }
}

override_data {
  target = data.github_rest_api.oidc_subject["dialogporten-manifests"]
  values = {
    code = 200
    body = "{\"use_default\":true,\"use_immutable_subject\":false,\"sub_claim_prefix\":\"repo:Altinn/dialogporten-manifests\"}"
  }
}

override_data {
  target = data.github_rest_api.oidc_subject["dis-padawan"]
  values = {
    code = 200
    body = "{\"use_default\":true,\"use_immutable_subject\":false,\"sub_claim_prefix\":\"repo:Altinn/dis-padawan\"}"
  }
}

override_data {
  target = data.github_rest_api.oidc_subject["info.altinn.no"]
  values = {
    code = 200
    body = "{\"use_default\":true,\"use_immutable_subject\":false,\"sub_claim_prefix\":\"repo:Altinn/info.altinn.no\"}"
  }
}

override_data {
  target = data.github_rest_api.oidc_subject["pre-invoice-system"]
  values = {
    code = 200
    body = "{\"use_default\":true,\"use_immutable_subject\":false,\"sub_claim_prefix\":\"repo:Altinn/pre-invoice-system\"}"
  }
}
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
      "pre-invoice-system", "dis-padawan", "altinn-profile"
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

run "shared_repository_accepts_different_environments" {
  command = plan
  variables {
    product_syncroot_source_repos = {
      studio      = { repo_name = "altinn-studio", environments = ["adminservices_test", "adminservices_prod"], branches = ["main"] }
      studio-pdf3 = { repo_name = "altinn-studio", environments = ["at22", "at23", "tt02", "prod"], branches = ["main"] }
    }
  }
  assert {
    condition     = toset(keys(module.syncroot_github_repo)) == toset(["altinn-studio"])
    error_message = "Different product environments must retain one shared publishing identity."
  }
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
