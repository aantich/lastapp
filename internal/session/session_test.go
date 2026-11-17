package session

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/config"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/registry"
	"github.com/aantich/lastapp/internal/sealer"
)

// manager builds a database-free Manager. Every path exercised here returns
// before the registry is queried; a query would panic on the nil DB.
func manager(t *testing.T, publicURL string) *Manager {
	t.Helper()
	seal, e := sealer.New(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), "")
	if e != nil {
		t.Fatal(e)
	}
	return &Manager{Config: &config.Config{PublicURL: publicURL}, Sealer: seal, Passwords: membership.NewPasswords(2), Proxies: &httpapi.TrustedProxies{}}
}

func cookies(w *httptest.ResponseRecorder) map[string]*http.Cookie {
	out := map[string]*http.Cookie{}
	for _, c := range w.Result().Cookies() {
		out[c.Name] = c
	}
	return out
}

func problemCode(t *testing.T, e error) (int, string) {
	t.Helper()
	var p *registry.APIError
	if !errors.As(e, &p) {
		t.Fatalf("not an API problem: %v", e)
	}
	return p.Status, p.Code
}

func TestSessionCookieAttributes(t *testing.T) {
	for _, tc := range []struct {
		publicURL, prefix string
		secure            bool
	}{
		{"https://admin.example", "__Host-", true},
		{"http://127.0.0.1:8787", "", false},
	} {
		t.Run(tc.publicURL, func(t *testing.T) {
			m := manager(t, tc.publicURL)
			w := httptest.NewRecorder()
			m.setSessionCookies(w, "session-token", "csrf-token")
			got := cookies(w)
			session, csrf := got[tc.prefix+httpapi.SessionCookie], got[tc.prefix+httpapi.CSRFCookie]
			if session == nil || csrf == nil || len(got) != 2 {
				t.Fatal("session cookies", got)
			}
			for _, c := range []*http.Cookie{session, csrf} {
				if c.Path != "/" || c.MaxAge != 8*3600 || c.Secure != tc.secure || c.SameSite != http.SameSiteLaxMode || c.Domain != "" {
					t.Fatalf("cookie %s attributes: %+v", c.Name, c)
				}
			}
			// The console reads the CSRF cookie; the session cookie stays HttpOnly.
			if !session.HttpOnly || csrf.HttpOnly || session.Value != "session-token" || csrf.Value != "csrf-token" {
				t.Fatalf("cookie values or HttpOnly: %+v %+v", session, csrf)
			}
			w = httptest.NewRecorder()
			m.cookie(w, flowCookie, "", -1, true)
			if c := cookies(w)[tc.prefix+flowCookie]; c == nil || c.MaxAge >= 0 || !c.HttpOnly {
				t.Fatalf("flow cookie is not cleared: %+v", c)
			}
		})
	}
}

func TestCSRFChecksOriginBeforeSession(t *testing.T) {
	m := manager(t, "https://admin.example")
	request := func(origin, cookie, token string) *http.Request {
		r := httptest.NewRequest("POST", "https://admin.example/v1/x", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: httpapi.CookieName(m.Config.PublicURL, httpapi.SessionCookie), Value: cookie})
		}
		if token != "" {
			r.Header.Set("X-CSRF-Token", token)
		}
		return r
	}
	for _, tc := range []struct {
		name                  string
		origin, cookie, token string
		status                int
		code                  string
	}{
		{"missing origin", "", "s", "c", 403, "invalid_origin"},
		{"foreign origin", "https://app.example", "s", "c", 403, "invalid_origin"},
		{"origin with path", "https://admin.example/", "s", "c", 403, "invalid_origin"},
		{"no session cookie", "https://admin.example", "", "c", 401, "unauthenticated"},
		{"no CSRF token", "https://admin.example", "s", "", 403, "csrf_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, code := problemCode(t, m.CSRF(context.Background(), request(tc.origin, tc.cookie, tc.token)))
			if status != tc.status || code != tc.code {
				t.Fatalf("got %d %s", status, code)
			}
		})
	}
	// The development UI origin is accepted only in development and exactly.
	m.Config.Development, m.Config.DevelopmentUIOrigin = true, "http://127.0.0.1:5173"
	if status, code := problemCode(t, m.CSRF(context.Background(), request("http://127.0.0.1:5174", "s", ""))); status != 403 || code != "invalid_origin" {
		t.Fatal("inexact development origin accepted", status, code)
	}
	if status, code := problemCode(t, m.CSRF(context.Background(), request("http://127.0.0.1:5173", "s", ""))); status != 403 || code != "csrf_failed" {
		t.Fatal("development UI origin rejected", status, code)
	}
}

