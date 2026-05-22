resource "random_string" "apim_random_part" {
  length  = 6
  special = false
  upper   = false
  numeric = true
}
