package dataapi

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/inventory"
	"github.com/aantich/lastapp/internal/registry"
)

const appTokenLifetime = 30 * time.Minute

func (s *Runtime) Launch(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		Challenge string `json:"challenge"`
		Nonce     string `json:"nonce"`
	}
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	if !registry.PKCEChallenge.MatchString(in.Challenge) || !registry.PKCEChallenge.MatchString(in.Nonce) {
		return registry.Bad("Valid S256 challenge and nonce required.")
	}
	v, e := inventory.AppRow(r.Context(), s.DB, r.PathValue("app"))
	if e != nil {
		return e
	}
	if v.State != registry.StateActive {
		return registry.Missing()
	}
	if e = s.Access.WorkspaceGuard(r.Context(), s.DB, p, v.SpaceID); e != nil {
		return e
	}
	if e = s.Access.Member(r.Context(), s.DB, p, v.TenantID, true); e != nil {
		return e
	}
	var sp string
	if e = s.DB.QueryRowContext(r.Context(), "SELECT state FROM spaces WHERE id=?", v.SpaceID).Scan(&sp); e != nil || sp != registry.StateActive {
		return registry.Missing()
	}
	a, e := registry.AccessOf(v.Access)
	if e != nil {
		return e
	}
	if a.Origin == "" {
		return registry.Conflict("A Space manager must approve the App origin first.")
	}
	cookie, e := r.Cookie(httpapi.CookieName(s.Config.PublicURL, httpapi.SessionCookie))
	if e != nil {
		return e
	}
	code := registry.Token(32)
	_, e = s.DB.ExecContext(r.Context(), "INSERT INTO sessions(token_hash,user_id,csrf_hash,purpose,parent_hash,tenant_id,space_id,app_id,origin,origin_generation,challenge,nonce,expires_at) VALUES(?,?,?,'launch',?,?,?,?,?,?,?,?,?)", registry.TokenHash(code), p.ID, registry.TokenHash(""), registry.TokenHash(cookie.Value), v.TenantID, v.SpaceID, v.ID, a.Origin, v.OriginGeneration, in.Challenge, in.Nonce, time.Now().Add(registry.LaunchLifetime))
	if e != nil {
		return e
	}
	httpapi.Send(w, 200, registry.Row{"code": code, "origin": a.Origin, "nonce": in.Nonce, "expires_in": int(registry.LaunchLifetime.Seconds())})
	return nil
}

func (s *Runtime) Exchange(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Code     string `json:"code"`
		Verifier string `json:"verifier"`
		Nonce    string `json:"nonce"`
	}
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	if !registry.PKCEVerifier.MatchString(in.Verifier) || len(in.Code) > 128 {
		return registry.Bad("Invalid launch exchange.")
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	v, e := registry.ScanOne[AppSession](r.Context(), tx, "SELECT "+appSessionColumns+" FROM sessions l JOIN users u ON u.id=l.user_id AND u.state='active' JOIN apps a ON a.id=l.app_id AND a.state='active' AND a.origin_generation=l.origin_generation JOIN spaces sp ON sp.id=l.space_id AND sp.state='active' JOIN tenants t ON t.id=l.tenant_id AND t.state='active' JOIN tenant_memberships m ON m.tenant_id=t.id AND m.user_id=u.id AND m.state='active' WHERE l.token_hash=? AND l.purpose='launch' AND l.state='active' AND l.expires_at>UTC_TIMESTAMP(6) FOR UPDATE", registry.TokenHash(in.Code))
	if e != nil {
		return registry.Problem(401, "invalid_exchange", "Launch code is invalid or expired.")
	}
	if e = s.appSessionParent(r.Context(), tx, v); e != nil {
		return e
	}
	if !registry.PKCEMatches(in.Verifier, v.Challenge) || subtle.ConstantTimeCompare([]byte(in.Nonce), []byte(v.Nonce)) != 1 || r.Header.Get("Origin") != access.Deref(v.Origin) {
		return registry.Problem(401, "invalid_exchange", "Launch challenge or origin mismatch.")
	}
	raw := registry.Token(32)
	_, e = tx.ExecContext(r.Context(), "UPDATE sessions SET state='consumed' WHERE token_hash=?", registry.TokenHash(in.Code))
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(r.Context(), "INSERT INTO sessions(token_hash,user_id,csrf_hash,purpose,parent_hash,tenant_id,space_id,app_id,origin,origin_generation,agent_grant_id,agent_grant_revision,expires_at) VALUES(?,?,?,'app',?,?,?,?,?,?,?,?,?)", registry.TokenHash(raw), v.UserID, registry.TokenHash(""), v.ParentHash, v.TenantID, v.SpaceID, v.AppID, v.Origin, v.OriginGeneration, v.AgentGrant, v.AgentRevision, time.Now().Add(appTokenLifetime))
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	w.Header().Set("Access-Control-Allow-Origin", access.Deref(v.Origin))
	w.Header().Add("Vary", "Origin")
	httpapi.Send(w, 200, registry.Row{"access_token": raw, "expires_in": int(appTokenLifetime.Seconds()), "token_type": "Bearer"})
	return nil
}

// AppSession is a launch code or App data session with the database
// resource of its Space.
type AppSession struct {
	UserID, TenantID, SpaceID, AppID, Challenge, Nonce, Resource string
	OriginGeneration                                             int64
	Origin, AgentGrant                                           *string
	AgentRevision                                                *int64
	ParentHash                                                   []byte
}

