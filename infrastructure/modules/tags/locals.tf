locals {
  today = formatdate("YYYY-MM-DD", timestamp())

  # Parse organization data from Altinn CDN with error handling
  orgs_response = can(jsondecode(data.http.altinn_orgs.response_body)) ? jsondecode(data.http.altinn_orgs.response_body) : { orgs = {} }

  # Create lookup map from service owner code to organization number
  org_lookup = {
    for code, org in local.orgs_response.orgs :
    code => org.orgnr
  }

  # Validate that the service owner code exists in the fetched data (when not using manual override)
  service_owner_exists = var.finops_serviceownerorgnr != null ? true : contains(keys(local.org_lookup), lower(var.finops_serviceownercode))

  # Validation: Fail if service owner code doesn't exist in external data and no manual override is provided
  validate_service_owner = var.finops_serviceownerorgnr == null && !local.service_owner_exists ? tobool("Service owner code '${var.finops_serviceownercode}' not found in Altinn organization registry. Check https://altinncdn.no/orgs/altinn-orgs.json for valid codes or provide finops_serviceownerorgnr manually.") : true

  # Sum all provided capacity values
  total_vcpus = sum(var.capacity_values)

  # Determine if capacity tag should be included
  # Auto-determine: include if capacity_values is provided, unless explicitly set to false
  # Explicit: use the include_capacity_tag value when provided
  should_include_capacity = var.include_capacity_tag != null ? var.include_capacity_tag : (length(var.capacity_values) > 0)

  # Base tags that are always included
  base_tags = {
    finops_environment       = lower(var.finops_environment)
    finops_product           = lower(var.finops_product)
    finops_serviceownercode  = lower(var.finops_serviceownercode)
    finops_serviceownerorgnr = var.finops_serviceownerorgnr != null ? var.finops_serviceownerorgnr : (local.service_owner_exists ? local.org_lookup[lower(var.finops_serviceownercode)] : "")
    createdby                = lower(var.current_user)
    createddate              = local.today
    modifiedby               = lower(var.current_user)
    modifieddate             = local.today
    repository               = lower(var.repository)
  }

  # Capacity tag (only for computing resources)
  capacity_tags = local.should_include_capacity ? {
    finops_capacity = "${local.total_vcpus}vcpu"
  } : {}

  # Combine base tags with conditional capacity tags
  tags = merge(local.base_tags, local.capacity_tags)
}
