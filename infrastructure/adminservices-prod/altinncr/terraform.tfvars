subscription_id = "a6e9ee7d-2b65-41e1-adfb-0c8c23515cf9"
acr_rgname      = "acr"
acrname         = "altinncr"
cache_rules = [
  {
    name              = "dockerio"
    target_repo       = "docker.io/*"
    source_repo       = "docker.io/*"
    credential_set_id = "/credentialSets/dockerhub"
  },
  {
    name              = "quayio"
    target_repo       = "quay.io/*"
    source_repo       = "quay.io/*"
    credential_set_id = null
  },
  {
    name              = "ghcrio"
    target_repo       = "ghcr.io/*"
    source_repo       = "ghcr.io/*"
    credential_set_id = null
  },
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
  }
]

acr_push_object_ids = [
  {
    object_id = "c5a4a3da-2990-4734-9281-c40368ac0861" # SP: GitHub: altinn/altinn-platform
    type      = "ServicePrincipal"
  },
  {
    object_id = "fc900cb0-fb05-45f9-be11-f4a663b3c9a3" # SP: GitHub: altinn/altinn-tools
    type      = "ServicePrincipal"
  },
  {
    object_id = "27bfa3f2-2b60-4de5-a3b9-09dd3b01b490" # SP: Github: altinn/correspondence-prod
    type      = "ServicePrincipal"
  },
  {
    object_id = "d3c35a12-5465-4ba3-b50d-8ab1bedbef2a" # SP: Github: altinn/correspondence-test
    type      = "ServicePrincipal"
  },
  {
    object_id = "e3d2ce71-0d61-4332-9d44-278aac2846ab" # SP: Github: altinn/correspondence-staging
    type      = "ServicePrincipal"
  },
  {
    object_id = "3f5e6dcb-b782-49ca-939f-fd21dda34e4e" # SP: Github: altinn/broker-prod
    type      = "ServicePrincipal"
  },
  {
    object_id = "e5213800-3bdc-4f13-a212-0f4c8cd6c1ea" # SP: Github: altinn/broker-test
    type      = "ServicePrincipal"
  },
  {
    object_id = "512341b6-a349-4d08-8301-154a48a43b13" # SP: Github: altinn/altinn-studio dev
    type      = "ServicePrincipal"
  },
  {
    object_id = "99357379-c461-45bc-b9e7-6a2a51ccdac0" # SP: Github: altinn/altinn-studio prod
    type      = "ServicePrincipal"
  }
]

acr_pull_object_ids = [
  {
    object_id = "416302ed-fbab-41a4-8c8d-61f486fa79ca" # Group: Altinn-30-Test-Developers
    type      = "Group"
  }
]

user_access_admin_acr_pull_object_ids = [
  {
    object_id = "4da564b5-c526-42f2-aa01-108c5b4932f2" # uami: github-core_dis-core-test
    type      = "ServicePrincipal"
  },
  {
    object_id = "476e7604-4701-48da-953e-e6616cfedd15" # uami: github-core_dis-core-staging
    type      = "ServicePrincipal"
  },
  {
    object_id = "9bfd0ad4-9f59-4ad5-b8e5-a9664ec375fd" # uami: github-core_dis-core-prod
    type      = "ServicePrincipal"
  }
]
