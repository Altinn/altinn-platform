// Package githubauth turns the flux-dispatch GitHub App's private key into a
// short-lived installation access token, caching it until shortly before it
// expires. The App installation is the security boundary: dispatches can only
// reach repositories the App is installed on.
package githubauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/githubapi"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// jwtLifetime is the App JWT validity. GitHub rejects anything over 10 min.
	jwtLifetime = 10 * time.Minute
	// jwtBackdate absorbs clock skew between the pod and GitHub.
	jwtBackdate = 60 * time.Second
	// refreshWindow is how long before expiry a cached token is replaced.
	// Installation tokens are valid for an hour.
	refreshWindow = 5 * time.Minute
	// maxErrorBody caps how much of a GitHub error response is quoted back.
	maxErrorBody = 512
)

// TokenError describes a failure to obtain an installation access token. It
// reports whether the failure is permanent, so the caller can tell a GitHub
// outage (worth a retry) apart from a misconfigured App (never worth one).
type TokenError struct {
	// Status is the HTTP status GitHub returned, or 0 when the failure
	// happened before or outside an HTTP response.
	Status int
	// Reason is the human-readable detail.
	Reason string

	permanent bool
}

func (e *TokenError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("github app token exchange failed (%d): %s", e.Status, e.Reason)
	}
	return fmt.Sprintf("github app token exchange failed: %s", e.Reason)
}

// Permanent reports whether retrying could ever succeed. A malformed private
// key, a wrong App or installation ID, and a revoked key are all permanent; a
// 5xx, a rate limit, a timeout and a transport failure are not.
func (e *TokenError) Permanent() bool { return e.permanent }

// App mints installation access tokens for one GitHub App installation.
type App struct {
	appID          string
	installationID string
	apiBase        string
	httpClient     *http.Client

	// keyErr is the deferred result of parsing the PEM at construction time, so
	// a bad key surfaces on the first Token call rather than panicking at boot.
	privateKey any
	keyErr     error

	mu        sync.Mutex
	token     string
	expiresAt time.Time
	// fetching is non-nil while one caller is exchanging a JWT for a token; it
	// closes when that exchange finishes. Other callers wait on it instead of
	// queueing their own exchange behind the mutex.
	fetching chan struct{}
	// fetchErr is the outcome of the most recent exchange, shared with the
	// callers that waited on it.
	fetchErr error

	// now is injectable for tests.
	now func() time.Time
}