func TestPrincipalRequiresSessionCookie(t *testing.T) {
	m := manager(t, "https://admin.example")
	r := httptest.NewRequest("GET", "/v1/me", nil)
	// The unprefixed name is not the console cookie on an HTTPS public URL.
	r.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: "s"})
	if status, code := problemCode(t, func() error { _, e := m.Principal(context.Background(), r); return e }()); status != 401 || code != "unauthenticated" {
		t.Fatal(status, code)
	}
}

// passwordLogin posts a login that is rejected before the registry is read:
// an oversized password is refused after rate charging, without PBKDF2 work.
func passwordLogin(m *Manager, peer, forwarded, origin, email string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(registry.Row{"email": email, "password": strings.Repeat("x", membership.MaxPasswordInputBytes+1)})
	r := httptest.NewRequest("POST", "/auth/login/password", bytes.NewReader(b))
	r.RemoteAddr = peer
	r.Header.Set("Origin", origin)
	if forwarded != "" {
		r.Header.Set("X-Forwarded-For", forwarded)
	}
	w := httptest.NewRecorder()
	m.PasswordLogin(w, r)
	return w
}

func TestPasswordLoginLimiter(t *testing.T) {
	m := manager(t, "https://admin.example")
	var e error
	if *m.Proxies, e = httpapi.ParseTrustedProxies([]string{"10.0.0.1"}); e != nil {
		t.Fatal(e)
	}
	// Origin is checked before any budget is spent.
	if w := passwordLogin(m, "192.0.2.1:1", "", "https://evil.example", "a@example.test"); w.Code != 403 || len(w.Result().Cookies()) != 0 {
		t.Fatal("foreign origin", w.Code)
	}
	if m.LoginLimiter.Has("ip:192.0.2.1") {
		t.Fatal("a rejected origin spent the client budget")
	}
	// Each client has a burst of loginIPBurst, whatever email it tries.
	for i := 1; i <= loginIPBurst+1; i++ {
		want := 401
		if i > loginIPBurst {
			want = 429
		}
		w := passwordLogin(m, "192.0.2.1:1", "", m.Config.PublicURL, fmt.Sprintf("c%d@example.test", i))
		if w.Code != want {
			t.Fatalf("attempt %d: got %d want %d", i, w.Code, want)
		}
		if want == 429 && w.Header().Get("Retry-After") != "1" {
			t.Fatal("rate limited response lacks Retry-After")
		}
	}
	// A rejected client does not spend the email budget of the address it tried.
	if m.LoginLimiter.Has(LoginEmailKey(fmt.Sprintf("c%d@example.test", loginIPBurst+1))) {
		t.Fatal("client rejection charged the email budget")
	}
	// Through a trusted proxy each forwarded client has its own budget, but an
	// email has one shared budget of loginEmailBurst across clients.
	for i := 1; i <= loginEmailBurst+1; i++ {
		want := 401
		if i > loginEmailBurst {
			want = 429
		}
		if w := passwordLogin(m, "10.0.0.1:1", fmt.Sprintf("198.51.100.%d", i), m.Config.PublicURL, " Target@Example.test "); w.Code != want {
			t.Fatalf("email attempt %d: got %d want %d", i, w.Code, want)
		}
	}
	// The email key is the normalized login email.
	if !m.LoginLimiter.Has(LoginEmailKey("target@example.test")) {
		t.Fatal("email budget is not keyed by the normalized login email")
	}
	// An untrusted peer's forwarding header is ignored.
	if w := passwordLogin(m, "192.0.2.1:1", "198.51.100.200", m.Config.PublicURL, "other@example.test"); w.Code != 429 {
		t.Fatal("spoofed forwarding header evaded the peer budget", w.Code)
	}
}

func TestPasswordLoginBusy(t *testing.T) {
	m := manager(t, "https://admin.example")
	m.Passwords = membership.NewPasswords(1)
	release, e := m.Passwords.Work()
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	b, _ := json.Marshal(registry.Row{"email": "busy@example.test", "password": "Busy password 8!"})
	r := httptest.NewRequest("POST", "/auth/login/password", bytes.NewReader(b))
	r.Header.Set("Origin", m.Config.PublicURL)
	w := httptest.NewRecorder()
	m.PasswordLogin(w, r)
	if w.Code != 429 || !strings.Contains(w.Body.String(), "authentication_busy") {
		t.Fatal("password login did not use the shared derivation bound", w.Code, w.Body.String())
	}
}

