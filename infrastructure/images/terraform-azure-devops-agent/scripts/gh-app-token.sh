#!/bin/bash
#
# Request an ACCESS_TOKEN to be used by a GitHub APP
# Environment variable that need to be set up:
# * APP_ID, the GitHub's app ID
# * APP_PRIVATE_KEY, the content of GitHub app's private key in PEM format,
#   or APP_PRIVATE_KEY_BASE64, the same key base64 encoded
# * APP_LOGIN, the login name used to install GitHub's app
#

_GITHUB_HOST=${GITHUB_HOST:-"github.com"}

set -o pipefail

# If URL is not github.com then use the enterprise api endpoint
if [[ ${_GITHUB_HOST} = "github.com" ]]; then
  URI="https://api.${_GITHUB_HOST}"
else
  URI="https://${_GITHUB_HOST}/api/v3"
fi

API_VERSION=v3
API_HEADER="Accept: application/vnd.github.${API_VERSION}+json"
CONTENT_LENGTH_HEADER="Content-Length: 0"
APP_INSTALLATIONS_URI="${URI}/app/installations?per_page=100"

# Bound both API calls so a hung connection cannot keep the agent from starting
CURL_CONNECT_TIMEOUT=10
CURL_MAX_TIME=30

JWT_IAT_DRIFT=60
JWT_EXP_DELTA=600

JWT_JOSE_HEADER='{
    "alg": "RS256",
    "typ": "JWT"
}'


die() {
    echo 1>&2 "error: $*"
    exit 1
}

# The private key can be handed to us base64 encoded, which is how it has to
# travel through an Azure DevOps secret variable since those cannot hold
# newlines. Whitespace is stripped before decoding, so a value that picked up
# line breaks on the way still decodes.
decode_private_key() {
    local decoded
    decoded=$(tr -d '[:space:]' <<< "${APP_PRIVATE_KEY_BASE64}" | base64 -d 2>/dev/null) \
        || die "APP_PRIVATE_KEY_BASE64 does not hold valid base64"
    [[ ${decoded} == *"-----BEGIN"*"PRIVATE KEY-----"* ]] \
        || die "APP_PRIVATE_KEY_BASE64 did not decode to a PEM formatted private key"
    printf '%s\n' "${decoded}"
}

validate_environment() {
    [ -n "${APP_ID}" ] || die "missing APP_ID environment variable"
    [[ ${APP_ID} =~ ^[0-9]+$ ]] || die "APP_ID must be a number, got '${APP_ID}'"
    [ -n "${APP_LOGIN}" ] || die "missing APP_LOGIN environment variable"
    if [ -z "${APP_PRIVATE_KEY}" ]; then
        [ -n "${APP_PRIVATE_KEY_BASE64}" ] \
            || die "missing APP_PRIVATE_KEY or APP_PRIVATE_KEY_BASE64 environment variable"
        APP_PRIVATE_KEY=$(decode_private_key) || exit 1
    fi
}

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

# A PEM that has travelled through an environment variable or a secret store
# can end up with its newlines as literal '\n' escapes, which openssl cannot
# read. The base64 body of a PEM holds no other escapes, so expanding them is
# safe.
normalize_private_key() {
    if [[ $1 == *'\n'* ]]; then
        printf '%b\n' "$1"
    else
        printf '%s\n' "$1"
    fi
}

rs256_sign() {
    openssl dgst -binary -sha256 -sign <(normalize_private_key "$1")
}

# Runs curl and splits the response into HTTP_BODY and HTTP_STATUS. The
# Authorization header is read from ${auth_header} and handed to curl through a
# config file on stdin, so the JWT ends up neither in the process arguments nor
# on disk.
http_request() {
    local response
    response=$(printf 'header = "%s"\n' "${auth_header}" \
        | curl -sS -w '\n%{http_code}' \
            --connect-timeout "${CURL_CONNECT_TIMEOUT}" \
            --max-time "${CURL_MAX_TIME}" \
            --config - "$@") || return 1
    HTTP_STATUS=${response##*$'\n'}
    HTTP_BODY=${response%$'\n'*}
}

# The access token URL is taken from the API response, and the JWT is sent to
# it. Require https on the same authority we queried, so a tampered response
# cannot redirect the credential to another host. curl is not given -L, so it
# does not follow redirects away from it either.
validate_api_url() {
    local url=$1
    local expected=${URI#https://}
    expected=${expected%%/*}

    [[ ${url} == https://* ]] || return 1

    local authority=${url#https://}
    authority=${authority%%/*}
    [ "${authority}" = "${expected}" ]
}

# The error text the API returned, falling back to the raw body when the
# response is not the JSON we expect.
api_message() {
    local message
    message=$(jq --raw-output '.message? // empty' <<< "${HTTP_BODY}" 2>/dev/null)
    if [ -n "${message}" ]; then
        printf '%s' "${message}"
    else
        printf '%s' "${HTTP_BODY:0:200}"
    fi
}

request_access_token() {
    jwt_payload=$(build_jwt_payload)
    encoded_jwt_parts=$(base64url <<<"${JWT_JOSE_HEADER}").$(base64url <<<"${jwt_payload}")
    encoded_mac=$(echo -n "${encoded_jwt_parts}" | rs256_sign "${APP_PRIVATE_KEY}" | base64url) \
        || die "could not sign the JWT, check that APP_PRIVATE_KEY holds a PEM formatted private key"
    generated_jwt="${encoded_jwt_parts}.${encoded_mac}"

    auth_header="Authorization: Bearer ${generated_jwt}"

    http_request -X GET \
        -H "${API_HEADER}" \
        "${APP_INSTALLATIONS_URI}" \
        || die "could not reach ${APP_INSTALLATIONS_URI}"

    [ "${HTTP_STATUS}" = "200" ] \
        || die "listing the installations of app ${APP_ID} failed with HTTP ${HTTP_STATUS}: $(api_message)"

    installations=${HTTP_BODY}
    access_token_url=$(jq --raw-output \
        --arg login "${APP_LOGIN}" \
        --argjson app_id "${APP_ID}" \
    '
        map(select(
            ((.account.login // "") | ascii_downcase) == ($login | ascii_downcase)
            and .app_id == $app_id
        ))
        | .[0].access_tokens_url // empty
    ' <<< "${installations}") || die "could not parse the app installations response: $(api_message)"

    if [ -z "${access_token_url}" ]; then
        available=$(jq --raw-output '[.[] | "\(.account.login // "?") (app_id \(.app_id))"] | join(", ")' <<< "${installations}" 2>/dev/null)
        die "app ${APP_ID} is not installed on '${APP_LOGIN}' at ${_GITHUB_HOST}, available installations: ${available:-none}"
    fi

    validate_api_url "${access_token_url}" \
        || die "refusing to send the JWT to '${access_token_url}', expected an https URL on ${URI}"

    http_request -X POST \
        -H "${CONTENT_LENGTH_HEADER}" \
        -H "${API_HEADER}" \
        "${access_token_url}" \
        || die "could not reach ${access_token_url}"

    [ "${HTTP_STATUS}" = "201" ] \
        || die "requesting an installation access token failed with HTTP ${HTTP_STATUS}: $(api_message)"

    token=$(jq --raw-output '.token // empty' <<< "${HTTP_BODY}")
    [ -n "${token}" ] || die "the installation access token response contained no token"

    printf '%s\n' "${token}"
}

validate_environment
request_access_token