// New returns an App that signs with pem and exchanges JWTs at apiBase. hc may
// be nil, in which case http.DefaultClient is used.
func New(appID, installationID string, pem []byte, apiBase string, hc *http.Client) *App {
	if hc == nil {
		hc = http.DefaultClient
	}

	app := &App{
		appID:          appID,
		installationID: installationID,
		apiBase:        strings.TrimSuffix(apiBase, "/"),
		httpClient:     hc,
		now:            time.Now,
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(pem)
	if err != nil {
		app.keyErr = &TokenError{
			Reason:    fmt.Sprintf("parse GitHub App private key: %v", err),
			permanent: true,
		}
	} else {
		app.privateKey = key
	}

	return app
}

// Token returns a valid installation access token, reusing the cached one until
// it is within the refresh window of expiring.
//
// Only one exchange runs at a time. Concurrent callers wait for that exchange
// and share its result rather than each performing their own, which would turn
// a slow GitHub into a queue of serial timeouts. The wait honours ctx, so a
// caller whose client already disconnected does not hold up the line.
func (a *App) Token(ctx context.Context) (string, error) {
	if a.keyErr != nil {
		return "", a.keyErr
	}

	for {
		a.mu.Lock()

		if token, ok := a.cachedLocked(); ok {
			a.mu.Unlock()
			return token, nil
		}

		if wait := a.fetching; wait != nil {
			a.mu.Unlock()
			select {
			case <-wait:
			case <-ctx.Done():
				return "", &TokenError{Reason: ctx.Err().Error()}
			}

			a.mu.Lock()
			token, ok := a.cachedLocked()
			err := a.fetchErr
			a.mu.Unlock()

			if ok {
				return token, nil
			}
			if err != nil {
				return "", err
			}
			// The token was invalidated between the exchange finishing and
			// this caller waking up. Start over as the leader.
			continue
		}

		done := make(chan struct{})
		a.fetching = done
		a.mu.Unlock()

		token, expiresAt, err := a.fetchInstallationToken(ctx)

		a.mu.Lock()
		if err == nil {
			a.token, a.expiresAt = token, expiresAt
		}
		a.fetchErr = err
		a.fetching = nil
		a.mu.Unlock()
		close(done)

		if err != nil {
			return "", err
		}
		return token, nil
	}
}

// InvalidateToken drops the cached token when it is still the one the caller
// used. GitHub revokes installation tokens the moment the App key is rotated or
// the installation changes, long before the cached expiry, and nothing in the
// exchange itself reveals that. Without this, every dispatch keeps presenting a
// dead token until the refresh window opens — up to 55 minutes.
//
// The comparison matters: another caller may already have replaced the token,
// and dropping that one would cause a needless second exchange.
func (a *App) InvalidateToken(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if token != "" && a.token == token {
		a.token, a.expiresAt = "", time.Time{}
	}
}

// cachedLocked returns the cached token when it is still outside the refresh
// window. Caller holds the lock.
func (a *App) cachedLocked() (string, bool) {
	if a.token != "" && a.now().Before(a.expiresAt.Add(-refreshWindow)) {
		return a.token, true
	}
	return "", false
}

// installationTokenResponse is the subset of GitHub's response we use.
type installationTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// fetchInstallationToken exchanges a freshly signed App JWT for an installation
// access token.
func (a *App) fetchInstallationToken(ctx context.Context) (string, time.Time, error) {
	signed, err := a.signJWT()
	if err != nil {
		return "", time.Time{}, &TokenError{Reason: err.Error(), permanent: true}
	}

	endpoint, err := url.JoinPath(a.apiBase, "app", "installations", a.installationID, "access_tokens")
	if err != nil {
		return "", time.Time{}, &TokenError{
			Reason:    fmt.Sprintf("build access-token URL: %v", err),
			permanent: true,
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", time.Time{}, &TokenError{
			Reason:    fmt.Sprintf("build access-token request: %v", err),
			permanent: true,
		}
	}
	req.Header.Set("Authorization", "Bearer "+signed)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		// Transport failures and timeouts are transient by nature.
		return "", time.Time{}, &TokenError{Reason: fmt.Sprintf("request installation token: %v", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return "", time.Time{}, &TokenError{
			Status:    resp.StatusCode,
			Reason:    strings.TrimSpace(string(body)),
			permanent: !githubapi.Retryable(resp.StatusCode, resp.Header, body),
		}
	}

	var decoded installationTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", time.Time{}, &TokenError{
			Status: resp.StatusCode,
			Reason: fmt.Sprintf("decode installation token response: %v", err),
		}
	}
	if decoded.Token == "" {
		return "", time.Time{}, &TokenError{
			Status: resp.StatusCode,
			Reason: "installation token response contained no token",
		}
	}
	// A missing or zero expires_at would make the cache check below always
	// fail, silently re-minting a token on every single dispatch.
	if decoded.ExpiresAt.IsZero() {
		return "", time.Time{}, &TokenError{
			Status: resp.StatusCode,
			Reason: "installation token response contained no expires_at",
		}
	}

	return decoded.Token, decoded.ExpiresAt, nil
}

// signJWT builds the RS256 App JWT GitHub authenticates the token exchange with.
func (a *App) signJWT() (string, error) {
	now := a.now()
	claims := jwt.RegisteredClaims{
		Issuer:    a.appID,
		IssuedAt:  jwt.NewNumericDate(now.Add(-jwtBackdate)),
		ExpiresAt: jwt.NewNumericDate(now.Add(jwtLifetime)),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(a.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign GitHub App JWT: %w", err)
	}
	return signed, nil
}
