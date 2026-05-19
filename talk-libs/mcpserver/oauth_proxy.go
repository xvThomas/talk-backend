package mcpserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// asProxyMeta is the minimal RFC 8414 Authorization Server Metadata document
// served at /.well-known/oauth-authorization-server when AS proxy is enabled.
type asProxyMeta struct {
	Issuer                        string   `json:"issuer"`
	AuthorizationEndpoint         string   `json:"authorization_endpoint"`
	TokenEndpoint                 string   `json:"token_endpoint"`
	ResponseTypesSupported        []string `json:"response_types_supported"`
	GrantTypesSupported           []string `json:"grant_types_supported,omitempty"`
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported,omitempty"`
}

// registerASProxy registers the three OAuth Authorization Server proxy endpoints
// on mux:
//
//   - GET  /.well-known/oauth-authorization-server — RFC 8414 AS metadata
//   - GET  /authorize                              — injects audience, redirects upstream
//   - POST /token                                  — transparent proxy to upstream token endpoint
//
// This allows OAuth clients that do not send an audience parameter (e.g.
// Claude.ai) to work with Auth0, which requires audience to issue a JWT.
func registerASProxy(mux *http.ServeMux, baseURL string, cfg *OAuthConfig) {
	proxy := cfg.ASProxy

	upstreamAuthorize := proxy.UpstreamAuthorizeURL
	if upstreamAuthorize == "" {
		upstreamAuthorize = strings.TrimRight(cfg.AuthorizationServerURL, "/") + "/authorize"
	}
	upstreamToken := proxy.UpstreamTokenURL
	if upstreamToken == "" {
		upstreamToken = strings.TrimRight(cfg.AuthorizationServerURL, "/") + "/oauth/token"
	}

	// Pre-marshal the AS metadata so it can be served cheaply on every request.
	meta := asProxyMeta{
		Issuer:                        baseURL,
		AuthorizationEndpoint:         baseURL + "/authorize",
		TokenEndpoint:                 baseURL + "/token",
		ResponseTypesSupported:        []string{"code"},
		GrantTypesSupported:           []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported: []string{"S256"},
	}
	metaJSON, _ := json.Marshal(meta)

	// /.well-known/oauth-authorization-server — RFC 8414 AS metadata.
	// OAuth clients discover this after reading the protected resource metadata.
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(metaJSON)
	})

	// /authorize — adds the audience parameter then redirects to the upstream AS.
	// Auth0 requires audience to issue a JWT access token instead of an opaque token.
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()
		if proxy.Audience != "" {
			params.Set("audience", proxy.Audience)
		}
		// Ensure offline_access is requested so Auth0 returns a refresh_token.
		scope := params.Get("scope")
		if !strings.Contains(scope, "offline_access") {
			if scope == "" {
				scope = "offline_access"
			} else {
				scope = scope + " offline_access"
			}
			params.Set("scope", scope)
		}
		slog.Debug("oauth proxy: /authorize redirect", "audience", proxy.Audience, "client_id", params.Get("client_id"), "scope", params.Get("scope"))
		target, err := url.Parse(upstreamAuthorize)
		if err != nil {
			slog.Debug("oauth proxy: /authorize parse error", "error", err)
			http.Error(w, "proxy misconfigured", http.StatusInternalServerError)
			return
		}
		target.RawQuery = params.Encode()
		http.Redirect(w, r, target.String(), http.StatusFound)
	})

	// /token — transparent proxy to the upstream token endpoint.
	// Optionally injects client_secret for confidential clients.
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			slog.Debug("oauth proxy: /token parse form error", "error", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		form := make(url.Values)
		for k, vs := range r.Form {
			form[k] = vs
		}
		slog.Debug("oauth proxy: /token request", "grant_type", form.Get("grant_type"), "client_id", form.Get("client_id"), "redirect_uri", form.Get("redirect_uri"))
		// Inject client_secret if configured and not already present in the request.
		if proxy.ClientSecret != "" && form.Get("client_secret") == "" {
			form.Set("client_secret", proxy.ClientSecret)
		}

		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamToken,
			strings.NewReader(form.Encode()))
		if err != nil {
			slog.Debug("oauth proxy: /token request build error", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// Forward Basic auth if the client sent it (confidential client auth).
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Debug("oauth proxy: /token upstream error", "error", err, "upstream", upstreamToken)
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Read body to log in debug mode if upstream returned an error.
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			slog.Debug("oauth proxy: /token upstream error response", "status", resp.StatusCode, "body", string(body))
		} else {
			// Log token response keys (without leaking actual tokens).
			var keys []string
			var respMap map[string]any
			if json.Unmarshal(body, &respMap) == nil {
				for k := range respMap {
					keys = append(keys, k)
				}
			}
			slog.Debug("oauth proxy: /token upstream success", "status", resp.StatusCode, "response_keys", keys)
		}

		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
	})
}
