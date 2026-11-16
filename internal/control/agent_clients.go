package control

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/registry"
)

// RFC 7591 registration records public-client metadata only. It cannot create
// users, grant authority, select a Tenant or skip interactive consent/S256 PKCE.
func (s *Server) agentRegisterClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if !s.admitAgent(w, r, &s.agentBootstrapLimiter, s.agentBootstrapSlots, "oauth-register", 10, 0.5) {
		return
	}
	defer func() { <-s.agentBootstrapSlots }()
	reject := func(code, message string) {
		send(w, 400, registry.Row{"error": code, "error_description": message})
	}
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		reject("invalid_client_metadata", "Send client metadata as application/json.")
		return
	}
	var in struct {
		Name      string   `json:"client_name"`
		Redirects []string `json:"redirect_uris"`
		Method    string   `json:"token_endpoint_auth_method"`
		Grants    []string `json:"grant_types"`
		Responses []string `json:"response_types"`
		Scope     string   `json:"scope"`
	}
	// RFC 7591 requires ignoring unrecognized metadata. Never fetch or reflect
	// client-supplied logo/website/software URLs or persist arbitrary extra data.
	r.Body = http.MaxBytesReader(w, r.Body, agentMaxClientMetadata)
	decoder := json.NewDecoder(r.Body)
	if decoder.Decode(&in) != nil {
		reject("invalid_client_metadata", "Invalid client metadata JSON.")
		return
	}
	var extra any
	if !errors.Is(decoder.Decode(&extra), io.EOF) {
		reject("invalid_client_metadata", "Send exactly one client metadata object.")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		in.Name = "MCP client"
	}
	if len(in.Name) > 160 || len(in.Redirects) == 0 || len(in.Redirects) > agentMaxClientRedirects {
		reject("invalid_client_metadata", "Provide a client name and a bounded list of redirect URIs.")
		return
	}
	for _, redirect := range in.Redirects {
		if !validAgentRedirect(redirect) || strings.Contains(redirect, "*") {
			reject("invalid_redirect_uri", "Redirects must be exact HTTPS or HTTP loopback URLs without wildcards, credentials or fragments.")
			return
		}
	}
	if in.Method != "" && in.Method != "none" {
		reject("invalid_client_metadata", "Automatic registration uses public clients with S256 PKCE and token_endpoint_auth_method none.")
		return
	}
	if len(in.Grants) == 0 {
		in.Grants = []string{"authorization_code", "refresh_token"}
	}
	for _, grant := range in.Grants {
		if grant != "authorization_code" && grant != "refresh_token" {
			reject("invalid_client_metadata", "Only authorization_code and refresh_token grants are supported.")
			return
		}
	}
	if !slices.Contains(in.Grants, "authorization_code") || len(in.Responses) > 0 && (len(in.Responses) != 1 || in.Responses[0] != "code") {
		reject("invalid_client_metadata", "Use authorization_code with response_type code.")
		return
	}
	scopes, e := agentScopeNames(in.Scope)
	if e != nil {
		reject("invalid_client_metadata", "Unsupported OAuth scope.")
		return
	}
	client := AgentClient{ID: "lastapp-client-" + registry.NewID(), Name: in.Name, Redirects: in.Redirects, Scope: strings.Join(scopes, " ")}
	metadata, _ := json.Marshal(client)
	conn, e := s.DB.Conn(ctx)
	if e != nil {
		writeError(w, r, e)
		return
	}
	defer conn.Close()
	// Serialize the small count/insert section across replicas. Reuse the shared
	// pinned-lock cleanup, which discards a connection if ownership is uncertain.
	const lock = "lastapp:agent-client-registration"
	var got int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,1)", lock).Scan(&got); e != nil {
		writeError(w, r, e)
		return
	}
	if got != 1 {
		writeError(w, r, rateLimited(w))
		return
	}
	defer releasePinnedLock(conn, lock)
	if e = cleanupAgentClients(ctx, conn); e != nil {
		writeError(w, r, e)
		return
	}
	var count int
	if e = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM agent_oauth_clients c WHERE NOT EXISTS (SELECT 1 FROM agent_grants g WHERE g.client_id=c.id)").Scan(&count); e != nil {
		writeError(w, r, e)
		return
	}
	if count >= agentMaxClients {
		writeError(w, r, rateLimited(w))
		return
	}
	if _, e = conn.ExecContext(ctx, "INSERT INTO agent_oauth_clients(id,metadata_json) VALUES(?,?)", client.ID, metadata); e != nil {
		writeError(w, r, e)
		return
	}
	send(w, 201, registry.Row{"client_id": client.ID, "client_id_issued_at": time.Now().Unix(), "client_name": client.Name, "redirect_uris": client.Redirects, "grant_types": in.Grants, "response_types": []string{"code"}, "token_endpoint_auth_method": "none", "scope": client.Scope})
}

// Unapproved registrations have a short, bounded lifetime. Approved client
// identities use the longer retention window and do not consume anonymous quota.
// Never evict a client during consent/code exchange or while a grant is live.
func cleanupAgentClients(ctx context.Context, q registry.Queryer) error {
	_, e := q.ExecContext(ctx, `DELETE FROM agent_oauth_clients WHERE
 ((created_at<? AND NOT EXISTS (SELECT 1 FROM agent_grants g WHERE g.client_id=agent_oauth_clients.id)) OR last_used_at<?)
 AND NOT EXISTS (SELECT 1 FROM agent_grants g WHERE g.client_id=agent_oauth_clients.id AND g.state='active' AND g.expires_at>UTC_TIMESTAMP(6))
 AND NOT EXISTS (SELECT 1 FROM agent_oauth_requests r WHERE r.client_id=agent_oauth_clients.id AND r.consumed=0 AND r.expires_at>UTC_TIMESTAMP(6))
 LIMIT 1000`, time.Now().UTC().Add(-agentUnapprovedClientLifetime), time.Now().UTC().Add(-agentClientIdleLifetime))
	return e
}
