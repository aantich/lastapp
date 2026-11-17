package control

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

// Console session, password and OIDC login, CSRF/origin and agent OAuth
// contracts: error shapes and the names/attributes (never values) of cookies.
func TestSessionShapeGolden(t *testing.T) {
	s, c, _, owner, _ := foundationFixture(t)
	rec := newShapeRecorder(t, c.h)
	send := func(label, method, path string, body any, status int, setup func(*http.Request)) *httptest.ResponseRecorder {
		t.Helper()
		var b []byte
		if body != nil {
			b, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, s.Config.PublicURL+path, bytes.NewReader(b))
		if body != nil {
			r.Header.Set("Content-Type", "application/json")
		}
		if setup != nil {
			setup(r)
		}
		return rec.raw(label, r, status)
	}
	signedIn := func(session, csrf string) func(*http.Request) {
		return func(r *http.Request) {
			r.Header.Set("Origin", s.Config.PublicURL)
			r.AddCookie(&http.Cookie{Name: httpapi.CookieName(s.Config.PublicURL, "lastapp_session"), Value: session})
			if csrf != "" {
				r.Header.Set("X-CSRF-Token", csrf)
			}
		}
	}
	oc := c.as(owner)

	// Public configuration and console authentication.
	send("GET /v1/config", "GET", "/v1/config", nil, 200, nil)
	send("GET /v1/me without session", "GET", "/v1/me", nil, 401, nil)
	send("GET /v1/me unknown session", "GET", "/v1/me", nil, 401, signedIn(registry.Token(32), ""))
	send("GET /v1/me", "GET", "/v1/me", nil, 200, signedIn(oc.session, ""))
	patch := registry.Row{"display_name": "Session golden"}
	send("POST /auth/logout foreign origin", "POST", "/auth/logout", nil, 403, func(r *http.Request) {
		signedIn(oc.session, oc.csrf)(r)
		r.Header.Set("Origin", "https://untrusted.example")
	})
	send("POST /auth/logout without CSRF", "POST", "/auth/logout", nil, 403, signedIn(oc.session, ""))
	send("POST /auth/logout invalid CSRF", "POST", "/auth/logout", nil, 403, signedIn(oc.session, registry.Token(32)))
	send("POST /admin/v1/users without session", "POST", "/admin/v1/users", patch, 401, func(r *http.Request) { r.Header.Set("Origin", s.Config.PublicURL) })

	// Local password accounts.
	email, password := "golden-"+registry.NewID()[:8]+"@example.test", "Golden session 8!"
	rec.http(c, "POST /admin/v1/users", "POST", "/admin/v1/users", registry.Row{"display_name": "Golden local", "email": email, "password": password}, 0, 201)
	login := func(label string, body any, origin string, status int) *httptest.ResponseRecorder {
		t.Helper()
		return send(label, "POST", "/auth/login/password", body, status, func(r *http.Request) {
			if origin != "" {
				r.Header.Set("Origin", origin)
			}
		})
	}
	login("POST /auth/login/password foreign origin", registry.Row{"email": email, "password": password}, "https://untrusted.example", 403)
	login("POST /auth/login/password malformed", registry.Row{"user": email}, s.Config.PublicURL, 400)
	login("POST /auth/login/password wrong password", registry.Row{"email": email, "password": "Wrong password 8!"}, s.Config.PublicURL, 401)
	w := login("POST /auth/login/password", registry.Row{"email": email, "password": password}, s.Config.PublicURL, 200)
	var session, csrf string
	for _, cookie := range w.Result().Cookies() {
		switch cookie.Name {
		case httpapi.CookieName(s.Config.PublicURL, "lastapp_session"):
			session = cookie.Value
		case httpapi.CookieName(s.Config.PublicURL, "lastapp_csrf"):
			csrf = cookie.Value
		}
	}
	send("GET /v1/me password change required", "GET", "/v1/me", nil, 200, signedIn(session, ""))
	send("GET /v1/my-tenants password change required", "GET", "/v1/my-tenants", nil, 403, signedIn(session, ""))
	send("POST /auth/password wrong current", "POST", "/auth/password", registry.Row{"current_password": "Wrong password 8!", "new_password": "Golden changed 9!"}, 401, signedIn(session, csrf))
	send("POST /auth/password weak", "POST", "/auth/password", registry.Row{"current_password": password, "new_password": "short"}, 400, signedIn(session, csrf))
	w = send("POST /auth/password", "POST", "/auth/password", registry.Row{"current_password": password, "new_password": "Golden changed 9!"}, 200, signedIn(session, csrf))
	for _, cookie := range w.Result().Cookies() {
		switch cookie.Name {
		case httpapi.CookieName(s.Config.PublicURL, "lastapp_session"):
			session = cookie.Value
		case httpapi.CookieName(s.Config.PublicURL, "lastapp_csrf"):
			csrf = cookie.Value
		}
	}
	send("POST /auth/logout", "POST", "/auth/logout", nil, 200, signedIn(session, csrf))
	send("GET /v1/me after logout", "GET", "/v1/me", nil, 401, signedIn(session, ""))
	limited := "limited-" + registry.NewID()[:8] + "@example.test"
	for range 8 {
		s.loginLimiter.AllowRate("email:"+hex.EncodeToString(hash(limited)), time.Now(), 8, 1.0/30)
	}
	login("POST /auth/login/password rate limited", registry.Row{"email": limited, "password": password}, s.Config.PublicURL, 429)

	// OIDC without a provider, then against a provider whose token endpoint
	// rejects every code (no real identity provider is contacted).
	send("GET /auth/login without provider", "GET", "/auth/login", nil, 404, nil)
	send("GET /auth/callback without provider", "GET", "/auth/callback", nil, 404, nil)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer provider.Close()
	saved := s.auth
	s.auth = &oidcAuth{config: oauth2.Config{ClientID: "golden", Endpoint: oauth2.Endpoint{AuthURL: provider.URL + "/authorize", TokenURL: provider.URL + "/token"}, RedirectURL: s.Config.PublicURL + "/auth/callback", Scopes: []string{"openid"}}}
	defer func() { s.auth = saved }()
	send("GET /auth/login invalid return origin", "GET", "/auth/login?return_origin="+url.QueryEscape("https://untrusted.example"), nil, 400, nil)
	start := send("GET /auth/login", "GET", "/auth/login", nil, 302, nil)
	var flow *http.Cookie
	for _, cookie := range start.Result().Cookies() {
		if cookie.Name == httpapi.CookieName(s.Config.PublicURL, "lastapp_flow") {
			flow = cookie
		}
	}
	location, _ := url.Parse(start.Header().Get("Location"))
	state := location.Query().Get("state")
	send("GET /auth/callback without flow", "GET", "/auth/callback?state="+state+"&code=x", nil, 400, nil)
	send("GET /auth/callback wrong state", "GET", "/auth/callback?state=wrong&code=x", nil, 400, func(r *http.Request) { r.AddCookie(flow) })
	send("GET /auth/callback rejected code", "GET", "/auth/callback?state="+url.QueryEscape(state)+"&code=x", nil, 401, func(r *http.Request) { r.AddCookie(flow) })

	// Agent OAuth: discovery, registration, authorization, consent, tokens.
	form := func(label, path string, values url.Values, status int) registry.Row {
		t.Helper()
		r := httptest.NewRequest("POST", s.Config.PublicURL+path, strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := rec.raw(label, r, status)
		out := registry.Row{}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	send("POST /mcp without token", "POST", "/mcp", registry.Row{}, 401, nil)
	send("POST /mcp invalid token", "POST", "/mcp", registry.Row{}, 401, func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+registry.Token(32)) })
	send("GET /.well-known/oauth-protected-resource/mcp", "GET", "/.well-known/oauth-protected-resource/mcp", nil, 200, nil)
	send("GET /.well-known/oauth-authorization-server", "GET", "/.well-known/oauth-authorization-server", nil, 200, nil)
	redirect := "http://localhost:43188/callback"
	send("POST /oauth/register unsafe redirect", "POST", "/oauth/register", registry.Row{"client_name": "Golden", "redirect_uris": []string{"http://evil.example/callback"}, "token_endpoint_auth_method": "none"}, 400, nil)
	registered := send("POST /oauth/register", "POST", "/oauth/register", registry.Row{"client_name": "Golden client", "redirect_uris": []string{redirect}, "token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code", "refresh_token"}}, 201, nil)
	var client registry.Row
	_ = json.Unmarshal(registered.Body.Bytes(), &client)
	verifier := registry.Token(48)
	sum := sha256.Sum256([]byte(verifier))
	parameters := url.Values{"response_type": {"code"}, "client_id": {registry.Str(client["client_id"])}, "redirect_uri": {redirect}, "resource": {s.mcpURL()}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "state": {"golden-state"}}
	authorize := func(label string, change func(url.Values), status int) *httptest.ResponseRecorder {
		t.Helper()
		v := url.Values{}
		for k, list := range parameters {
			v[k] = list
		}
		if change != nil {
			change(v)
		}
		return send(label, "GET", "/oauth/authorize?"+v.Encode(), nil, status, nil)
	}
	authorize("GET /oauth/authorize unregistered redirect", func(v url.Values) { v.Set("redirect_uri", "http://localhost:43188/other") }, 400)
	authorize("GET /oauth/authorize without PKCE", func(v url.Values) { v.Del("code_challenge_method") }, 400)
	authorize("GET /oauth/authorize unknown client", func(v url.Values) { v.Set("client_id", registry.NewID()) }, 400)
	consent, _ := url.Parse(authorize("GET /oauth/authorize", nil, 302).Header().Get("Location"))
	requestID := consent.Query().Get("request")
	rec.http(c, "GET /v1/agent-access/requests/{request}", "GET", "/v1/agent-access/requests/"+requestID, nil, 0, 200)
	rec.http(oc, "GET /v1/agent-access/requests/{request} missing", "GET", "/v1/agent-access/requests/"+registry.Token(32), nil, 0, 404)
	rec.http(oc, "POST /v1/agent-access/requests/{request} invalid scope", "POST", "/v1/agent-access/requests/"+requestID, registry.Row{"scope_kind": "platform", "permissions": []string{"read"}, "days": 1}, 0, 400)
	approved := rec.http(c, "POST /v1/agent-access/requests/{request}", "POST", "/v1/agent-access/requests/"+requestID, registry.Row{"scope_kind": "platform", "permissions": []string{"read"}, "days": 1}, 0, 200)
	callback, _ := url.Parse(registry.Str(approved["redirect_url"]))
	exchange := url.Values{"grant_type": {"authorization_code"}, "client_id": {registry.Str(client["client_id"])}, "redirect_uri": {redirect}, "resource": {s.mcpURL()}, "code": {callback.Query().Get("code")}, "code_verifier": {registry.Token(48)}}
	form("POST /oauth/token unsupported grant", "/oauth/token", url.Values{"grant_type": {"password"}}, 400)
	form("POST /oauth/token wrong verifier", "/oauth/token", exchange, 400)
	exchange.Set("code_verifier", verifier)
	tokens := form("POST /oauth/token", "/oauth/token", exchange, 200)
	form("POST /oauth/token code replay", "/oauth/token", exchange, 400)
	refresh := url.Values{"grant_type": {"refresh_token"}, "client_id": {registry.Str(client["client_id"])}, "refresh_token": {registry.Str(tokens["refresh_token"])}, "resource": {s.mcpURL()}}
	refreshed := form("POST /oauth/token refresh", "/oauth/token", refresh, 200)
	form("POST /oauth/token refresh replay", "/oauth/token", refresh, 400)
	grants := rec.http(c, "GET /v1/agent-access", "GET", "/v1/agent-access", nil, 0, 200)
	rec.http(oc, "GET /v1/agent-access/config", "GET", "/v1/agent-access/config", nil, 0, 200)
	form("POST /oauth/revoke", "/oauth/revoke", url.Values{"client_id": {registry.Str(client["client_id"])}, "token": {registry.Str(refreshed["access_token"])}}, 200)
	send("POST /mcp revoked token", "POST", "/mcp", registry.Row{}, 401, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+registry.Str(refreshed["access_token"]))
	})
	grant := grants["items"].([]any)[0].(map[string]any)
	rec.http(c, "PATCH /v1/agent-access/{grant} stale", "PATCH", "/v1/agent-access/"+registry.Str(grant["id"]), registry.Row{"state": "revoked"}, registry.Integer(grant["revision"])+5, 412)
	rec.http(c, "PATCH /v1/agent-access/{grant}", "PATCH", "/v1/agent-access/"+registry.Str(grant["id"]), registry.Row{"state": "revoked"}, registry.Integer(grant["revision"]), 200)
	rec.golden("session_shapes.golden.txt")
}
