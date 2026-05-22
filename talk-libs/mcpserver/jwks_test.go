package mcpserver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// testRSAKey generates a deterministic-ish RSA key pair for tests.
func testRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	return key
}

// serveJWKS starts an httptest.Server that serves a JWKS endpoint with the given key.
func serveJWKS(t *testing.T, kid string, pub *rsa.PublicKey) *httptest.Server {
	t.Helper()

	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())

	jwks := map[string]any{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"kid": kid,
				"use": "sig",
				"n":   n,
				"e":   e,
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func signToken(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return signed
}

func TestParseRSAPublicKey_Valid(t *testing.T) {
	key := testRSAKey(t)
	pub := key.Public().(*rsa.PublicKey)

	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())

	parsed, err := parseRSAPublicKey(jwkKey{
		Kty: "RSA",
		Kid: "test-kid",
		N:   n,
		E:   e,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.N.Cmp(pub.N) != 0 {
		t.Error("parsed modulus does not match")
	}
	if parsed.E != pub.E {
		t.Errorf("parsed exponent %d != expected %d", parsed.E, pub.E)
	}
}

func TestParseRSAPublicKey_InvalidModulus(t *testing.T) {
	_, err := parseRSAPublicKey(jwkKey{
		Kty: "RSA",
		Kid: "test-kid",
		N:   "!!!invalid-base64",
		E:   "AQAB",
	})
	if err == nil {
		t.Error("expected error for invalid modulus")
	}
}

func TestParseRSAPublicKey_InvalidExponent(t *testing.T) {
	key := testRSAKey(t)
	pub := key.Public().(*rsa.PublicKey)
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())

	_, err := parseRSAPublicKey(jwkKey{
		Kty: "RSA",
		Kid: "test-kid",
		N:   n,
		E:   "!!!invalid",
	})
	if err == nil {
		t.Error("expected error for invalid exponent")
	}
}

func TestJWKSVerifier_ValidToken(t *testing.T) {
	key := testRSAKey(t)
	pub := key.Public().(*rsa.PublicKey)
	kid := "key-1"

	srv := serveJWKS(t, kid, pub)

	verifier := NewJWKSTokenVerifier(JWKSVerifierConfig{
		IssuerURL:  srv.URL,
		Audience:   "my-api",
		HTTPClient: srv.Client(),
		CacheTTL:   1 * time.Minute,
	})

	token := signToken(t, key, kid, jwt.MapClaims{
		"iss":   srv.URL + "/",
		"aud":   []string{"my-api"},
		"sub":   "user-123",
		"scope": "read write",
		"exp":   time.Now().Add(1 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})

	info, err := verifier(context.Background(), token, httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UserID != "user-123" {
		t.Errorf("expected UserID %q, got %q", "user-123", info.UserID)
	}
	if len(info.Scopes) != 2 || info.Scopes[0] != "read" || info.Scopes[1] != "write" {
		t.Errorf("expected scopes [read write], got %v", info.Scopes)
	}
}

func TestJWKSVerifier_ExpiredToken(t *testing.T) {
	key := testRSAKey(t)
	pub := key.Public().(*rsa.PublicKey)
	kid := "key-1"

	srv := serveJWKS(t, kid, pub)

	verifier := NewJWKSTokenVerifier(JWKSVerifierConfig{
		IssuerURL:  srv.URL,
		HTTPClient: srv.Client(),
		CacheTTL:   1 * time.Minute,
	})

	token := signToken(t, key, kid, jwt.MapClaims{
		"iss": srv.URL + "/",
		"sub": "user-123",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	})

	_, err := verifier(context.Background(), token, httptest.NewRequest(http.MethodGet, "/", nil))
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestJWKSVerifier_WrongAudience(t *testing.T) {
	key := testRSAKey(t)
	pub := key.Public().(*rsa.PublicKey)
	kid := "key-1"

	srv := serveJWKS(t, kid, pub)

	verifier := NewJWKSTokenVerifier(JWKSVerifierConfig{
		IssuerURL:  srv.URL,
		Audience:   "expected-api",
		HTTPClient: srv.Client(),
		CacheTTL:   1 * time.Minute,
	})

	token := signToken(t, key, kid, jwt.MapClaims{
		"iss": srv.URL + "/",
		"aud": []string{"wrong-api"},
		"sub": "user-123",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})

	_, err := verifier(context.Background(), token, httptest.NewRequest(http.MethodGet, "/", nil))
	if err == nil {
		t.Error("expected error for wrong audience")
	}
}

func TestJWKSVerifier_UnknownKid(t *testing.T) {
	key := testRSAKey(t)
	pub := key.Public().(*rsa.PublicKey)

	srv := serveJWKS(t, "known-kid", pub)

	verifier := NewJWKSTokenVerifier(JWKSVerifierConfig{
		IssuerURL:  srv.URL,
		HTTPClient: srv.Client(),
		CacheTTL:   1 * time.Minute,
	})

	token := signToken(t, key, "unknown-kid", jwt.MapClaims{
		"iss": srv.URL + "/",
		"sub": "user-123",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})

	_, err := verifier(context.Background(), token, httptest.NewRequest(http.MethodGet, "/", nil))
	if err == nil {
		t.Error("expected error for unknown kid")
	}
}

func TestJWKSCache_Refresh(t *testing.T) {
	key := testRSAKey(t)
	pub := key.Public().(*rsa.PublicKey)
	kid := "cache-test"

	srv := serveJWKS(t, kid, pub)

	cache := &jwksCache{
		jwksURL:    srv.URL + "/.well-known/jwks.json",
		httpClient: srv.Client(),
		ttl:        1 * time.Minute,
	}

	got, err := cache.getKey(context.Background(), kid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.N.Cmp(pub.N) != 0 {
		t.Error("cached key modulus does not match")
	}
}