// ChangePassword applies the authoritative password policy before any
// verifier work or registry access.
func TestChangePasswordAppliesPolicy(t *testing.T) {
	m := manager(t, "https://admin.example")
	local := access.Principal{ID: registry.NewID(), AuthMethod: "password"}
	change := func(p access.Principal, current, next string) error {
		b, _ := json.Marshal(registry.Row{"current_password": current, "new_password": next})
		r := httptest.NewRequest("POST", "/auth/password", bytes.NewReader(b))
		return m.ChangePassword(httptest.NewRecorder(), r, p)
	}
	if status, _ := problemCode(t, change(access.Principal{ID: local.ID, AuthMethod: "oidc"}, "Current pass 8!", "Next password 9!")); status != 400 {
		t.Fatal("single sign-on account changed a local password", status)
	}
	for _, n := range []int{membership.MinPasswordLength - 1, membership.MaxPasswordLength + 1} {
		e := change(local, "Current pass 8!", strings.Repeat("密", n))
		if status, _ := problemCode(t, e); status != 400 || !strings.Contains(e.Error(), membership.PasswordLengthHint()) {
			t.Fatalf("length %d outside the policy accepted or misreported: %v", n, e)
		}
	}
	if status, _ := problemCode(t, change(local, "Same password 8!", "Same password 8!")); status != 400 {
		t.Fatal("unchanged password accepted", status)
	}
	if status, code := problemCode(t, change(local, strings.Repeat("x", membership.MaxPasswordInputBytes+1), "Next password 9!")); status != 401 || code != "invalid_credentials" {
		t.Fatal("oversized current password", status, code)
	}
	m.Passwords = membership.NewPasswords(1)
	release, e := m.Passwords.Work()
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	if status, code := problemCode(t, change(local, "Current pass 8!", "Next password 9!")); status != 429 || code != "authentication_busy" {
		t.Fatal("password change did not use the shared derivation bound", status, code)
	}
}

// stubKeys accepts any signature; the tests exercise the flow and nonce
// checks around verification, not the signature algorithm.
type stubKeys struct{}

func (stubKeys) VerifySignature(_ context.Context, jwt string) ([]byte, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed token")
	}
	return base64.RawURLEncoding.DecodeString(parts[1])
}

