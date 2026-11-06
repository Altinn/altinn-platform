#!/bin/bash
#
# Request an ACCESS_TOKEN to be used by a GitHub APP
# Environment variable that need to be set up:
# * APP_ID, the GitHub's app ID
# * APP_PRIVATE_KEY, the content of GitHub app's private key in PEM format.
# * APP_LOGIN, the organization the GitHub app is installed on
# Optional:
# * INSTALL_ID, the GitHub app's installation ID. Saves an API call per runner, as the installation does not have to be looked up
#

set -euo pipefail

# shellcheck source=github-api.sh
source "$(dirname "${BASH_SOURCE[0]}")/github-api.sh"

JWT_IAT_DRIFT=60
JWT_EXP_DELTA=600

JWT_JOSE_HEADER='{
    "alg": "RS256",
    "typ": "JWT"
}'


build_jwt_payload() {
    now=$(date +%s)
    iat=$((now - JWT_IAT_DRIFT))
    jq -c \
        --arg iat_str "${iat}" \
        --arg exp_delta_str "${JWT_EXP_DELTA}" \
        --arg app_id_str "${APP_ID}" \
    '
        ($iat_str | tonumber) as $iat
        | ($exp_delta_str | tonumber) as $exp_delta
        | ($app_id_str | tonumber) as $app_id
        | .iat = $iat
        | .exp = ($iat + $exp_delta)
        | .iss = $app_id
    ' <<< "{}" | tr -d '\n'
}

base64url() {
    base64 | tr '+/' '-_' | tr -d '=\n'
}

rs256_sign() {
    openssl dgst -binary -sha256 -sign <(echo "$1")
}

# Print an Authorization header with a freshly signed app JWT. The JWT is only
# valid for about 9 minutes, so github_api_request calls this before every
# attempt instead of reusing one JWT across long rate limit waits.
jwt_auth_header() {
    local jwt_payload encoded_jwt_parts encoded_mac
    jwt_payload=$(build_jwt_payload)
    encoded_jwt_parts=$(base64url <<<"${JWT_JOSE_HEADER}").$(base64url <<<"${jwt_payload}")
    encoded_mac=$(echo -n "${encoded_jwt_parts}" | rs256_sign "${APP_PRIVATE_KEY}" | base64url)
    echo "Authorization: Bearer ${encoded_jwt_parts}.${encoded_mac}"
}

request_access_token() {
    if [[ -n ${INSTALL_ID:-} ]]; then
        access_token_url="${GITHUB_API_URI}/app/installations/${INSTALL_ID}/access_tokens"
    else
        if ! installation_response=$(github_api_request GET "${GITHUB_API_URI}/orgs/${APP_LOGIN}/installation" jwt_auth_header); then
            echo "Could not find an installation of app ${APP_ID} on ${APP_LOGIN}" >&2
            exit 1
        fi
        access_token_url=$(jq -r '.access_tokens_url // empty' <<<"${installation_response}")
        if [[ -z ${access_token_url} ]]; then
            echo "Could not find an installation of app ${APP_ID} on ${APP_LOGIN}" >&2
            exit 1
        fi
    fi

    access_token_response=$(github_api_request POST "${access_token_url}" jwt_auth_header)
    access_token=$(jq -r '.token // empty' <<<"${access_token_response}")
    if [[ -z ${access_token} ]]; then
        echo "GitHub did not return an access token for app ${APP_ID}" >&2
        exit 1
    fi

    echo "${access_token}"
}

request_access_token
