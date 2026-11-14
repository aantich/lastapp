package control

import (
	"context"
	"database/sql"
	"net/http"
	"time"
)

// Both browser and delegated sessions use the same parent check on exchange and
// every data request. Agent tokens never authenticate directly to the data API.
func (s *Server) appSessionParent(ctx context.Context, q Queryer, v Row) error {
	denied := problem(401, "unauthenticated", "App session expired or revoked.")
	p := Principal{ID: str(v["user_id"])}
	if str(v["agent_grant_id"]) != "" {
		g, e := s.readAgentGrant(ctx, q, str(v["agent_grant_id"]))
		if e != nil {
			return denied
		}
		p.Agent = g
		if g.Revision != integer(v["agent_grant_revision"]) || requireAgent(g, "workspaces.test") != nil {
			return denied
		}
		var workspace string
		if e = q.QueryRowContext(ctx, "SELECT id FROM development_workspaces WHERE backing_space_id=? AND state='ready'", v["space_id"]).Scan(&workspace); e != nil {
			return denied
		}
		if e = s.authorize(ctx, q, p, "workspace.manage", Resource{Tenant: str(v["tenant_id"]), Space: str(v["space_id"])}, true); e != nil {
			return denied
		}
	} else {
		var n int
		e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions parent JOIN users u ON u.id=parent.user_id AND u.state='active' LEFT JOIN local_accounts la ON la.user_id=u.id WHERE parent.token_hash=? AND parent.user_id=? AND parent.purpose='console' AND parent.state='active' AND parent.expires_at>UTC_TIMESTAMP(6) AND (parent.auth_method='oidc' OR la.must_change_password=0)", v["parent_hash"], p.ID).Scan(&n)
		if e != nil {
			return e
		}
		if n != 1 {
			return denied
		}
		if e = s.workspaceGuard(ctx, q, p, str(v["space_id"])); e != nil {
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

func (s *Server) developmentLaunch(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.Agent == nil {
		return bad("Use ordinary browser App sign-in; development launch codes require a delegated agent connection.")
	}
	if e := requireAgent(p.Agent, "workspaces.test"); e != nil {
		return e
	}
	var in developmentLaunchInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if !pkceRE.MatchString(in.Challenge) || !pkceRE.MatchString(in.Nonce) {
		return bad("Provide an S256 challenge and random nonce.")
	}
	v, e := s.appRow(r.Context(), s.DB, r.PathValue("app"))
	if e != nil {
		return e
	}
	var wid string
	if e = s.DB.QueryRowContext(r.Context(), "SELECT id FROM development_workspaces WHERE backing_space_id=? AND state='ready'", v["space_id"]).Scan(&wid); e != nil {
		return missing()
	}
	if _, e = s.workspaceRow(r.Context(), s.DB, p, wid); e != nil {
		return e
	}
	a, e := accessOf(v)
	if e != nil {
		return e
	}
	if in.Origin == "" || in.Origin != a.Origin {
		return bad("Approve the exact development App origin using app_access_update first.")
	}
	code := token(32)
	e = s.mutate(r.Context(), p, "workspace.manage", Resource{Tenant: str(v["tenant_id"]), Space: str(v["space_id"])}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		_, e := tx.ExecContext(r.Context(), "INSERT INTO sessions(token_hash,user_id,csrf_hash,purpose,agent_grant_id,agent_grant_revision,tenant_id,space_id,app_id,origin,origin_generation,challenge,nonce,expires_at) VALUES(?,?,?,'launch',?,?,?,?,?,?,?,?,?,?)", hash(code), p.ID, hash(""), p.Agent.ID, p.Agent.Revision, v["tenant_id"], v["space_id"], v["id"], in.Origin, v["origin_generation"], in.Challenge, in.Nonce, time.Now().Add(launchLifetime))
		return e
	})
	if e != nil {
		return e
	}
	send(w, 200, Row{"code": code, "nonce": in.Nonce, "origin": in.Origin, "expires_in": int(launchLifetime.Seconds()), "instruction": "Exchange once with the locally held verifier using SDK connectDevelopment. Never put this code or an App token in a build."})
	return nil
}
