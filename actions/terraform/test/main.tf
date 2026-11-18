terraform {
  backend "azurerm" {}
}

variable "run_id" {
  type        = string
  description = "Changes on every run, so every plan has a change to apply."
}

resource "terraform_data" "run" {
  input = var.run_id
}
