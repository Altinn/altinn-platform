#!/bin/bash
#
# Request a short lived runner registration token
# Environment variable that need to be set up:
# * ACCESS_TOKEN, a GitHub app installation access token
# * ORG_NAME, the organization the runner is registered in
# * RUNNER_SCOPE, "org" for an organization runner or "repo" for a repository runner
# * REPO_NAME, the repository the runner is registered in. Only used for repository runners
#

set -euo pipefail

# shellcheck source=github-api.sh
source "$(dirname "${BASH_SOURCE[0]}")/github-api.sh"

AUTH_HEADER="Authorization: token ${ACCESS_TOKEN}"

case ${RUNNER_SCOPE:-} in
  org*)
    _FULL_URL="${GITHUB_API_URI}/orgs/${ORG_NAME}/actions/runners/registration-token"
    ;;

  *)
    _FULL_URL="${GITHUB_API_URI}/repos/${ORG_NAME}/${REPO_NAME}/actions/runners/registration-token"
    ;;
esac

RUNNER_TOKEN=$(github_api_request POST "${_FULL_URL}" "${AUTH_HEADER}" | jq -r '.token // empty')

jq -n -c --arg token "${RUNNER_TOKEN}" --arg full_url "${_FULL_URL}" '{token: $token, full_url: $full_url}'
