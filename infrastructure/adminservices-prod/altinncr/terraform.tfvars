subscription_id = "a6e9ee7d-2b65-41e1-adfb-0c8c23515cf9"
acr_rgname      = "acr"
acrname         = "altinncr"
cache_rules = [
  {
    name              = "traefik"
    target_repo       = "traefik"
    source_repo       = "docker.io/library/traefik"
    credential_set_id = "/credentialSets/dockerhub"
  },
  {
    name              = "postgres"
    target_repo       = "postgres"
    source_repo       = "docker.io/library/postgres"
    credential_set_id = "/credentialSets/dockerhub"
  },
  {
    name              = "browserless"
    target_repo       = "browserless/chrome"
    source_repo       = "docker.io/browserless/chrome"
    credential_set_id = "/credentialSets/dockerhub"
  },
  {
    name              = "alpine"
    target_repo       = "alpine/*"
    source_repo       = "docker.io/alpine/*"
    credential_set_id = "/credentialSets/dockerhub"
  },
  {
    name              = "linkerd"
    target_repo       = "linkerd/*"
    source_repo       = "ghcr.io/linkerd/*"
    credential_set_id = null
  },
  {
    name              = "grafana"
    target_repo       = "grafana/*"
    source_repo       = "docker.io/grafana/*"
    credential_set_id = "/credentialSets/dockerhub"
  },
  {
    name              = "altinn-platform"
    target_repo       = "altinn-platform/*"
    source_repo       = "ghcr.io/altinn-platform/*"
    credential_set_id = null
  }
]
