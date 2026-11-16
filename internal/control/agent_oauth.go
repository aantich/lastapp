package control

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/registry"
)

type AgentClient struct {
	ID        string   `json:"client_id"`
	Name      string   `json:"client_name"`
	Redirects []string `json:"redirect_uris"`
	Scope     string   `json:"scope,omitempty"`
}

func (s *Server) mcpURL() string { return s.Config.PublicURL + "/mcp" }
func (s *Server) agentRoutes() {
	s.mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, registry.Row{"resource": s.mcpURL(), "authorization_servers": []string{s.Config.PublicURL}, "scopes_supported": agentScopeCodes(), "bearer_methods_supported": []string{"header"}})
	})
	s.mux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, registry.Row{"issuer": s.Config.PublicURL, "authorization_endpoint": s.Config.PublicURL + "/oauth/authorize", "token_endpoint": s.Config.PublicURL + "/oauth/token", "registration_endpoint": s.Config.PublicURL + "/oauth/register", "revocation_endpoint": s.Config.PublicURL + "/oauth/revoke", "response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"}, "code_challenge_methods_supported": []string{"S256"}, "token_endpoint_auth_methods_supported": []string{"none"}, "scopes_supported": agentScopeCodes(), "client_id_metadata_document_supported": true, "authorization_response_iss_parameter_supported": true})
	})
	s.mux.HandleFunc("POST /oauth/register", s.agentRegisterClient)
	s.mux.HandleFunc("GET /oauth/authorize", s.agentAuthorize)
	s.mux.HandleFunc("POST /oauth/token", s.agentToken)
	s.mux.HandleFunc("POST /oauth/revoke", s.agentRevokeToken)
	s.api("GET /v1/agent-access/config", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		send(w, 200, registry.Row{"mcp_url": s.mcpURL(), "permissions": agentPermissionCatalog(), "presets": agentPresets, "default_permissions": agentDefaultPermissions, "default_limits": agentLimits{TargetIDs: []string{}, MaxDatabases: 1}, "default_days": agentGrantDays, "max_days": agentMaxGrantDays, "max_databases": agentMaxDatabases, "max_targets": agentMaxTargets, "access_token_seconds": int(agentAccessLifetime.Seconds()), "docs": agentGuides})
		return nil
	})
	s.api("GET /v1/agent-access", s.agentGrantList)
	s.api("PATCH /v1/agent-access/{grant}", s.agentGrantUpdate)
	s.api("GET /v1/agent-access/requests/{request}", s.agentConsentInfo)
	s.api("POST /v1/agent-access/requests/{request}", s.agentConsent)
	s.mux.Handle("/mcp", s.mcpHandler())
}

func validAgentRedirect(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || len(raw) > 1500 || u.User != nil || u.Fragment != "" || u.Host == "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	return u.Scheme == "http" && (u.Hostname() == "localhost" || net.ParseIP(u.Hostname()) != nil && net.ParseIP(u.Hostname()).IsLoopback())
}

