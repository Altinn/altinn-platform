locals {
  today = formatdate("YYYY-MM-DD", timestamp())

  # Normalize values to lowercase where appropriate and build standardized tags
  tags = {
    finops_environment       = lower(var.finops_environment)
    finops_product           = lower(var.finops_product)
    finops_serviceownercode  = lower(var.finops_serviceownercode)
    finops_serviceownerorgnr = var.finops_serviceownerorgnr
    finops_capacity          = lower(var.finops_capacity)
    createdby                = lower(var.createdby)
    createddate              = local.today
    modifiedby               = lower(var.modifiedby)
    modifieddate             = local.today
    repository               = lower(var.repository)
  }
}
