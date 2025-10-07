# Data source to fetch organization data from Altinn CDN
data "http" "altinn_orgs" {
  url = "https://altinncdn.no/orgs/altinn-orgs.json"
}
