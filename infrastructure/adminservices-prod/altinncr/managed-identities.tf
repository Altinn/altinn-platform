resource "azurerm_user_assigned_identity" "github_pusher" {
  name                = "github-altinn-platform-pusher"
  location            = azurerm_resource_group.acr.location
  resource_group_name = azurerm_resource_group.acr.name
}

resource "azurerm_federated_identity_credential" "altinn_platform_federation_main" {
  name                = "github.altinn.altinn-platform.ref.main"
  parent_id           = azurerm_user_assigned_identity.github_pusher.id
  resource_group_name = azurerm_resource_group.acr.name
  audience            = "api://AzureADTokenExchange"
  issuer              = "https://token.actions.githubusercontent.com"
  subject             = "repo:Altinn/altinn-platform:ref:refs/heads/main"
}

resource "azurerm_federated_identity_credential" "altinn_platform_federation_flux_release" {
  name                = "github.altinn.altinn-platform.environment.flux-release"
  parent_id           = azurerm_user_assigned_identity.github_pusher.id
  resource_group_name = azurerm_resource_group.acr.name
  audience            = "api://AzureADTokenExchange"
  issuer              = "https://token.actions.githubusercontent.com"
  subject             = "repo:Altinn/altinn-platform:environment:flux-release"
}
