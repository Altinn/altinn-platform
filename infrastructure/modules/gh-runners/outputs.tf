output "vnet_id" {
  value       = azurerm_virtual_network.gh_runners.id
  description = "ID of the GitHub runners virtual network"
}

output "subnet_id" {
  value       = azurerm_subnet.gh_runners.id
  description = "ID of the GitHub runners subnet"
}
