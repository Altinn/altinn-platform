#!/bin/bash
#
# Helpers for calling the GitHub REST API. Meant to be sourced, not executed.
#
# All runners share the rate limit of the GitHub App installation they use, so
# requests are retried with backoff when GitHub reports rate limiting instead of
# failing the runner (which makes KEDA start a new one and spend more requests).
#
# Optional environment variables:
# * GITHUB_HOST, the GitHub host. Default: github.com
# * GITHUB_API_RETRY_BUDGET, max seconds to spend waiting for retries per request. Default: 600 (max 3600)

_GITHUB_HOST=${GITHUB_HOST:-"github.com"}

# If URL is not github.com then use the enterprise api endpoint
# shellcheck disable=SC2034 # GITHUB_API_URI is used by the scripts sourcing this file
if [[ ${_GITHUB_HOST} = "github.com" ]]; then
  GITHUB_API_URI="https://api.${_GITHUB_HOST}"
else
  GITHUB_API_URI="https://${_GITHUB_HOST}/api/v3"
fi

_GITHUB_API_RETRY_BUDGET=${GITHUB_API_RETRY_BUDGET:-600}
if [[ ! ${_GITHUB_API_RETRY_BUDGET} =~ ^[0-9]+$ ]]; then
  echo "GITHUB_API_RETRY_BUDGET must be a whole number of seconds, got '${_GITHUB_API_RETRY_BUDGET}'. Using 600" >&2
  _GITHUB_API_RETRY_BUDGET=600
fi
# Strip leading zeros, so a value such as "090" is not read as octal
_GITHUB_API_RETRY_BUDGET=${_GITHUB_API_RETRY_BUDGET#"${_GITHUB_API_RETRY_BUDGET%%[!0]*}"}
_GITHUB_API_RETRY_BUDGET=${_GITHUB_API_RETRY_BUDGET:-0}
# Cap at the installation token lifetime (1 hour). The length check comes first,
# so huge values can't overflow bash arithmetic into a negative budget.
_GITHUB_API_MAX_RETRY_BUDGET=3600
if ((${#_GITHUB_API_RETRY_BUDGET} > ${#_GITHUB_API_MAX_RETRY_BUDGET} || _GITHUB_API_RETRY_BUDGET > _GITHUB_API_MAX_RETRY_BUDGET)); then
  echo "GITHUB_API_RETRY_BUDGET is capped at ${_GITHUB_API_MAX_RETRY_BUDGET} seconds" >&2
  _GITHUB_API_RETRY_BUDGET=${_GITHUB_API_MAX_RETRY_BUDGET}
fi
_GITHUB_API_MAX_ATTEMPTS=10

# Print the value of a response header from a curl header dump (case-insensitive).
_github_api_header() {
  grep -i "^$1:" "$2" | tail -n 1 | cut -d: -f2- | tr -d '[:space:]' || true
}

# Exponential backoff with jitter: 2, 4, 8, 16, 30, 30, ... seconds plus 0-4s.
_github_api_backoff() {
  local delay=$((2 ** $1))
  ((delay > 30)) && delay=30
  echo $((delay + RANDOM % 5))
}

# Print how many seconds to wait before retrying, or nothing if the response
# should not be retried.
_github_api_retry_delay() {
  local status=$1 headers=$2 body=$3 attempt=$4
  local retry_after remaining reset now

  case ${status} in
    403 | 429)
      retry_after=$(_github_api_header retry-after "${headers}")
      remaining=$(_github_api_header x-ratelimit-remaining "${headers}")
      reset=$(_github_api_header x-ratelimit-reset "${headers}")

      if [[ ${retry_after} =~ ^[0-9]+$ ]]; then
        echo $((retry_after > 0 ? retry_after : 1))
      elif [[ ${remaining} = "0" && ${reset} =~ ^[0-9]+$ ]]; then
        # Primary rate limit, wait until the window resets
        now=$(date +%s)
        echo $((reset > now ? reset - now + 1 : 1))
      elif [[ ${status} = "429" ]] || grep -qi "rate limit" "${body}"; then
        # Secondary rate limit without headers, GitHub asks for at least a minute
        echo $((60 + $(_github_api_backoff "${attempt}")))
      fi
      # Any other 403 is a permission problem, retrying will not help
      ;;
    000 | 5??)
      # Network error or server error
      _github_api_backoff "${attempt}"
      ;;
  esac
}

# github_api_request METHOD URL AUTH
#
# Call the GitHub API and print the response body on stdout. Retries rate
# limited, server and network errors for up to GITHUB_API_RETRY_BUDGET seconds.
# Errors are logged on stderr. Returns non-zero if the request did not succeed.
#
# AUTH is either a full "Authorization: ..." header, or the name of a function
# that prints one. A function is called before every attempt, so short lived
# credentials such as an app JWT are still valid after long waits.
github_api_request() {
  local method=$1 url=$2 auth=$3
  local headers body status delay message auth_header remaining_budget
  local attempt=1 waited=0 rc=1

  headers=$(mktemp)
  body=$(mktemp)

  while true; do
    if declare -F "${auth}" >/dev/null; then
      auth_header=$("${auth}") || { rm -f "${headers}" "${body}"; return 1; }
    else
      auth_header=${auth}
    fi

    status=$(curl -sS -X "${method}" \
      -H "${auth_header}" \
      -H "Accept: application/vnd.github.v3+json" \
      -H "Content-Length: 0" \
      -D "${headers}" \
      -o "${body}" \
      -w '%{http_code}' \
      "${url}") || status=000

    if [[ ${status} = 2?? ]]; then
      cat "${body}"
      rc=0
      break
    fi

    message=$(jq -r '.message // empty' "${body}" 2>/dev/null || true)
    delay=$(_github_api_retry_delay "${status}" "${headers}" "${body}" "${attempt}")

    if [[ -z ${delay} ]]; then
      echo "GitHub API ${method} ${url} failed with HTTP ${status}${message:+: ${message}}" >&2
      break
    fi
    remaining_budget=$((_GITHUB_API_RETRY_BUDGET - waited))
    if ((attempt >= _GITHUB_API_MAX_ATTEMPTS || remaining_budget <= 0)); then
      echo "GitHub API ${method} ${url} failed with HTTP ${status}${message:+: ${message}}, giving up after ${attempt} attempts and ${waited}s of waiting" >&2
      break
    fi
    # Wait out the rest of the budget rather than exiting straight away when the
    # rate limit resets later than that. Exiting would only make KEDA start a
    # new runner, which spends more requests against the same limit.
    ((delay > remaining_budget)) && delay=${remaining_budget}

    echo "GitHub API ${method} ${url} returned HTTP ${status}${message:+: ${message}}, retrying in ${delay}s (attempt ${attempt})" >&2
    sleep "${delay}"
    waited=$((waited + delay))
    attempt=$((attempt + 1))
  done

  rm -f "${headers}" "${body}"
  return "${rc}"
}
