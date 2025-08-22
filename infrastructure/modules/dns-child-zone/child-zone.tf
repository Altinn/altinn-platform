

resource "azurerm_resource_group" "dns_child_zone_rg" {
  name     = var.child_dns_zone_rg_name != "" ? var.child_dns_zone_rg_name : "${var.prefix}-${var.environment}-dns-rg"
  location = var.location
}

resource "azurerm_dns_zone" "child_zone" {
  name                = var.child_dns_zone_name != "" ? var.child_dns_zone_name : "${var.environment}.${var.prefix}.altinn.cloud"
  resource_group_name = azurerm_resource_group.dns_child_zone_rg.name
}

resource "azurerm_dns_a_record" "wildcard_record" {
  name                = "*"
  zone_name           = azurerm_dns_zone.child_zone.name
  resource_group_name = azurerm_dns_zone.child_zone.resource_group_name
  records             = toset(["${var.cluster_ipv4_address}"])
  ttl                 = 300
}

resource "azurerm_dns_aaaa_record" "wildcard_record" {
  name                = "*"
  zone_name           = azurerm_dns_zone.child_zone.name
  resource_group_name = azurerm_dns_zone.child_zone.resource_group_name
  records             = toset(["${var.cluster_ipv6_address}"])
  ttl                 = 300
}

resource "azurerm_dns_caa_record" "issue_lets_encrypt" {
  name                = "@"
  zone_name           = azurerm_dns_zone.child_zone.name
  resource_group_name = azurerm_dns_zone.child_zone.resource_group_name
  record {
    flags = 0
    tag   = "issue"
    value = "letsencrypt.org"
  }
  record {
    flags = 0
    tag   = "issuewild"
    value = "letsencrypt.org"
  }
  ttl = 300
}
