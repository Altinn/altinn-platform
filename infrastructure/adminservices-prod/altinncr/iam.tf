resource "azurerm_role_assignment" "altinncr_acrpush" {
  for_each                         = { for pusher in var.acr_push_object_ids : pusher.object_id => pusher }
  principal_id                     = each.value.object_id
  role_definition_name             = "AcrPush"
  principal_type                   = each.value.type
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "altinncr_acrpull" {
  for_each                         = { for puller in var.acr_pull_object_ids : puller.object_id => puller }
  principal_id                     = each.value.object_id
  role_definition_name             = "AcrPull"
  principal_type                   = each.value.type
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "altinncr_reader" {
  for_each                         = { for puller in var.acr_pull_object_ids : puller.object_id => puller }
  principal_id                     = each.value.object_id
  role_definition_name             = "Reader"
  principal_type                   = each.value.type
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "altinncr_acrpush_altinn_platform" {
  principal_id                     = azurerm_user_assigned_identity.github_pusher.principal_id
  role_definition_name             = "AcrPush"
  principal_type                   = "ServicePrincipal"
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "altinncr_writer_corr" {
  principal_id         = "27bfa3f2-2b60-4de5-a3b9-09dd3b01b490"
  role_definition_name = "Container Registry Repository Writer"
  principal_type       = "ServicePrincipal"
  scope                = azurerm_container_registry.acr.id
  condition            = <<-EOT
(
 (
  !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/content/write'})
  AND
  !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/content/read'})
  AND
  !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/metadata/read'})
  AND
  !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/metadata/write'})
 )
 OR 
 (
  @Request[Microsoft.ContainerRegistry/registries/repositories:name] StringStartsWith 'corr'
 )
)
EOT
}

resource "azurerm_role_assignment" "altinncr_writer_corr_test" {
  principal_id         = "ec40031a-d620-4313-a10b-ac2fb329281e"
  role_definition_name = "Container Registry Repository Writer"
  principal_type       = "ServicePrincipal"
  scope                = azurerm_container_registry.acr.id
  condition            = <<-EOT
(
 (
  !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/content/write'})
  AND
  !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/content/read'})
  AND
  !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/metadata/read'})
  AND
  !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/metadata/write'})
 )
 OR 
 (
  @Request[Microsoft.ContainerRegistry/registries/repositories:name] StringStartsWith 'corr'
 )
)
EOT
}

resource "azurerm_role_assignment" "altinncr_writer_broker" {
  principal_id         = "3f5e6dcb-b782-49ca-939f-fd21dda34e4e"
  role_definition_name = "Container Registry Repository Writer"
  principal_type       = "ServicePrincipal"
  scope                = azurerm_container_registry.acr.id
  condition            = <<-EOT
(
 (
  !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/content/write'})
  AND
  !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/content/read'})
  AND
  !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/metadata/read'})
  AND
  !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/metadata/write'})
 )
 OR 
 (
  @Request[Microsoft.ContainerRegistry/registries/repositories:name] StringStartsWith 'corr'
 )
)
EOT
}
