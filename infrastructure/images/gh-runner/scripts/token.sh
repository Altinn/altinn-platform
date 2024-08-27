#!/bin/bash

_GITHUB_HOST=${GITHUB_HOST:-"github.com"}
# If URL is not github.com then use the enterprise api endpoint
if [[ ${_GITHUB_HOST} = "github.com" ]]; then
  URI="https://api.${_GITHUB_HOST}"
else
  URI="https://${_GITHUB_HOST}/api/v3"
fi

API_VERSION=v3
API_HEADER="Accept: application/vnd.github.${API_VERSION}+json"
AUTH_HEADER="Authorization: token ${ACCESS_TOKEN}"
CONTENT_LENGTH_HEADER="Content-Length: 0"

case ${RUNNER_SCOPE} in
  org*)
    _FULL_URL="${URI}/orgs/${ORG_NAME}/actions/runners/registration-token"
    ;;
    
  *)
    _FULL_URL="${URI}/repos/${ORG_NAME}/${REPO_NAME}/actions/runners/registration-token"
    ;;
esac

RUNNER_TOKEN="$(curl -XPOST -fsSL \
  -H "${CONTENT_LENGTH_HEADER}" \
  -H "${AUTH_HEADER}" \
  -H "${API_HEADER}" \
  "${_FULL_URL}" \
| jq -r '.token')"

echo "{\"token\": \"${RUNNER_TOKEN}\", \"full_url\": \"${_FULL_URL}\"}"