#!/bin/bash
#
# git credential helper handing the GitHub App installation token, obtained by
# the entrypoint, to git. Only the 'get' operation is implemented, 'store' and
# 'erase' are no-ops since the token is managed by the entrypoint.
#
set -e

GH_APP_TOKEN_FILE=${GH_APP_TOKEN_FILE:-/azp/.github_token}

if [ "$1" != "get" ]; then
  exit 0
fi

if [ ! -s "$GH_APP_TOKEN_FILE" ]; then
  exit 0
fi

echo "username=x-access-token"
echo "password=$(cat "$GH_APP_TOKEN_FILE")"
