environments = [
  {
    name = "dev"
    workspaces = [
      {
        arm_subscription = "dev"
        names            = ["dev"]
      },
      {
        arm_subscription = "test"
        names            = ["at21", "at22", "at23", "at24", "at25"]
    }]
  },
  {
    name = "prod"
    workspaces = [
      {
        arm_subscription = "staging"
        names            = ["tt02"]
      },
      {
        arm_subscription = "prod"
        names            = ["prod"]
    }]
  }
]
