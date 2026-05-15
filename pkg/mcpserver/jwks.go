package mcpserver

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"encoding/base64"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

// JWKSVerifierConfig configures the JWKS-based token verifier.
type JWKSVerifierConfig struct {
	// IssuerURL is the base URL of the Authorization Server
	// (e.g. "https://my-tenant.auth0.com"). The JWKS endpoint is discovered
	// at {IssuerURL}/.well-known/jwks.json.
	IssuerURL string

	// Audience is the expected "aud" claim in the JWT. Optional.
	// When empty, the audience claim is not validated.
	Audience string

	// HTTPClient is the HTTP client used to fetch the JWKS. Optional.
	// Defaults to a client with a 10-second timeout.
	HTTPClient *http.Client

	// CacheTTL is how long the JWKS is cached before being refreshed.
	// Defaults to 1 hour.
	CacheTTL time.Duration
}

// NewJWKSTokenVerifier returns an auth.TokenVerifier that validates JWT tokens
// by fetching the Authorization Server's public keys from its JWKS endpoint.
//
// The verifier:
//   - Fetches and caches JWKS keys from {IssuerURL}/.well-known/jwks.json
//   - Validates the JWT signature (RS256)
//   - Validates exp, iss, and optionally aud claims
//   - Extracts scopes from the "scope" claim (space-separated, per RFC 8693)
//   - Extracts user ID from the "sub" claim
func NewJWKSTokenVerifier(cfg JWKSVerifierConfig) auth.TokenVerifier {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 1 * time.Hour
	}

	cache := &jwksCache{
		jwksURL:    strings.TrimRight(cfg.IssuerURL, "/") + "/.well-known/jwks.json",
		httpClient: cfg.HTTPClient,
		ttl:        cfg.CacheTTL,
	}

	// Auth0 always uses a trailing slash in the iss claim. Normalize to
	// include the trailing slash so the parser matches the actual token.
	issuer := strings.TrimRight(cfg.IssuerURL, "/") + "/"

	parserOpts := []jwt.ParserOption{
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}),
	}
	if cfg.Audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(cfg.Audience))
	}

	parser := jwt.NewParser(parserOpts...)

	return func(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		keyFunc := func(t *jwt.Token) (any, error) {
			kid, ok := t.Header["kid"].(string)
			if !ok {
				return nil, fmt.Errorf("%w: missing kid header", auth.ErrInvalidToken)
			}
			key, err := cache.getKey(ctx, kid)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", auth.ErrInvalidToken, err)
			}
			return key, nil
		}

		var claims tokenClaims
		parsed, err := parser.ParseWithClaims(token, &claims, keyFunc)
		if err != nil {
			slog.Debug("jwks verifier: token parse failed", "error", err)
			return nil, fmt.Errorf("%w: %v", auth.ErrInvalidToken, err)
		}
		if !parsed.Valid {
			slog.Debug("jwks verifier: token not valid after parsing")
			return nil, fmt.Errorf("%w: token is not valid", auth.ErrInvalidToken)
		}

		slog.Debug("jwks verifier: token verified", "sub", claims.Subject, "iss", claims.Issuer, "aud", claims.Audience, "scope", claims.Scope)
		info := &auth.TokenInfo{
			UserID: claims.Subject,
		}
		if claims.ExpiresAt != nil {
			info.Expiration = claims.ExpiresAt.Time
		}
		if claims.Scope != "" {
			info.Scopes = strings.Split(claims.Scope, " ")
		}
		return info, nil
	}
}

// tokenClaims extends RegisteredClaims with the OAuth "scope" claim.
type tokenClaims struct {
	jwt.RegisteredClaims
	Scope string `json:"scope"`
}

// jwksCache fetches and caches the JWKS key set from the Authorization Server.
type jwksCache struct {
	jwksURL    string
	httpClient *http.Client
	ttl        time.Duration

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

func (c *jwksCache) getKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	if time.Since(c.fetchedAt) < c.ttl {
		if key, ok := c.keys[kid]; ok {
			c.mu.RUnlock()
			return key, nil
		}
	}
	c.mu.RUnlock()

	// Cache miss or expired — refresh.
	if err := c.refresh(ctx); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	key, ok := c.keys[kid]
	if !ok {
		return nil, fmt.Errorf("key %q not found in JWKS", kid)
	}
	return key, nil
}

func (c *jwksCache) refresh(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: another goroutine may have refreshed while we waited.
	if time.Since(c.fetchedAt) < c.ttl && len(c.keys) > 0 {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("creating JWKS request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decoding JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		pub, err := parseRSAPublicKey(k)
		if err != nil {
			continue // skip malformed keys
		}
		keys[k.Kid] = pub
	}

	if len(keys) == 0 {
		return errors.New("JWKS contains no usable RSA keys")
	}

	c.keys = keys
	c.fetchedAt = time.Now()
	return nil
}

// jwksResponse is the JSON structure returned by /.well-known/jwks.json.
type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func parseRSAPublicKey(k jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decoding modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decoding exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}
