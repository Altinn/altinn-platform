// Package githubapi holds the GitHub REST semantics shared by the token
// exchange and the repository_dispatch call: deciding whether a response is
// worth retrying. Both callers must agree on this, because the answer decides
// whether the service asks Flux to redeliver an event or acknowledges it.
package githubapi

import (
	"net/http"
	"strings"
)

// Retryable reports whether an unsuccessful GitHub REST response is transient.
//
// A plain status check is not enough: GitHub signals both primary and secondary
// rate limits with 403 as well as 429, and those are transient — treating them
// as permanent silently drops the request. Every other 4xx (bad credentials,
// App not installed, unprocessable payload) is permanent.
func Retryable(status int, header http.Header, body []byte) bool {
	switch {
	case status >= 500:
		return true
	case status == http.StatusTooManyRequests:
		return true
	case status == http.StatusForbidden:
		return rateLimited(header, body)
	default:
		return false
	}
}

// rateLimited reports whether a 403 is GitHub throttling rather than refusing.
// GitHub marks the throttled case with Retry-After, an exhausted rate-limit
// budget, or an explicit message in the body.
func rateLimited(header http.Header, body []byte) bool {
	if header.Get("Retry-After") != "" {
		return true
	}
	if header.Get("X-RateLimit-Remaining") == "0" {
		return true
	}
	message := strings.ToLower(string(body))
	return strings.Contains(message, "rate limit") || strings.Contains(message, "abuse")
}
