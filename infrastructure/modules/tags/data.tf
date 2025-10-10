# Data source to fetch organization data from Altinn CDN
data "http" "altinn_orgs" {
  url = "https://altinncdn.no/orgs/altinn-orgs.json"

  # Add timeout and retry configuration for better reliability
  request_timeout_ms = 30000

  retry {
    attempts     = 3
    min_delay_ms = 1000
    max_delay_ms = 5000
  }
}
