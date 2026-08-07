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
		app.keyErr = fmt.Errorf("parse GitHub App private key: %w", err)
	} else {
		app.privateKey = key
	}

	return app
}

// Token returns a valid installation access token, reusing the cached one until
// it is within the refresh window of expiring.
func (a *App) Token(ctx context.Context) (string, error) {
	if a.keyErr != nil {
		return "", a.keyErr
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.token != "" && a.now().Before(a.expiresAt.Add(-refreshWindow)) {
		return a.token, nil
	}

	token, expiresAt, err := a.fetchInstallationToken(ctx)
	if err != nil {
		return "", err
	}

	a.token, a.expiresAt = token, expiresAt
	return a.token, nil
}

// installationTokenResponse is the subset of GitHub's response we use.
type installationTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// fetchInstallationToken exchanges a freshly signed App JWT for an installation
// access token. Caller holds the lock.
func (a *App) fetchInstallationToken(ctx context.Context) (string, time.Time, error) {
	signed, err := a.signJWT()
	if err != nil {
		return "", time.Time{}, err
	}

	endpoint, err := url.JoinPath(a.apiBase, "app", "installations", a.installationID, "access_tokens")
	if err != nil {
		return "", time.Time{}, fmt.Errorf("build access-token URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("build access-token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+signed)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("request installation token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return "", time.Time{}, fmt.Errorf("installation token request returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded installationTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", time.Time{}, fmt.Errorf("decode installation token response: %w", err)
	}
	if decoded.Token == "" {
		return "", time.Time{}, fmt.Errorf("installation token response contained no token")
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