func (s *Server) resolveAgentClient(ctx context.Context, key string) (AgentClient, error) {
	for _, c := range s.Config.AgentClients {
		if c.ID == key {
			return c, nil
		}
	}
	if s.Config.Development && key == "lastapp-development" {
		return AgentClient{ID: key, Name: "Local development agent", Redirects: []string{"http://127.0.0.1:8790/callback"}}, nil
	}
	if strings.HasPrefix(key, "lastapp-client-") {
		var metadata []byte
		if e := s.DB.QueryRowContext(ctx, "SELECT metadata_json FROM agent_oauth_clients c WHERE id=? AND (created_at>? OR EXISTS (SELECT 1 FROM agent_grants g WHERE g.client_id=c.id))", key, time.Now().UTC().Add(-agentUnapprovedClientLifetime)).Scan(&metadata); e != nil {
			return AgentClient{}, registry.Bad("Unknown OAuth client. Reconnect from your agent to register automatically.")
		}
		var c AgentClient
		if e := json.Unmarshal(metadata, &c); e != nil || c.ID != key {
			return AgentClient{}, registry.Bad("Invalid OAuth client metadata.")
		}
		return c, nil
	}
	// Client ID Metadata Documents are the only dynamic fetch. No redirects,
	// proxies, local/private addresses or unbounded bodies are accepted.
	u, e := url.Parse(key)
	if e != nil || len(key) > 500 || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Hostname() == "" {
		return AgentClient{}, registry.Bad("Use a pre-registered client ID or a public HTTPS client metadata URL.")
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if e != nil {
			return nil, e
		}
		for _, ip := range ips {
			if !publicAgentIP(ip.String()) {
				return nil, registry.Bad("Client metadata must use a public address.")
			}
		}
		if len(ips) == 0 {
			return nil, registry.Bad("Client metadata host unavailable.")
		}
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequestWithContext(ctx, "GET", key, nil)
	req.Header.Set("Accept", "application/json")
	response, e := client.Do(req)
	if e != nil {
		return AgentClient{}, registry.Bad("Client metadata could not be verified.")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return AgentClient{}, registry.Bad("Client metadata could not be verified.")
	}
	b, e := io.ReadAll(io.LimitReader(response.Body, agentMaxClientMetadata+1))
	if e != nil || len(b) > agentMaxClientMetadata {
		return AgentClient{}, registry.Bad("Client metadata exceeds its limit.")
	}
	var c AgentClient
	if json.Unmarshal(b, &c) != nil || c.ID != key || c.Name == "" || len(c.Name) > 160 || len(c.Redirects) == 0 || len(c.Redirects) > agentMaxClientRedirects {
		return c, registry.Bad("Invalid client metadata document.")
	}
	for _, redirect := range c.Redirects {
		if !validAgentRedirect(redirect) {
			return c, registry.Bad("Invalid client redirect URI.")
		}
	}
	return c, nil
}

func publicAgentIP(raw string) bool {
	ip := net.ParseIP(raw)
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.To4() == nil {
		_, global, _ := net.ParseCIDR("2000::/3")
		if !global.Contains(ip) {
			return false
		}
	}
	for _, cidr := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/23", "2002::/16", "64:ff9b::/96"} {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return false
		}
	}
	return true
}

func agentRedirectMatches(allowed, got string) bool {
	if allowed == got {
		return true
	}
	a, e := url.Parse(allowed)
	b, f := url.Parse(got)
	if e != nil || f != nil {
		return false
	}
	// RFC 8252 permits a dynamic port for native loopback redirects only.
	ip := net.ParseIP(a.Hostname())
	if a.Scheme != "http" || (a.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback())) || a.Hostname() != b.Hostname() {
		return false
	}
	a.Host = a.Hostname()
	b.Host = b.Hostname()
	return a.String() == b.String()
}

func agentScopeNames(raw string) ([]string, error) {
	out := strings.Fields(raw)
	if len(out) == 0 {
		// Offer capabilities at consent; only the user's checked choices become
		// a grant. General-purpose MCP clients need no Lastapp scope configuration.
		out = agentScopeCodes()
	}
	for _, code := range out {
		found := false
		for _, p := range agentPermissions {
			found = found || registry.Str(p["code"]) == code
		}
		if !found {
			return nil, registry.Bad("Unsupported agent scope.")
		}
	}
	slices.Sort(out)
	out = slices.Compact(out)
	return out, nil
}

func (s *Server) agentAuthorize(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if !s.admitAgent(w, r, &s.agentBootstrapLimiter, s.agentBootstrapSlots, "oauth-authorize", 20, 1) {
		return
	}
	defer func() { <-s.agentBootstrapSlots }()
	q := r.URL.Query()
	c, e := s.resolveAgentClient(ctx, q.Get("client_id"))
	if e != nil {
		writeError(w, r, e)
		return
	}
	redirect := q.Get("redirect_uri")
	matched := false
	for _, v := range c.Redirects {
		matched = matched || agentRedirectMatches(v, redirect)
	}
	scopes, e := agentScopeNames(q.Get("scope"))
	if q.Get("scope") == "" && c.Scope != "" {
		scopes, e = agentScopeNames(c.Scope)
	}
	if !matched || !validAgentRedirect(redirect) || e != nil || q.Get("response_type") != "code" || q.Get("resource") != s.mcpURL() || q.Get("code_challenge_method") != "S256" || len(q.Get("code_challenge")) != 43 || len(q.Get("state")) > 1024 || !verifierRE.MatchString(q.Get("code_challenge")) {
		writeError(w, r, registry.Bad("Invalid OAuth request: use the MCP resource, a registered redirect and S256 PKCE."))
		return
	}
	key := registry.NewID()
	var pending int
	if e = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM agent_oauth_requests WHERE expires_at>UTC_TIMESTAMP(6) AND consumed=0").Scan(&pending); e != nil {
		writeError(w, r, e)
		return
	}
	if pending >= 10000 {
		writeError(w, r, rateLimited(w))
		return
	}
	_, e = s.DB.ExecContext(ctx, "INSERT INTO agent_oauth_requests(id,client_id,client_name,redirect_uri,challenge,state_value,scopes,expires_at,resource_uri) VALUES(?,?,?,?,?,?,?,?,?)", key, c.ID, c.Name, redirect, q.Get("code_challenge"), q.Get("state"), strings.Join(scopes, " "), time.Now().UTC().Add(agentCodeLifetime), s.mcpURL())
	if e != nil {
		writeError(w, r, e)
		return
	}
	http.Redirect(w, r, "/console/agent-consent?request="+key, http.StatusFound)
}

