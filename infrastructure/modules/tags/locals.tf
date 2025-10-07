locals {
  today = formatdate("YYYY-MM-DD", timestamp())

  # Parse organization data from Altinn CDN
  orgs_response = jsondecode(data.http.altinn_orgs.response_body)

  # Create lookup map from service owner code to organization number
  org_lookup = {
    for code, org in local.orgs_response.orgs :
    code => org.orgnr
  }

  # Sum all provided capacity values
  total_vcpus = sum(values(var.capacity_values))

  # Normalize values to lowercase where appropriate and build standardized tags
  tags = {
    finops_environment       = lower(var.finops_environment)
    finops_product           = lower(var.finops_product)
    finops_serviceownercode  = lower(var.finops_serviceownercode)
    finops_serviceownerorgnr = var.finops_serviceownerorgnr != null ? var.finops_serviceownerorgnr : lookup(local.org_lookup, lower(var.finops_serviceownercode), "")
    finops_capacity          = "${local.total_vcpus}vcpu"
    createdby                = lower(var.createdby)
    createddate              = local.today
    modifiedby               = lower(var.modifiedby)
    modifieddate             = local.today
    repository               = lower(var.repository)
  }
}
