module "foundational" {
  source = "./modules/foundational"
  namespaces = [
    "authentication",
    "core",
    "correspondence",
    "dialogporten",
  ]
}

module "services" {
  depends_on = [module.foundational]
  source     = "./modules/services"

  k8s_rbac = {
    authentication = {
      namespace = "authentication"
      dev_group = "5c42ac79-86e2-46d0-85d3-ae751dd5f057"
      sp_group  = "328cbe61-aeb1-4782-bb36-d288c69b4f15"
    }

    core = {
      namespace = "core"
      dev_group = "4dde4651-a9ca-4df1-9e05-216272284c7d"
      sp_group  = "e87d6f10-6fc0-4a09-a9b0-e8c994ed4052"
    }

    correspondence = {
      namespace = "correspondence"
      dev_group = "954a4d24-8c7e-4382-9861-2b5d1a515253"
      sp_group  = "e36ca3b3-f495-45a5-bca4-4fc83424633f"
    }

    dialogporten = {
      namespace = "dialogporten",
      dev_group = "c403060d-5c8a-41b0-8c19-84fa60d0ce18"
      sp_group  = "b22b612d-9dc5-4f8b-8816-e551749bd19c"
    }
  }
  oidc_issuer_url       = "placeholder" # TODO: Update with real values
  remote_write_endpoint = "placeholder" # TODO: Update with real values
  /*
  providers = {

  }
  */
}
