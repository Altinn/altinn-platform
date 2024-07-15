workspaces = [
  {
    name = "dev"
    environments = [
      {
        arm_subscription = "dev"
        names            = ["dev"]
      },
      {
        arm_subscription = "test"
        names            = ["test", "at21", "at22", "at23", "at24", "at25", "yt01"]
    }]
  },
  {
    name = "prod"
    environments = [
      {
        arm_subscription = "staging"
        names            = ["staging", "tt02"]
      },
      {
        arm_subscription = "prod"
        names            = ["prod"]
    }]
  }
]
