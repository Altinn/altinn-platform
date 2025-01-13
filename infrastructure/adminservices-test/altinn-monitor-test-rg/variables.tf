variable "insights_workspace_test_dp" {
  type = map(string)
  default = {
    "dp-be-test-insightsWorkspace" = "dp-be-test-rg"
    "dp-be-yt01-insightsWorkspace" = "dp-be-yt01-rg"
  }
}