// appSessionColumns selects an AppSession from sessions l joined to spaces sp.
const appSessionColumns = "l.user_id,l.tenant_id,l.space_id,l.app_id,COALESCE(l.challenge,''),COALESCE(l.nonce,''),sp.database_resource_id,l.origin_generation,l.origin,l.agent_grant_id,l.agent_grant_revision,l.parent_hash"

func (v *AppSession) ScanFields() []any {
	return []any{&v.UserID, &v.TenantID, &v.SpaceID, &v.AppID, &v.Challenge, &v.Nonce, &v.Resource, &v.OriginGeneration, &v.Origin, &v.AgentGrant, &v.AgentRevision, &v.ParentHash}
}

// Both browser and delegated sessions use the same parent check on exchange and
// every data request. Agent tokens never authenticate directly to the data API.
func (s *Runtime) appSessionParent(ctx context.Context, q registry.Queryer, v AppSession) error {
	denied := registry.Problem(401, "unauthenticated", "App session expired or revoked.")
	p := access.Principal{ID: v.UserID}
	if access.Deref(v.AgentGrant) != "" {
		g, e := s.Access.ReadAgentGrant(ctx, q, access.Deref(v.AgentGrant))
		if e != nil {
			return denied
		}
		p.Agent = g
		if g.Revision != access.Deref(v.AgentRevision) || access.RequireAgent(g, "workspaces.test") != nil {
			return denied
		}
		var workspace string
		if e = q.QueryRowContext(ctx, "SELECT id FROM development_workspaces WHERE backing_space_id=? AND state='ready'", v.SpaceID).Scan(&workspace); e != nil {
			return denied
		}
		if e = s.Access.Authorize(ctx, q, p, "workspace.manage", access.Resource{Tenant: v.TenantID, Space: v.SpaceID}, true); e != nil {
			return denied
		}
	} else {
		var n int
		e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions parent JOIN users u ON u.id=parent.user_id AND u.state='active' LEFT JOIN local_accounts la ON la.user_id=u.id WHERE parent.token_hash=? AND parent.user_id=? AND parent.purpose='console' AND parent.state='active' AND parent.expires_at>UTC_TIMESTAMP(6) AND (parent.auth_method='oidc' OR la.must_change_password=0)", v.ParentHash, p.ID).Scan(&n)
		if e != nil {
			return e
		}
		if n != 1 {
			return denied
		}
		if e = s.Access.WorkspaceGuard(ctx, q, p, v.SpaceID); e != nil {
			return denied
		}
	}
	return nil
}

func (s *Runtime) Session(ctx context.Context, r *http.Request) (AppSession, error) {
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if raw == "" || raw == r.Header.Get("Authorization") || len(raw) > 128 {
		return AppSession{}, registry.Problem(401, "unauthenticated", "An App bearer token is required.")
	}
	v, e := registry.ScanOne[AppSession](ctx, s.DB, "SELECT "+appSessionColumns+" FROM sessions l JOIN users u ON u.id=l.user_id AND u.state='active' LEFT JOIN local_accounts la ON la.user_id=u.id JOIN apps a ON a.id=l.app_id AND a.tenant_id=l.tenant_id AND a.space_id=l.space_id AND a.state='active' AND a.origin_generation=l.origin_generation JOIN spaces sp ON sp.id=l.space_id AND sp.state='active' JOIN tenants t ON t.id=l.tenant_id AND t.state='active' JOIN tenant_memberships m ON m.tenant_id=t.id AND m.user_id=u.id AND m.state='active' WHERE l.token_hash=? AND l.purpose='app' AND l.state='active' AND l.expires_at>UTC_TIMESTAMP(6)", registry.TokenHash(raw))
	if e != nil {
		return v, registry.Problem(401, "unauthenticated", "App session expired or revoked.")
	}
	if e = s.appSessionParent(ctx, s.DB, v); e != nil {
		return v, e
	}
	if r.Header.Get("Origin") != access.Deref(v.Origin) {
		return v, registry.Problem(403, "invalid_origin", "App origin mismatch.")
	}
	return v, nil
}

func (s *Runtime) LaunchInfo(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var appID, name, space, spaceName, tenant string
	var dataAccess registry.JSONValue
	e := registry.Lookup(r.Context(), s.DB, "SELECT a.id,a.display_name,a.data_access_json,a.space_id,sp.display_name AS space_name,a.tenant_id FROM apps a JOIN spaces sp ON sp.id=a.space_id AND sp.state='active' WHERE a.id=? AND a.state='active'", []any{r.PathValue("app")}, &appID, &name, &dataAccess, &space, &spaceName, &tenant)
	if e != nil {
		return e
	}
	if e = s.Access.WorkspaceGuard(r.Context(), s.DB, p, space); e != nil {
		return e
	}
	if e = s.Access.Member(r.Context(), s.DB, p, tenant, true); e != nil {
		return e
	}
	a, e := registry.AccessOf(dataAccess)
	if e != nil {
		return e
	}
	if a.Origin == "" {
		return registry.Conflict("A Space manager must approve this App's origin.")
	}
	httpapi.Send(w, 200, registry.Row{"id": appID, "name": name, "space": spaceName, "origin": a.Origin})
	return nil
}