type agentConsentInput struct {
	Tenant      string      `json:"tenant_id"`
	Kind        string      `json:"scope_kind"`
	Scope       string      `json:"scope_id"`
	Permissions []string    `json:"permissions"`
	Limits      agentLimits `json:"limits"`
	Days        int         `json:"days"`
	Deny        bool        `json:"deny"`
	State       string      `json:"state"`
}

func (s *Server) validateAgentConsent(ctx context.Context, q registry.Queryer, p Principal, in *agentConsentInput) error {
	if in.Days < 1 || in.Days > agentMaxGrantDays {
		return registry.Bad("Invalid grant lifetime.")
	}
	if len(in.Permissions) == 0 || len(in.Permissions) > len(agentPermissions) {
		return registry.Bad("Choose at least one agent permission.")
	}
	normalized, e := agentScopeNames(strings.Join(in.Permissions, " "))
	if e != nil {
		return e
	}
	in.Permissions = normalized
	for _, permission := range in.Permissions {
		if !slices.Contains(agentPermissionKinds(permission), in.Kind) {
			return registry.Bad("The selected permission is not supported at this scope.")
		}
	}
	if in.Kind == "platform" {
		if !p.Platform || in.Tenant != "" || in.Scope != "" {
			return registry.Bad("Invalid platform scope.")
		}
		return s.authorize(ctx, q, p, "platform.admin", Resource{}, true)
	}
	if p.Platform {
		return registry.Bad("Use a Tenant account to delegate Tenant authoring.")
	}
	if e := registry.ID(in.Tenant); e != nil {
		return e
	}
	if e := s.member(ctx, q, p, in.Tenant, true); e != nil {
		return e
	}
	res := Resource{Tenant: in.Tenant}
	readAction := "tenant.read"
	switch in.Kind {
	case "tenant":
		if in.Scope != "" {
			return registry.Bad("Tenant scope has no child scope ID.")
		}
	case "team":
		res.Team = in.Scope
		readAction = "team.read"
	case "space":
		res.Space = in.Scope
		readAction = "space.read"
	case "app":
		res.App = in.Scope
		readAction = "app.read"
	default:
		return registry.Bad("Unsupported agent scope.")
	}
	if in.Kind != "tenant" {
		if e := registry.ID(in.Scope); e != nil {
			return e
		}
	}
	if e := s.authorize(ctx, q, p, readAction, res, false); e != nil {
		return e
	}
	for _, permission := range in.Permissions {
		action := ""
		switch permission {
		case "read":
			continue
		case "workspaces.write", "workspaces.test":
			action = "workspace.manage"
		case "releases.write":
			action = "release.manage"
		case "production.deploy":
			action = "production.deploy"
		case "resources.write":
			action = "space.manage"
			if in.Kind == "space" {
				action = "app.manage"
			}
			if in.Kind == "app" {
				action = "microapp.manage"
			}
			if in.Kind == "team" {
				action = "team.manage"
			}
		case "entities.draft", "entities.check", "entities.publish":
			if in.Kind != "tenant" && in.Kind != "space" {
				return registry.Bad("Entity authoring requires Tenant or Space scope.")
			}
			action = "entity.manage"
			if permission == "entities.publish" {
				action = "entity.publish"
			}
		case "access.manage":
			action = "authorization.manage"
		case "lifecycle.manage":
			continue // Each lifecycle effect also requires ordinary authority and access.manage.
		case "databases.provision":
			if in.Kind != "tenant" || in.Limits.MaxDatabases < 1 || in.Limits.MaxDatabases > agentMaxDatabases || len(in.Limits.TargetIDs) == 0 || len(in.Limits.TargetIDs) > agentMaxTargets {
				return registry.Bad("Provisioning requires Tenant scope, approved targets and a database limit within the published policy.")
			}
			for _, target := range in.Limits.TargetIDs {
				var n int
				if q.QueryRowContext(ctx, "SELECT COUNT(*) FROM database_targets WHERE id=? AND state='active' AND managed_enabled=1", target).Scan(&n) != nil || n != 1 {
					return registry.Bad("Choose an active managed database target.")
				}
			}
			action = "database.manage"
		case "platform.manage":
			return registry.Bad("Platform operations require a platform grant.")
		}
		if e := s.authorize(ctx, q, p, action, res, true); e != nil {
			return e
		}
	}
	return nil
}

