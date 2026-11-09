#!/bin/bash
# shellcheck shell=bash
set -euo pipefail

export PATH=${PATH}:/actions-runner

##### ENVVARS
# APP_ID ID of app used for registering and setting up runner
# APP_PRIVATE_KEY Private key for app (should be kept in a vault)
# INSTALL_ID (optional) Installation ID of the app. Saves an API call per runner, as the installation does not have to be looked up
# ORG_NAME Name of the org the runner should be added to (For repo runners please supply REPO_NAME as well)
# REPO_NAME (optional) Name of the repository to add this runner to. Leave unset for org runners
# RUNNER_NAME_PREFIX (optional) The name will have random string add after the prefix. Default: github-runner
# RUNNER_WORKDIR (optional) Work dir for the runner. Default: /_work/${RUNNER_NAME}
# LABELS (optional) Runner labels. Default: default
# RUNNER_GROUP (optional) Name of runner group. Default: Default
# RUNNER_GROUP_ID (optional) Id of runner group. Default: 1
# GITHUB_API_RETRY_BUDGET (optional) Max seconds to wait for retries of a rate limited or failed GitHub API request. Default: 600 (max 3600)

# Un-export these, so that they must be passed explicitly to the environment of
# any command that needs them.  This may help prevent leaks.
export -n ACCESS_TOKEN
export -n RUNNER_TOKEN
export -n APP_ID
export -n APP_PRIVATE_KEY

_RUNNER_NAME=${RUNNER_NAME_PREFIX:-github-runner}-$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c 13 ; echo '')
_RUNNER_WORKDIR=${RUNNER_WORKDIR:-/_work/${_RUNNER_NAME}}
_LABELS=${LABELS:-default}
_RUNNER_GROUP=${RUNNER_GROUP:-Default}
_RUNNER_GROUP_ID=${RUNNER_GROUP_ID:-1}
_GITHUB_HOST=${GITHUB_HOST:-"github.com"}
_BASE_URI="https://${_GITHUB_HOST}"

## Unset these, this may help prevent leaks
unset_env() {
  unset ACCESS_TOKEN
  unset RUNNER_TOKEN
  unset APP_ID
  unset APP_PRIVATE_KEY
}

if [[ -z ${APP_ID:-} ]]; then
  echo "APP_ID is required"
  exit 1
fi
if [[ -z ${APP_PRIVATE_KEY:-} ]]; then
  echo "APP_PRIVATE_KEY is required"
  exit 1
fi
if [[ -z ${ORG_NAME:-} ]]; then
  echo "ORG_NAME is required, to define a Repo runner define REPO_NAME as well"
  exit 1
fi

APP_LOGIN="${ORG_NAME}"

if [[ -z ${REPO_NAME:-} ]]; then
  _REPO_URL="${_BASE_URI}/${ORG_NAME}"
  RUNNER_SCOPE="org"
else
  _REPO_URL="${_BASE_URI}/${ORG_NAME}/${REPO_NAME}"
  RUNNER_SCOPE="repo"
fi

echo "Obtaining access token for app_id ${APP_ID} and login ${APP_LOGIN}"

# Keys stored with literal "\n" sequences instead of line breaks are converted to a valid PEM
nl=$'\n'
if ! ACCESS_TOKEN=$(APP_ID="${APP_ID}" APP_PRIVATE_KEY="${APP_PRIVATE_KEY//\\n/${nl}}" APP_LOGIN="${APP_LOGIN}" INSTALL_ID="${INSTALL_ID:-}" bash ./app-token.sh); then
  echo "Failed to obtain an access token for app_id ${APP_ID}"
  exit 1
fi

# Retrieve a short lived runner registration token using the APP_LOGIN
if ! _TOKEN=$(ACCESS_TOKEN="${ACCESS_TOKEN}" ORG_NAME="${ORG_NAME}" REPO_NAME="${REPO_NAME:-}" RUNNER_SCOPE="${RUNNER_SCOPE}" bash ./token.sh); then
  echo "Failed to obtain a runner registration token for ${_REPO_URL}"
  exit 1
fi
RUNNER_TOKEN=$(echo "${_TOKEN}" | jq -r '.token // empty')
if [[ -z ${RUNNER_TOKEN} ]]; then
  echo "GitHub did not return a runner registration token for ${_REPO_URL}"
  exit 1
fi

mkdir -p "${_RUNNER_WORKDIR}"

./config.sh \
    --url "${_REPO_URL}" \
    --token "${RUNNER_TOKEN}" \
    --labels "${_LABELS}" \
    --work "${_RUNNER_WORKDIR}" \
    --name "${_RUNNER_NAME}" \
    --runnergroup "${_RUNNER_GROUP}" \
    --unattended \
    --replace \
    --ephemeral

unset_env
./run.sh
