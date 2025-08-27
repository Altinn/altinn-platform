resource "azurerm_dns_ns_record" "child_zone" {
  name                = replace(azurerm_dns_zone.child_zone.name, "/\\.${var.parent_dns_zone_name}$/", "")
  zone_name           = var.parent_dns_zone_name
  resource_group_name = var.parent_dns_zone_rg
  ttl                 = 300
  records             = azurerm_dns_zone.child_zone.name_servers
  provider            = azurerm.parent_zone
}