// oauthRequest is a pending agent authorization request.
type oauthRequest struct {
	ID, ClientID, ClientName, RedirectURI, Challenge, State, Scopes, Resource string
	GrantID                                                                   *string
	ExpiresAt                                                                 time.Time
}

const oauthRequestColumns = "id,client_id,client_name,redirect_uri,challenge,state_value,scopes,resource_uri,grant_id,expires_at"

func (v *oauthRequest) ScanFields() []any {
	return []any{&v.ID, &v.ClientID, &v.ClientName, &v.RedirectURI, &v.Challenge, &v.State, &v.Scopes, &v.Resource, &v.GrantID, &v.ExpiresAt}
}

func (s *Server) agentConsentInfo(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := registry.ScanOne[oauthRequest](r.Context(), s.DB, "SELECT "+oauthRequestColumns+" FROM agent_oauth_requests WHERE id=? AND resource_uri=? AND consumed=0 AND grant_id IS NULL AND expires_at>UTC_TIMESTAMP(6)", r.PathValue("request"), s.mcpURL())
	if e != nil {
		return e
	}
	send(w, 200, registry.Row{
		"id": v.ID, "client_name": v.ClientName, "client_id": v.ClientID, "redirect_uri": v.RedirectURI, "scopes": v.Scopes, "expires_at": v.ExpiresAt, "resource_uri": v.Resource,
		"client_identity_verified": false, "permissions": agentPermissionCatalog(), "default_days": agentGrantDays, "max_days": agentMaxGrantDays,
	})
	return nil
}

