package control

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aantich/lastapp/internal/registry"
)

func TestAgentAutomaticConnection(t *testing.T) {
	s, c, _, _, _ := foundationFixture(t)
	request := func(method, path, body, contentType string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, s.Config.PublicURL+path, strings.NewReader(body))
		r.Header.Set("Content-Type", contentType)
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		return w
	}
	object := func(w *httptest.ResponseRecorder) registry.Row {
		t.Helper()
		out := registry.Row{}
		if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			t.Fatal("expected JSON response", w.Code)
		}
		return out
	}
	challenge := request("POST", "/mcp", `{}`, "application/json")
	if challenge.Code != 401 || !strings.Contains(challenge.Header().Get("WWW-Authenticate"), "resource_metadata=") || strings.Contains(challenge.Header().Get("WWW-Authenticate"), "scope=") {
		t.Fatal("URL-only discovery must not force a read-only OAuth request")
	}
	resource := object(request("GET", "/.well-known/oauth-protected-resource/mcp", "", ""))
	authorization := object(request("GET", "/.well-known/oauth-authorization-server", "", ""))
	if resource["resource"] != s.mcpURL() || authorization["registration_endpoint"] != s.Config.PublicURL+"/oauth/register" {
		t.Fatal("automatic discovery incomplete")
	}
	offered := resource["scopes_supported"].([]any)
	if len(offered) != len(agentScopeCodes()) || !slices.Contains(offered, any("resources.write")) {
		t.Fatal("authoring capabilities cannot be discovered")
	}
	register := func(scope string) registry.Row {
		t.Helper()
		metadata, _ := json.Marshal(registry.Row{"client_name": "Automatic test client", "redirect_uris": []string{"http://localhost/callback"}, "token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code", "refresh_token"}, "scope": scope, "logo_uri": "http://127.0.0.1/private-do-not-fetch", "unknown_extension": true})
		response := request("POST", "/oauth/register", string(metadata), "application/json")
		if response.Code != 201 || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("automatic registration failed", response.Code)
		}
		out := object(response)
		if out["client_secret"] != nil || out["logo_uri"] != nil || out["token_endpoint_auth_method"] != "none" {
			t.Fatal("registration returned unsupported client metadata or secrets")
		}
		return out
	}
	registered := register("")
	if !strings.Contains(registry.Str(registered["scope"]), "entities.draft") {
		t.Fatal("client must discover authoring without configuring OAuth scopes")
	}
	var grants int
	if e := s.DB.QueryRow("SELECT COUNT(*) FROM agent_grants").Scan(&grants); e != nil || grants != 0 {
		t.Fatal("public registration created user authority", e)
	}
	verifier := token(48)
	sum := sha256.Sum256([]byte(verifier))
	redirect := "http://localhost:43187/callback"
	parameters := url.Values{"response_type": {"code"}, "client_id": {registry.Str(registered["client_id"])}, "redirect_uri": {redirect}, "resource": {s.mcpURL()}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "state": {"client-state"}}
	for _, badRedirect := range []string{"http://127.0.0.1:43187/callback", "http://localhost:43187/other", "https://evil.example/callback"} {
		parameters.Set("redirect_uri", badRedirect)
		if request("GET", "/oauth/authorize?"+parameters.Encode(), "", "").Code != 400 {
			t.Fatal("registered redirect boundary was bypassed")
		}
	}
	parameters.Set("redirect_uri", redirect)
	parameters.Del("code_challenge_method")
	if request("GET", "/oauth/authorize?"+parameters.Encode(), "", "").Code != 400 {
		t.Fatal("DCR bypassed mandatory PKCE")
	}
	parameters.Set("code_challenge_method", "S256")
	start := request("GET", "/oauth/authorize?"+parameters.Encode(), "", "")
	if start.Code != 302 {
		t.Fatal("URL-only authorization did not open consent", start.Code)
	}
	consentURL, _ := url.Parse(start.Header().Get("Location"))
	requestID := consentURL.Query().Get("request")
	info := c.want("GET", "/v1/agent-access/requests/"+requestID, nil, 0, 200)
	if !strings.Contains(registry.Str(info["scopes"]), "platform.manage") || !strings.Contains(registry.Str(info["scopes"]), "resources.write") {
		t.Fatal("consent cannot offer the user's authoring choices")
	}
	approved := c.want("POST", "/v1/agent-access/requests/"+requestID, registry.Row{"scope_kind": "platform", "permissions": []string{"read"}, "days": 1}, 0, 200)
	callback, _ := url.Parse(registry.Str(approved["redirect_url"]))
	if callback.Query().Get("iss") != s.Config.PublicURL || callback.Query().Get("state") != "client-state" {
		t.Fatal("authorization response lost issuer/state binding")
	}
	exchange := url.Values{"grant_type": {"authorization_code"}, "client_id": {registry.Str(registered["client_id"])}, "redirect_uri": {redirect}, "resource": {s.mcpURL()}, "code": {callback.Query().Get("code")}, "code_verifier": {token(48)}}
	if request("POST", "/oauth/token", exchange.Encode(), "application/x-www-form-urlencoded").Code != 400 {
		t.Fatal("dynamic clients bypassed verifier binding")
	}
	exchange.Set("code_verifier", verifier)
	response := request("POST", "/oauth/token", exchange.Encode(), "application/x-www-form-urlencoded")
	if response.Code != 200 {
		t.Fatal("automatic token exchange failed", response.Code)
	}
	tokens := object(response)
	if tokens["scope"] != "read" {
		t.Fatal("offered capabilities became authority without consent")
	}
	endpoint := httptest.NewServer(c.h)
	defer endpoint.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "automatic-registration-test", Version: "1"}, nil)
	session, e := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: endpoint.URL + "/mcp", HTTPClient: &http.Client{Transport: agentRoundTripper{registry.Str(tokens["access_token"])}}}, nil)
	if e != nil {
		t.Fatal("automatically registered client cannot connect", e)
	}
	defer session.Close()
	tools, e := session.ListTools(context.Background(), nil)
	if e != nil || len(tools.Tools) == 0 {
		t.Fatal("no tools after automatic connection", e)
	}
	for _, tool := range tools.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Fatal("unapproved mutation exposed", tool.Name)
		}
	}
	limited := register("read")
	parameters.Set("client_id", registry.Str(limited["client_id"]))
	start = request("GET", "/oauth/authorize?"+parameters.Encode(), "", "")
	consentURL, _ = url.Parse(start.Header().Get("Location"))
	info = c.want("GET", "/v1/agent-access/requests/"+consentURL.Query().Get("request"), nil, 0, 200)
	if info["scopes"] != "read" {
		t.Fatal("explicit client scope restriction was ignored")
	}
	if _, e = s.DB.Exec("UPDATE agent_oauth_clients SET last_used_at=?,created_at=?", time.Now().UTC().Add(-agentClientIdleLifetime-time.Hour), time.Now().UTC().Add(-agentClientIdleLifetime-time.Hour)); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("UPDATE agent_oauth_requests SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 MINUTE) WHERE client_id=?", limited["client_id"]); e != nil {
		t.Fatal(e)
	}
	if e = cleanupAgentClients(context.Background(), s.DB); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM agent_oauth_clients WHERE id=?", registered["client_id"]).Scan(&count); e != nil || count != 1 {
		t.Fatal("cleanup removed a client with a live grant", e)
	}
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM agent_oauth_clients WHERE id=?", limited["client_id"]).Scan(&count); e != nil || count != 0 {
		t.Fatal("unused registration not cleaned up", e)
	}
}

func TestAgentRegistrationRejectsUnsafeMetadata(t *testing.T) {
	_, c, _, _, _ := foundationFixture(t)
	for i, body := range []string{
		`{"redirect_uris":["http://evil.example/callback"]}`,
		`{"redirect_uris":["https://example.com/*"]}`,
		`{"redirect_uris":["https://user:password@example.com/cb"]}`,
		`{"redirect_uris":["https://example.com/cb#fragment"]}`,
		`{"redirect_uris":["http://localhost/cb"],"grant_types":["client_credentials"]}`,
		`{"redirect_uris":["http://localhost/cb"],"token_endpoint_auth_method":"client_secret_basic"}`,
		`{"redirect_uris":["http://localhost/cb"],"response_types":["token"]}`,
		`{"redirect_uris":["http://localhost/cb"],"scope":"superuser"}`,
		`null`,
		`{} {}`,
		`{"client_name":"` + strings.Repeat("x", agentMaxClientMetadata) + `"}`,
	} {
		r := httptest.NewRequest("POST", "/oauth/register", strings.NewReader(body))
		r.RemoteAddr = "198.51.100." + registry.Str(i+1) + ":1234"
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("unsafe registration case %d accepted: %d", i, w.Code)
		}
	}
}