// stubProvider is a token endpoint that answers every code with idToken (or
// rejects the exchange when idToken is "reject").
func stubProvider(t *testing.T, idToken *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if *idToken == "reject" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		body := registry.Row{"access_token": "a", "token_type": "Bearer", "expires_in": 60}
		if *idToken != "" {
			body["id_token"] = *idToken
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func idToken(issuer, audience, nonce string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, _ := json.Marshal(registry.Row{"iss": issuer, "aud": audience, "sub": "subject", "nonce": nonce, "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix()})
	return header + "." + base64.RawURLEncoding.EncodeToString(claims) + "." + base64.RawURLEncoding.EncodeToString([]byte("signature"))
}

func TestOIDCStateAndNonce(t *testing.T) {
	m := manager(t, "http://127.0.0.1:8787")
	m.Config.Development = true
	get := func(handler http.HandlerFunc, target string, flow *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", target, nil)
		if flow != nil {
			r.AddCookie(flow)
		}
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	if w := get(m.Login, "/auth/login", nil); w.Code != 404 {
		t.Fatal("login without a provider", w.Code)
	}
	if w := get(m.Callback, "/auth/callback", nil); w.Code != 404 {
		t.Fatal("callback without a provider", w.Code)
	}
	token := "reject"
	provider := stubProvider(t, &token)
	const issuer, client = "https://issuer.example", "lastapp-test"
	m.OIDC = &OIDC{
		Config:   oauth2.Config{ClientID: client, Endpoint: oauth2.Endpoint{AuthURL: provider.URL + "/authorize", TokenURL: provider.URL + "/token"}, RedirectURL: m.Config.PublicURL + "/auth/callback", Scopes: []string{oidc.ScopeOpenID}},
		Verifier: oidc.NewVerifier(issuer, stubKeys{}, &oidc.Config{ClientID: client}),
	}
	// The stub verifier accepts these tokens, so the nonce mismatch below is
	// rejected by the nonce check itself.
	if ident, e := m.OIDC.Verifier.Verify(context.Background(), idToken(issuer, client, "another nonce")); e != nil || ident.Nonce != "another nonce" {
		t.Fatal("stub identity token is not verifiable", e)
	}
	if w := get(m.Login, "/auth/login?return_origin="+url.QueryEscape("https://untrusted.example"), nil); w.Code != 400 || len(w.Result().Cookies()) != 0 {
		t.Fatal("untrusted return origin", w.Code)
	}
	start := get(m.Login, "/auth/login", nil)
	if start.Code != http.StatusFound {
		t.Fatal("login", start.Code)
	}
	flow := cookies(start)[flowCookie]
	if flow == nil || !flow.HttpOnly || flow.MaxAge != flowSeconds || flow.SameSite != http.SameSiteLaxMode {
		t.Fatalf("flow cookie %+v", flow)
	}
	location, _ := url.Parse(start.Header().Get("Location"))
	q := location.Query()
	state := q.Get("state")
	if state == "" || q.Get("nonce") == "" || q.Get("code_challenge_method") != "S256" || len(q.Get("code_challenge")) != 43 || q.Get("client_id") != client {
		t.Fatal("authorization request", location)
	}
	// The flow is sealed: state, nonce and PKCE verifier never appear in the cookie.
	var sealed loginFlow
	raw, _ := base64.RawURLEncoding.DecodeString(flow.Value)
	if strings.Contains(flow.Value, state) || m.Sealer.Unseal("oidc_flow", "browser", "", raw, &sealed) != nil || sealed.State != state || sealed.Nonce != q.Get("nonce") {
		t.Fatal("flow cookie is not the sealed flow")
	}
	reseal := func(f loginFlow) *http.Cookie {
		b, e := m.Sealer.Seal("oidc_flow", "browser", "", f)
		if e != nil {
			t.Fatal(e)
		}
		return &http.Cookie{Name: flowCookie, Value: base64.RawURLEncoding.EncodeToString(b)}
	}
	callback := func(state string, flow *http.Cookie) *httptest.ResponseRecorder {
		return get(m.Callback, "/auth/callback?code=c&state="+url.QueryEscape(state), flow)
	}
	expired, future := sealed, sealed
	expired.Issued = time.Now().Unix() - flowSeconds - 1
	future.Issued = time.Now().Unix() + 60
	for name, tc := range map[string]struct {
		state string
		flow  *http.Cookie
		want  int
	}{
		"missing flow":   {state, nil, 400},
		"wrong state":    {"wrong", flow, 400},
		"forged flow":    {state, &http.Cookie{Name: flowCookie, Value: base64.RawURLEncoding.EncodeToString([]byte("forged"))}, 400},
		"expired flow":   {state, reseal(expired), 400},
		"future flow":    {state, reseal(future), 400},
		"rejected code":  {state, flow, 401},
		"no id token":    {state, flow, 401},
		"nonce mismatch": {state, flow, 401},
	} {
		t.Run(name, func(t *testing.T) {
			switch name {
			case "no id token":
				token = ""
			case "nonce mismatch":
				token = idToken(issuer, client, "another nonce")
			default:
				token = "reject"
			}
			w := callback(tc.state, tc.flow)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d: %s", w.Code, tc.want, w.Body.String())
			}
			// Every callback consumes the flow cookie and issues no session.
			got := cookies(w)
			if c := got[flowCookie]; tc.flow != nil && (c == nil || c.MaxAge >= 0) {
				t.Fatal("flow cookie not cleared")
			}
			if got[httpapi.SessionCookie] != nil || got[httpapi.CSRFCookie] != nil {
				t.Fatal("a failed callback issued a session")
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOIDCTransportAndOrigins(t *testing.T) {
	called := false
	transport := secureTransport{roundTripFunc(func(r *http.Request) (*http.Response, error) { called = true; return nil, errors.New("test") })}
	r := httptest.NewRequest("GET", "http://issuer.example/keys", nil)
	if _, e := transport.RoundTrip(r); e == nil || called {
		t.Fatal("insecure OIDC request reached transport")
	}
	m := manager(t, "https://admin.example")
	for _, endpoint := range []string{"http://issuer.example/token", "https://user:password@issuer.example/token", "https://issuer.example/token#fragment"} {
		if secureEndpoint(endpoint) {
			t.Fatalf("accepted %s", endpoint)
		}
	}
	redirect := httptest.NewRequest("GET", "http://issuer.example/redirect", nil)
	if m.oidcClient().CheckRedirect(redirect, []*http.Request{r}) == nil {
		t.Fatal("OIDC HTTPS downgrade allowed")
	}
	if e := m.InitAuth(context.Background()); e != nil || m.OIDC != nil {
		t.Fatal("no issuer must leave single sign-on disabled", e)
	}
	m.Config.OIDCIssuer = "http://issuer.example"
	if e := m.InitAuth(context.Background()); e == nil || m.OIDC != nil {
		t.Fatal("production accepted an HTTP issuer")
	}
	if httpapi.AllowedOrigin(m.Config, "https://app.example") {
		t.Fatal("untrusted app accepted as admin origin")
	}
	m.Config.Development = true
	m.Config.DevelopmentUIOrigin = "http://127.0.0.1:5173"
	if !httpapi.AllowedOrigin(m.Config, m.Config.DevelopmentUIOrigin) || httpapi.AllowedOrigin(m.Config, "http://127.0.0.1:5174") {
		t.Fatal("development allowlist is not exact")
	}
	if httpapi.CookieName(m.Config.PublicURL, httpapi.SessionCookie) != "__Host-lastapp_session" {
		t.Fatal("production cookie lacks host prefix")
	}
	for _, origin := range []string{"https://admin.example/#fragment", "https://admin.example?query", "https://user@admin.example", "javascript:alert(1)"} {
		if config.ValidOrigin(origin, false) {
			t.Fatalf("accepted malformed origin %s", origin)
		}
	}
}