func (s *Server) agentConsent(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in agentConsentInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var activeUser string
	if e = tx.QueryRowContext(r.Context(), "SELECT state FROM users WHERE id=? FOR UPDATE", p.ID).Scan(&activeUser); e != nil {
		return e
	}
	if activeUser != registry.StateActive {
		return registry.Missing()
	}
	v, e := registry.ScanOne[oauthRequest](r.Context(), tx, "SELECT "+oauthRequestColumns+" FROM agent_oauth_requests WHERE id=? AND resource_uri=? AND consumed=0 AND grant_id IS NULL AND expires_at>UTC_TIMESTAMP(6) FOR UPDATE", r.PathValue("request"), s.mcpURL())
	if e != nil {
		return e
	}
	target, _ := url.Parse(v.RedirectURI)
	params := target.Query()
	params.Set("state", v.State)
	params.Set("iss", s.Config.PublicURL)
	if in.Deny {
		params.Set("error", "access_denied")
		_, e = tx.ExecContext(r.Context(), "UPDATE agent_oauth_requests SET consumed=1 WHERE id=?", v.ID)
	} else {
		if e = s.validateAgentConsent(r.Context(), tx, p, &in); e != nil {
			return e
		}
		requested := strings.Fields(v.Scopes)
		for _, permission := range in.Permissions {
			if !slices.Contains(requested, permission) {
				return registry.Bad("Reconnect requesting the additional OAuth scopes before approving them.")
			}
		}
		grant, code := registry.NewID(), token(32)
		var existing int
		if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM agent_grants WHERE user_id=? AND state='active' AND expires_at>UTC_TIMESTAMP(6)", p.ID).Scan(&existing); e != nil {
			return e
		}
		if existing >= agentMaxConnections {
			return registry.Conflict("Revoke unused agent connections before approving another.")
		}
		permissions, _ := json.Marshal(in.Permissions)
		limits, _ := json.Marshal(in.Limits)
		_, e = tx.ExecContext(r.Context(), "INSERT INTO agent_grants(id,user_id,client_id,client_name,tenant_id,scope_kind,scope_id,permissions_json,limits_json,expires_at,audience_uri) VALUES(?,?,?,?,?,?,?,?,?,?,?)", grant, p.ID, v.ClientID, v.ClientName, registry.Nullable(in.Tenant), in.Kind, registry.Nullable(in.Scope), permissions, limits, time.Now().UTC().Add(time.Duration(in.Days)*24*time.Hour), v.Resource)
		if e == nil {
			_, e = tx.ExecContext(r.Context(), "UPDATE agent_oauth_requests SET grant_id=?,code_hash=?,expires_at=? WHERE id=?", grant, hash(code), time.Now().UTC().Add(agentCodeLifetime), v.ID)
		}
		if e == nil {
			e = audit(r.Context(), tx, in.Tenant, p.ID, "agent.approved", "agent_grant", grant, r.Header.Get("X-Request-ID"), registry.Row{"client_id": v.ClientID, "scope_kind": in.Kind, "scope_id": in.Scope, "permissions": in.Permissions})
		}
		params.Set("code", code)
	}
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	target.RawQuery = params.Encode()
	send(w, 200, registry.Row{"redirect_url": target.String()})
	return nil
}

// AgentGrantItem is an agent connection in its owner's or Tenant's list.
type AgentGrantItem struct {
	ClientID    string             `json:"client_id"`
	ClientName  string             `json:"client_name"`
	CreatedAt   time.Time          `json:"created_at"`
	ExpiresAt   time.Time          `json:"expires_at"`
	ID          string             `json:"id"`
	LastUsedAt  *time.Time         `json:"last_used_at"`
	Limits      registry.JSONValue `json:"limits_json"`
	OwnerName   string             `json:"owner_name"`
	Permissions registry.JSONValue `json:"permissions_json"`
	Revision    int64              `json:"revision"`
	ScopeID     *string            `json:"scope_id"`
	ScopeKind   string             `json:"scope_kind"`
	ScopeName   string             `json:"scope_name"`
	State       string             `json:"state"`
	TenantID    *string            `json:"tenant_id"`
	TenantName  *string            `json:"tenant_name"`
	UserID      string             `json:"user_id"`
}

func (v *AgentGrantItem) ScanFields() []any {
	return []any{&v.ClientID, &v.ClientName, &v.CreatedAt, &v.ExpiresAt, &v.ID, &v.LastUsedAt, &v.Limits, &v.OwnerName, &v.Permissions, &v.Revision, &v.ScopeID, &v.ScopeKind, &v.ScopeName, &v.State, &v.TenantID, &v.TenantName, &v.UserID}
}

func (s *Server) agentGrantList(w http.ResponseWriter, r *http.Request, p Principal) error {
	tenant := r.URL.Query().Get("tenant_id")
	filter := "g.user_id=?"
	args := []any{p.ID}
	if tenant != "" {
		if e := s.authorize(r.Context(), s.DB, p, "authorization.manage", Resource{Tenant: tenant}, false); e != nil {
			return e
		}
		filter = "g.tenant_id=?"
		args = []any{tenant}
	}
	items, e := pageRows[AgentGrantItem](r.Context(), s.DB, r, `SELECT g.client_id,g.client_name,g.created_at,g.expires_at,g.id,g.last_used_at,g.limits_json,u.display_name AS owner_name,g.permissions_json,g.revision,g.scope_id,g.scope_kind,
 COALESCE(sp.display_name,a.display_name,team.display_name,t.display_name,'Platform') AS scope_name,g.state,g.tenant_id,t.display_name AS tenant_name,g.user_id
 FROM agent_grants g JOIN users u ON u.id=g.user_id LEFT JOIN tenants t ON t.id=g.tenant_id
 LEFT JOIN spaces sp ON g.scope_kind='space' AND sp.id=g.scope_id AND sp.tenant_id=g.tenant_id
 LEFT JOIN apps a ON g.scope_kind='app' AND a.id=g.scope_id AND a.tenant_id=g.tenant_id
 LEFT JOIN groups team ON g.scope_kind='team' AND team.id=g.scope_id AND team.tenant_id=g.tenant_id WHERE `+filter+" ORDER BY g.created_at DESC,g.id", args...)
	if e == nil {
		sendPage(w, r, items)
	}
	return e
}

