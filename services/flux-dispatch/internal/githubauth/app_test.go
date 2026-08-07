package githubauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testAppID          = "123456"
	testInstallationID = "7891011"
)

// newTestKey generates a throwaway RSA key and returns it both as a parsed key
// and PEM-encoded, the way the App's private key is mounted in the pod.
func newTestKey(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return key, encoded
}

// fakeGitHub serves the installation access-token endpoint, counting hits and
// recording the Authorization header of the last request.
type fakeGitHub struct {
	server    *httptest.Server
	hits      atomic.Int64
	lastAuth  atomic.Value // string
	lastPath  atomic.Value // string
	tokenTTL  time.Duration
	status    int
	tokenName string
}

func newFakeGitHub(t *testing.T, tokenName string, tokenTTL time.Duration, status int) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{tokenTTL: tokenTTL, status: status, tokenName: tokenName}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		f.lastAuth.Store(r.Header.Get("Authorization"))
		f.lastPath.Store(r.URL.Path)

		if f.status != http.StatusCreated {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(`{"message":"nope"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":      fmt.Sprintf("%s-%d", f.tokenName, f.hits.Load()),
			"expires_at": time.Now().Add(f.tokenTTL).UTC().Format(time.RFC3339),
		})
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGitHub) auth() string {
	value, _ := f.lastAuth.Load().(string)
	return value
}

func (f *fakeGitHub) path() string {
	value, _ := f.lastPath.Load().(string)
	return value
}

func TestTokenFetchesAndSignsJWT(t *testing.T) {
	key, keyPEM := newTestKey(t)
	fake := newFakeGitHub(t, "ghs_first", time.Hour, http.StatusCreated)

	app := New(testAppID, testInstallationID, keyPEM, fake.server.URL, fake.server.Client())

	token, err := app.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() returned error: %v", err)
	}
	if token != "ghs_first-1" {
		t.Errorf("Token() = %q, want %q", token, "ghs_first-1")
	}
	if fake.hits.Load() != 1 {
		t.Errorf("endpoint hits = %d, want 1", fake.hits.Load())
	}

	wantPath := "/app/installations/" + testInstallationID + "/access_tokens"
	if got := fake.path(); got != wantPath {
		t.Errorf("request path = %q, want %q", got, wantPath)
	}

	auth := fake.auth()
	if !strings.HasPrefix(auth, "Bearer ") {
		t.Fatalf("Authorization = %q, want a Bearer JWT", auth)
	}

	claims := jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(strings.TrimPrefix(auth, "Bearer "), &claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
				return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
			}
			return &key.PublicKey, nil
		})
	if err != nil {
		t.Fatalf("JWT does not verify with the test public key: %v", err)
	}
	if !parsed.Valid {
		t.Error("JWT is not valid")
	}
	if claims.Issuer != testAppID {
		t.Errorf("JWT iss = %q, want %q", claims.Issuer, testAppID)
	}

	now := time.Now()
	if iat := claims.IssuedAt.Time; iat.After(now.Add(-30*time.Second)) || iat.Before(now.Add(-5*time.Minute)) {
		t.Errorf("JWT iat = %v, want ~60s in the past (now %v)", iat, now)
	}
	if exp := claims.ExpiresAt.Time; exp.Before(now.Add(9*time.Minute)) || exp.After(now.Add(11*time.Minute)) {
		t.Errorf("JWT exp = %v, want ~10m ahead (now %v)", exp, now)
	}
}

func TestTokenServedFromCache(t *testing.T) {
	_, keyPEM := newTestKey(t)
	fake := newFakeGitHub(t, "ghs_cached", time.Hour, http.StatusCreated)

	app := New(testAppID, testInstallationID, keyPEM, fake.server.URL, fake.server.Client())

	first, err := app.Token(context.Background())
	if err != nil {
		t.Fatalf("first Token() returned error: %v", err)
	}
	for range 5 {
		again, err := app.Token(context.Background())
		if err != nil {
			t.Fatalf("cached Token() returned error: %v", err)
		}
		if again != first {
			t.Fatalf("cached Token() = %q, want %q", again, first)
		}
	}
	if fake.hits.Load() != 1 {
		t.Errorf("endpoint hits = %d, want 1 (token should be cached)", fake.hits.Load())
	}
}

func TestTokenRefreshesNearExpiry(t *testing.T) {
	_, keyPEM := newTestKey(t)
	// A token that expires in 2 minutes is inside the 5-minute refresh window,
	// so every call must go back to GitHub.
	fake := newFakeGitHub(t, "ghs_short", 2*time.Minute, http.StatusCreated)

	app := New(testAppID, testInstallationID, keyPEM, fake.server.URL, fake.server.Client())

	first, err := app.Token(context.Background())
	if err != nil {
		t.Fatalf("first Token() returned error: %v", err)
	}
	second, err := app.Token(context.Background())
	if err != nil {
		t.Fatalf("second Token() returned error: %v", err)
	}

	if fake.hits.Load() != 2 {
		t.Errorf("endpoint hits = %d, want 2 (near-expiry token must be refreshed)", fake.hits.Load())
	}
	if first == second {
		t.Errorf("Token() returned the stale token %q after refresh", first)
	}
}

func TestTokenErrorOnNon201(t *testing.T) {
	_, keyPEM := newTestKey(t)

	for _, status := range []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			fake := newFakeGitHub(t, "ghs", time.Hour, status)
			app := New(testAppID, testInstallationID, keyPEM, fake.server.URL, fake.server.Client())

			if _, err := app.Token(context.Background()); err == nil {
				t.Fatalf("Token() with status %d returned no error", status)
			}
		})
	}
}

func TestTokenErrorOnInvalidPEM(t *testing.T) {
	fake := newFakeGitHub(t, "ghs", time.Hour, http.StatusCreated)
	app := New(testAppID, testInstallationID, []byte("not a pem file"), fake.server.URL, fake.server.Client())

	if _, err := app.Token(context.Background()); err == nil {
		t.Fatal("Token() with an invalid private key returned no error")
	}
	if fake.hits.Load() != 0 {
		t.Errorf("endpoint hits = %d, want 0 (should fail before calling GitHub)", fake.hits.Load())
	}
}

func TestTokenHonoursContextCancellation(t *testing.T) {
	_, keyPEM := newTestKey(t)
	fake := newFakeGitHub(t, "ghs", time.Hour, http.StatusCreated)
	app := New(testAppID, testInstallationID, keyPEM, fake.server.URL, fake.server.Client())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := app.Token(ctx); err == nil {
		t.Fatal("Token() with a cancelled context returned no error")
	}
}
