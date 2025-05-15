module "observability" {
  source                          = "../../modules/observability"
  depends_on                      = [module.aks]
  prefix                          = "auth"
  environment                     = "at22"
  oidc_issuer_url                 = module.aks.aks_oidc_issuer_url
}