func (s *Server) agentGrantUpdate(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in agentConsentInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	g, e := s.readAgentGrant(r.Context(), s.DB, r.PathValue("grant"))
	if e != nil {
		return e
	}
	if g.User != p.ID {
		if in.State != registry.StateRevoked || g.Tenant == "" {
			return registry.Missing()
		}
		if e = s.authorize(r.Context(), s.DB, p, "authorization.manage", Resource{Tenant: g.Tenant}, true); e != nil {
			return e
		}
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var current lockedRecord
	if e = registry.Lookup(r.Context(), tx, "SELECT revision,state FROM agent_grants WHERE id=? FOR UPDATE", []any{g.ID}, &current.Revision, &current.State); e != nil {
		return e
	}
	if e = checkRevision(current.Revision, rev); e != nil {
		return e
	}
	if current.State != registry.StateActive {
		return registry.Conflict("Revoked grants cannot be restored. Reconnect the agent.")
	}
	if g.User != p.ID {
		if e = s.authorize(r.Context(), tx, p, "authorization.manage", Resource{Tenant: g.Tenant}, true); e != nil {
			return e
		}
	}
	if in.State == registry.StateRevoked {
		_, e = tx.ExecContext(r.Context(), "UPDATE agent_grants SET state='revoked',revision=revision+1 WHERE id=?", g.ID)
	} else {
		in.Tenant = g.Tenant
		in.Kind = g.Kind
		in.Scope = g.Scope
		if e = s.validateAgentConsent(r.Context(), tx, p, &in); e != nil {
			return e
		}
		permissions, _ := json.Marshal(in.Permissions)
		limits, _ := json.Marshal(in.Limits)
		_, e = tx.ExecContext(r.Context(), "UPDATE agent_grants SET permissions_json=?,limits_json=?,expires_at=?,revision=revision+1 WHERE id=?", permissions, limits, time.Now().UTC().Add(time.Duration(in.Days)*24*time.Hour), g.ID)
	}
	if e != nil {
		return e
	}
	if e = audit(r.Context(), tx, g.Tenant, p.ID, "agent.changed", "agent_grant", g.ID, r.Header.Get("X-Request-ID"), registry.Row{"state": in.State, "permissions": in.Permissions}); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	send(w, 200, registry.Row{"id": g.ID, "revision": rev + 1})
	return nil
}

func oauthError(w http.ResponseWriter, code string) {
	send(w, 400, registry.Row{"error": code, "error_description": "OAuth request rejected. Reconnect through Lastapp if necessary."})
}

func (s *Server) agentToken(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if !s.admitAgent(w, r, &s.agentBootstrapLimiter, s.agentTokenSlots, "oauth-token", 30, 5) {
		return
	}
	defer func() { <-s.agentTokenSlots }()
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if r.ParseForm() != nil {
		oauthError(w, "invalid_request")
		return
	}
	if r.Form.Get("resource") != s.mcpURL() || r.Form.Get("client_id") == "" {
		oauthError(w, "invalid_target")
		return
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		writeError(w, r, e)
		return
	}
	defer tx.Rollback()
	var grantID, family string
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		v, err := registry.ScanOne[oauthRequest](ctx, tx, "SELECT "+oauthRequestColumns+" FROM agent_oauth_requests WHERE code_hash=? AND resource_uri=? AND consumed=0 AND expires_at>UTC_TIMESTAMP(6) FOR UPDATE", hash(r.Form.Get("code")), s.mcpURL())
		if err != nil {
			oauthError(w, "invalid_grant")
			return
		}
		verifier := r.Form.Get("code_verifier")
		sum := sha256.Sum256([]byte(verifier))
		challenge := base64.RawURLEncoding.EncodeToString(sum[:])
		if !verifierRE.MatchString(verifier) || subtle.ConstantTimeCompare([]byte(challenge), []byte(v.Challenge)) != 1 || r.Form.Get("client_id") != v.ClientID || r.Form.Get("redirect_uri") != v.RedirectURI {
			oauthError(w, "invalid_grant")
			return
		}
		grantID = deref(v.GrantID)
		family = registry.NewID()
		_, e = tx.ExecContext(ctx, "UPDATE agent_oauth_requests SET consumed=1 WHERE id=?", v.ID)
	case "refresh_token":
		var session, grant, state, client string
		err := registry.Lookup(ctx, tx, "SELECT t.id,t.grant_id,t.family_id,t.state,g.client_id FROM agent_token_sessions t JOIN agent_grants g ON g.id=t.grant_id WHERE t.refresh_hash=? FOR UPDATE", []any{hash(r.Form.Get("refresh_token"))}, &session, &grant, &family, &state, &client)
		if err != nil || client != r.Form.Get("client_id") {
			oauthError(w, "invalid_grant")
			return
		}
		if state != registry.StateActive {
			_, e = tx.ExecContext(ctx, "UPDATE agent_token_sessions SET state='revoked' WHERE family_id=?", family)
			if e == nil {
				e = tx.Commit()
			}
			if e != nil {
				writeError(w, r, e)
				return
			}
			oauthError(w, "invalid_grant")
			return
		}
		grantID = grant
		_, e = tx.ExecContext(ctx, "UPDATE agent_token_sessions SET state='rotated' WHERE id=?", session)
	default:
		oauthError(w, "unsupported_grant_type")
		return
	}
	if e != nil {
		writeError(w, r, e)
		return
	}
	g, e := s.readAgentGrant(ctx, tx, grantID)
	if e != nil {
		oauthError(w, "invalid_grant")
		return
	}
	p := Principal{ID: g.User, Agent: g}
	if e = s.checkAgent(ctx, tx, p, Resource{Tenant: g.Tenant}, false); e != nil {
		oauthError(w, "invalid_grant")
		return
	}
	access, refresh := token(32), token(48)
	var rotations int
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM agent_token_sessions WHERE grant_id=?", g.ID).Scan(&rotations); e != nil {
		writeError(w, r, e)
		return
	}
	if rotations >= agentMaxTokenRotations {
		oauthError(w, "invalid_grant")
		return
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO agent_token_sessions(id,grant_id,grant_revision,family_id,access_hash,refresh_hash,access_expires_at) VALUES(?,?,?,?,?,?,?)", registry.NewID(), g.ID, g.Revision, family, hash(access), hash(refresh), time.Now().UTC().Add(agentAccessLifetime))
	if e != nil {
		writeError(w, r, e)
		return
	}
	if _, e = tx.ExecContext(ctx, "UPDATE agent_oauth_clients SET last_used_at=UTC_TIMESTAMP(6) WHERE id=?", g.Client); e != nil {
		writeError(w, r, e)
		return
	}
	if e = tx.Commit(); e != nil {
		writeError(w, r, e)
		return
	}
	send(w, 200, registry.Row{"access_token": access, "refresh_token": refresh, "token_type": "Bearer", "expires_in": int(agentAccessLifetime.Seconds()), "scope": strings.Join(g.Permissions, " ")})
}

func (s *Server) agentRevokeToken(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if !s.admitAgent(w, r, &s.agentBootstrapLimiter, s.agentTokenSlots, "oauth-revoke", 30, 5) {
		return
	}
	defer func() { <-s.agentTokenSlots }()
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if r.ParseForm() != nil {
		oauthError(w, "invalid_request")
		return
	}
	var family string
	e := s.DB.QueryRowContext(r.Context(), "SELECT t.family_id FROM agent_token_sessions t JOIN agent_grants g ON g.id=t.grant_id WHERE (t.access_hash=? OR t.refresh_hash=?) AND g.client_id=?", hash(r.Form.Get("token")), hash(r.Form.Get("token")), r.Form.Get("client_id")).Scan(&family)
	if e == nil {
		_, e = s.DB.ExecContext(r.Context(), "UPDATE agent_token_sessions SET state='revoked' WHERE family_id=?", family)
	}
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		writeError(w, r, e)
		return
	}
	send(w, 200, registry.Row{})
}
