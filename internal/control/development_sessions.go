package control

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

// Both browser and delegated sessions use the same parent check on exchange and
// every data request. Agent tokens never authenticate directly to the data API.
func (s *Server) appSessionParent(ctx context.Context, q registry.Queryer, v appSession) error {
	denied := registry.Problem(401, "unauthenticated", "App session expired or revoked.")
	p := access.Principal{ID: v.UserID}
	if access.Deref(v.AgentGrant) != "" {
		g, e := s.ReadAgentGrant(ctx, q, access.Deref(v.AgentGrant))
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
		if e = s.Authorize(ctx, q, p, "workspace.manage", access.Resource{Tenant: v.TenantID, Space: v.SpaceID}, true); e != nil {
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
		if e = s.WorkspaceGuard(ctx, q, p, v.SpaceID); e != nil {
			return denied
		}
	}
	return nil
}

type developmentLaunchInput struct {
	Challenge string `json:"challenge"`
	Nonce     string `json:"nonce"`
	Origin    string `json:"origin"`
}

func (s *Server) Launch(ctx context.Context, actor op.Actor, app string, in developmentLaunchInput) (registry.Row, error) {
	p := actor.Principal
	if p.Agent == nil {
		return nil, registry.Bad("Use ordinary browser App sign-in; development launch codes require a delegated agent connection.")
	}
	if e := access.RequireAgent(p.Agent, "workspaces.test"); e != nil {
		return nil, e
	}
	if !pkceRE.MatchString(in.Challenge) || !pkceRE.MatchString(in.Nonce) {
		return nil, registry.Bad("Provide an S256 challenge and random nonce.")
	}
	v, e := s.appRow(ctx, s.DB, app)
	if e != nil {
		return nil, e
	}
	var wid string
	if e = s.DB.QueryRowContext(ctx, "SELECT id FROM development_workspaces WHERE backing_space_id=? AND state='ready'", v.SpaceID).Scan(&wid); e != nil {
		return nil, registry.Missing()
	}
	if _, e = s.workspaceRow(ctx, s.DB, p, wid); e != nil {
		return nil, e
	}
	a, e := registry.AccessOf(v.Access)
	if e != nil {
		return nil, e
	}
	if in.Origin == "" || in.Origin != a.Origin {
		return nil, registry.Bad("Approve the exact development App origin using app_access_update first.")
	}
	code := registry.Token(32)
	e = s.run.Mutate(ctx, actor, "workspace.manage", access.Resource{Tenant: v.TenantID, Space: v.SpaceID}, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, "INSERT INTO sessions(token_hash,user_id,csrf_hash,purpose,agent_grant_id,agent_grant_revision,tenant_id,space_id,app_id,origin,origin_generation,challenge,nonce,expires_at) VALUES(?,?,?,'launch',?,?,?,?,?,?,?,?,?,?)", hash(code), p.ID, hash(""), p.Agent.ID, p.Agent.Revision, v.TenantID, v.SpaceID, v.ID, in.Origin, v.OriginGeneration, in.Challenge, in.Nonce, time.Now().Add(launchLifetime))
		return e
	})
	if e != nil {
		return nil, e
	}
	return registry.Row{"code": code, "nonce": in.Nonce, "origin": in.Origin, "expires_in": int(launchLifetime.Seconds()), "instruction": "Exchange once with the locally held verifier using SDK connectDevelopment. Never put this code or an App token in a build."}, nil
}

func (s *Server) developmentLaunch(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in developmentLaunchInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.Launch(r.Context(), httpActor(r, p), r.PathValue("app"), in)
	return reply(w, 200, v, e)
}
