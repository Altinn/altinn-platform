#!/bin/bash
#
# Wrapper around the base image entrypoint (/azp/start.sh).
#
# If the GitHub App environment variables are set, an installation access token
# is requested and git is configured to use it when talking to GitHub over
# https. The token is written to a file and handed to git through a credential
# helper so it never ends up in the git config itself.
#
# Environment variables:
# * APP_ID, the GitHub app's ID
# * APP_PRIVATE_KEY, the content of the GitHub app's private key in PEM format,
#   or APP_PRIVATE_KEY_BASE64, the same key base64 encoded. Use the latter when
#   the value comes from an Azure DevOps secret variable, those cannot hold
#   newlines
# * APP_LOGIN, the login name (org) the GitHub app is installed on
# * GITHUB_HOST, optional, defaults to github.com
#
set -e

# /azp in the image, where the base image entrypoint lives as well
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

_GITHUB_HOST=${GITHUB_HOST:-"github.com"}
GH_APP_TOKEN_FILE=${GH_APP_TOKEN_FILE:-/azp/.github_token}

print_header() {
  lightcyan='\033[1;36m'
  nocolor='\033[0m'
  echo -e "${lightcyan}$1${nocolor}"
}

configure_git_github_app_auth() {
  print_header "Requesting GitHub App installation token for '${APP_LOGIN}' on ${_GITHUB_HOST}..."

  # Keep the assignment separate from the local declaration, otherwise the
  # exit status of the script is masked
  local token
  if ! token=$("${SCRIPT_DIR}/gh-app-token.sh") || [ -z "$token" ] || [ "$token" == "null" ]; then
    echo 1>&2 "error: could not obtain a GitHub App installation token, see the error above"
    exit 1
  fi

  install -m 600 /dev/null "$GH_APP_TOKEN_FILE"
  printf '%s' "$token" > "$GH_APP_TOKEN_FILE"
  unset token

  # The helper resolves to /usr/local/bin/git-credential-gh-app, which reads
  # the token from $GH_APP_TOKEN_FILE.
  export GH_APP_TOKEN_FILE
  git config --global "credential.https://${_GITHUB_HOST}.username" x-access-token
  git config --global "credential.https://${_GITHUB_HOST}.helper" gh-app

  echo "git configured to authenticate against https://${_GITHUB_HOST} as the GitHub App installation"
}

if [ -n "$APP_ID" ] || [ -n "$APP_PRIVATE_KEY" ] || [ -n "$APP_PRIVATE_KEY_BASE64" ] || [ -n "$APP_LOGIN" ]; then
  if [ -z "$APP_ID" ] || [ -z "$APP_LOGIN" ] ||
     { [ -z "$APP_PRIVATE_KEY" ] && [ -z "$APP_PRIVATE_KEY_BASE64" ]; }; then
    echo 1>&2 "error: APP_ID, APP_LOGIN and one of APP_PRIVATE_KEY or APP_PRIVATE_KEY_BASE64 must all be set to use GitHub App authentication"
    exit 1
  fi
  configure_git_github_app_auth
else
  echo "No GitHub App environment variables set, skipping GitHub App authentication for git"
fi

# Keep the private key out of the environment of the agent and its jobs
unset APP_PRIVATE_KEY APP_PRIVATE_KEY_BASE64

exec "${SCRIPT_DIR}/start.sh" "$@"
