locals {
  today = formatdate("YYYY-MM-DD", timestamp())

  # Sum all provided capacity values
  total_vcpus = sum(values(var.capacity_values))

  # Normalize values to lowercase where appropriate and build standardized tags
  tags = {
    finops_environment       = lower(var.finops_environment)
    finops_product           = lower(var.finops_product)
    finops_serviceownercode  = lower(var.finops_serviceownercode)
    finops_serviceownerorgnr = var.finops_serviceownerorgnr
    finops_capacity          = "${local.total_vcpus}vcpu"
    createdby                = lower(var.createdby)
    createddate              = local.today
    modifiedby               = lower(var.modifiedby)
    modifieddate             = local.today
    repository               = lower(var.repository)
  }
}
