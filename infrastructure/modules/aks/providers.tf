terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
-      version = "< 4.31.0"
+      version = ">= 4.0.0, < 4.31.0"
    }
  }
}
    }
  }
}
