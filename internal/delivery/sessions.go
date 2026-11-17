package delivery

import (
	"context"
	"database/sql"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/inventory"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

type DevelopmentLaunchInput struct {
	Challenge string `json:"challenge"`
	Nonce     string `json:"nonce"`
	Origin    string `json:"origin"`
}

func (s *Service) Launch(ctx context.Context, actor op.Actor, app string, in DevelopmentLaunchInput) (registry.Row, error) {
	p := actor.Principal
	if p.Agent == nil {
		return nil, registry.Bad("Use ordinary browser App sign-in; development launch codes require a delegated agent connection.")
	}
	if e := access.RequireAgent(p.Agent, "workspaces.test"); e != nil {
		return nil, e
	}
	if !registry.PKCEChallenge.MatchString(in.Challenge) || !registry.PKCEChallenge.MatchString(in.Nonce) {
		return nil, registry.Bad("Provide an S256 challenge and random nonce.")
	}
	v, e := inventory.AppRow(ctx, s.DB, app)
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
	e = s.Run.Mutate(ctx, actor, "workspace.manage", access.Resource{Tenant: v.TenantID, Space: v.SpaceID}, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, "INSERT INTO sessions(token_hash,user_id,csrf_hash,purpose,agent_grant_id,agent_grant_revision,tenant_id,space_id,app_id,origin,origin_generation,challenge,nonce,expires_at) VALUES(?,?,?,'launch',?,?,?,?,?,?,?,?,?,?)", registry.TokenHash(code), p.ID, registry.TokenHash(""), p.Agent.ID, p.Agent.Revision, v.TenantID, v.SpaceID, v.ID, in.Origin, v.OriginGeneration, in.Challenge, in.Nonce, time.Now().Add(registry.LaunchLifetime))
		return e
	})
	if e != nil {
		return nil, e
	}
	return registry.Row{"code": code, "nonce": in.Nonce, "origin": in.Origin, "expires_in": int(registry.LaunchLifetime.Seconds()), "instruction": "Exchange once with the locally held verifier using SDK connectDevelopment. Never put this code or an App token in a build."}, nil
}
