package control

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/authz"
)

// membership owns Tenant memberships, teams and their members, and Tenant
// administrators. Console handlers and MCP tools both call these methods.
type membership struct {
	db  *sql.DB
	run *runner
	// Collaborator of a domain not yet extracted, passed explicitly.
	prepareAccount func(accountInput) (preparedAccount, error)
}

// accountChoice selects an existing user or supplies a local account.
type accountChoice struct {
	User    string        `json:"user_id"`
	Account *accountInput `json:"account"`
}

type (
	memberRef struct {
		UserID string `json:"user_id"`
	}
	done struct {
		OK bool `json:"ok"`
	}
	teamPatch struct {
		Name  *string `json:"display_name"`
		State *string `json:"state"`
	}
)

// Membership is a Tenant member with account state, platform administration
// and Tenant administrator assignment.
type Membership struct {
	AdministratorID       *string   `json:"administrator_id"`
	AdministratorRevision *int64    `json:"administrator_revision"`
	CreatedAt             time.Time `json:"created_at"`
	Name                  string    `json:"display_name"`
	Email                 *string   `json:"email"`
	PlatformAdmin         bit       `json:"platform_admin"`
	Revision              int64     `json:"revision"`
	State                 string    `json:"state"`
	UserID                string    `json:"user_id"`
	UserState             string    `json:"user_state"`
}

func (v *Membership) fields() []any {
	return []any{&v.AdministratorID, &v.AdministratorRevision, &v.CreatedAt, &v.Name, &v.Email, &v.PlatformAdmin, &v.Revision, &v.State, &v.UserID, &v.UserState}
}

// Team is a Tenant team.
type Team struct {
	Name     string `json:"display_name"`
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
	Slug     string `json:"slug"`
	State    string `json:"state"`
}

const teamColumns = "g.display_name,g.id,g.revision,g.slug,g.state"

func (v *Team) fields() []any { return []any{&v.Name, &v.ID, &v.Revision, &v.Slug, &v.State} }

// TeamMember is a team member with account and Tenant membership state and
// the member's team administrator and reviewer assignments.
type TeamMember struct {
	AdministratorID       *string `json:"administrator_id"`
	AdministratorRevision *int64  `json:"administrator_revision"`
	Name                  string  `json:"display_name"`
	PlatformAdmin         bit     `json:"platform_admin"`
	ReviewerID            *string `json:"reviewer_id"`
	ReviewerRevision      *int64  `json:"reviewer_revision"`
	State                 string  `json:"state"`
	TenantState           string  `json:"tenant_state"`
	UserID                string  `json:"user_id"`
	UserState             string  `json:"user_state"`
}

func (v *TeamMember) fields() []any {
	return []any{&v.AdministratorID, &v.AdministratorRevision, &v.Name, &v.PlatformAdmin, &v.ReviewerID, &v.ReviewerRevision, &v.State, &v.TenantState, &v.UserID, &v.UserState}
}

// memberState is a membership's audited prior state.
type memberState struct {
	Revision int64  `json:"revision"`
	State    string `json:"state"`
	UserID   string `json:"user_id"`
}

// teamMemberState is a team membership's audited prior state.
type teamMemberState struct {
	State  string `json:"state"`
	UserID string `json:"user_id"`
}

// grantRef is an audited role assignment.
type grantRef struct {
	ID     string `json:"id"`
	RoleID string `json:"role_id"`
}

func (v *grantRef) fields() []any { return []any{&v.ID, &v.RoleID} }

func (m *membership) Memberships(ctx context.Context, a Actor, tenant string, page PageRequest) (Page[Membership], error) {
	if e := m.run.authorize(ctx, a, "membership.read", Resource{Tenant: tenant}); e != nil {
		return Page[Membership]{}, e
	}
	return pageScan[Membership](ctx, m.db, page, collectionFor("memberships"), "SELECT ag.id AS administrator_id,ag.revision AS administrator_revision,m.created_at,u.display_name,u.email,EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=u.id AND pg.role_id=?) AS platform_admin,m.revision,m.state,m.user_id,u.state AS user_state FROM tenant_memberships m JOIN users u ON u.id=m.user_id LEFT JOIN access_grants ag ON ag.user_id=m.user_id AND ag.tenant_id=m.tenant_id AND ag.role_id=? WHERE m.tenant_id=? ORDER BY u.display_name,u.id", SuperRole, TenantAdminRole, tenant)
}

// AddMembership adds an active existing user, or a local account, to tenant.
// Supplying a new password additionally requires platform administration.
func (m *membership) AddMembership(ctx context.Context, a Actor, tenant string, in accountChoice) (memberRef, error) {
	if (in.User == "") == (in.Account == nil) {
		return memberRef{}, bad("Select an existing user or supply an account.")
	}
	var account preparedAccount
	if in.Account != nil {
		if e := m.run.authorize(ctx, a, "membership.manage", Resource{Tenant: tenant}); e != nil {
			return memberRef{}, e
		}
		if in.Account.Password != "" {
			if e := m.run.authorize(ctx, a, "platform.admin", Resource{}); e != nil {
				return memberRef{}, e
			}
		}
		var e error
		account, e = m.prepareAccount(*in.Account)
		if e != nil {
			return memberRef{}, e
		}
	} else if e := id(in.User); e != nil {
		return memberRef{}, e
	}
	ctx, cancel := account.deadline(ctx)
	defer cancel()
	e := m.run.mutate(ctx, a, "membership.manage", Resource{Tenant: tenant}, func(tx *sql.Tx) error {
		if e := metadataQuota(ctx, tx, tenant, "tenant_memberships"); e != nil {
			return e
		}
		if in.Account != nil {
			platform, e := isPlatform(ctx, tx, a.Principal.ID)
			if e != nil {
				return e
			}
			in.User, e = localAccount(ctx, tx, account, platform)
			if e != nil {
				return e
			}
		}
		var state string
		if e := tx.QueryRowContext(ctx, "SELECT state FROM users WHERE id=? FOR UPDATE", in.User).Scan(&state); e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return bad("An active existing user is required.")
			}
			return e
		}
		if state != stateActive {
			return bad("An active existing user is required. They must sign in once first.")
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO tenant_memberships (tenant_id,user_id) VALUES (?,?)", tenant, in.User); e != nil {
			return e
		}
		return auditAs(ctx, tx, a, tenant, "membership.created", "user", in.User, Row{"before": nil, "after": Row{"user_id": in.User, "state": stateActive}})
	})
	if e != nil {
		return memberRef{}, e
	}
	return memberRef{UserID: in.User}, nil
}

// UpdateMembership enables or disables a Tenant membership under its
// revision, keeping an active Tenant administrator. Disabling revokes the
// user's agent delegations in the Tenant.
func (m *membership) UpdateMembership(ctx context.Context, a Actor, tenant, user, state string, revision int64) (done, error) {
	if !enabledState(state) {
		return done{}, bad("Invalid membership state.")
	}
	if revision < 1 {
		return done{}, revisionRequired()
	}
	e := m.run.mutate(ctx, a, "membership.manage", Resource{Tenant: tenant}, func(tx *sql.Tx) error {
		var before memberState
		e := lookup(ctx, tx, "SELECT user_id,state,revision FROM tenant_memberships WHERE tenant_id=? AND user_id=? FOR UPDATE", []any{tenant, user}, &before.UserID, &before.State, &before.Revision)
		if e != nil {
			return e
		}
		if e = checkRevision(before.Revision, revision); e != nil {
			return e
		}
		res, e := tx.ExecContext(ctx, "UPDATE tenant_memberships SET state=?,revision=revision+1 WHERE tenant_id=? AND user_id=?", state, tenant, user)
		if e != nil {
			return e
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			var found int
			if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenant_memberships WHERE tenant_id=? AND user_id=?", tenant, user).Scan(&found); e != nil {
				return e
			}
			if found == 0 {
				return missing()
			}
		}
		if e = ensureTenantAdmin(ctx, tx, tenant); e != nil {
			return e
		}
		if state == stateDisabled {
			if _, e = tx.ExecContext(ctx, "UPDATE agent_grants SET state='revoked',revision=revision+1 WHERE tenant_id=? AND user_id=? AND state='active'", tenant, user); e != nil {
				return e
			}
		}
		return auditAs(ctx, tx, a, tenant, "membership.updated", "user", user, Row{"before": before, "after": Row{"user_id": user, "state": state, "revision": revision + 1}})
	})
	if e != nil {
		return done{}, e
	}
	return done{OK: true}, nil
}

func (m *membership) Teams(ctx context.Context, a Actor, tenant string, page PageRequest) (Page[Team], error) {
	sc, e := m.run.scopes(ctx, a, tenant, "team.read")
	if e != nil {
		return Page[Team]{}, e
	}
	filter, args := teamSQL(sc, "g.id")
	args = append([]any{tenant}, args...)
	return pageScan[Team](ctx, m.db, page, collectionFor("teams"), "SELECT "+teamColumns+" FROM groups g WHERE g.tenant_id=? AND "+filter+" ORDER BY g.display_name,g.id", args...)
}

func teamSQL(sc scopeSet, column string) (string, []any) {
	if sc.All {
		return "1=1", nil
	}
	parts := []string{"1=0"}
	args := []any{}
	for _, v := range sc.Teams {
		parts = append(parts, column+"=?")
		args = append(args, v)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

func (m *membership) Team(ctx context.Context, a Actor, tenant, team string) (Team, error) {
	if e := m.run.authorize(ctx, a, "team.read", Resource{Tenant: tenant, Team: team}); e != nil {
		return Team{}, e
	}
	return scanOne[Team](ctx, m.db, "SELECT "+teamColumns+" FROM groups g WHERE g.tenant_id=? AND g.id=?", tenant, team)
}

func (m *membership) CreateTeam(ctx context.Context, a Actor, tenant string, in namedInput) (Ref, error) {
	if e := id(in.ID); e != nil {
		return Ref{}, e
	}
	if e := named(in.Slug, in.Name); e != nil {
		return Ref{}, e
	}
	e := m.run.mutate(ctx, a, "team.manage", Resource{Tenant: tenant}, func(tx *sql.Tx) error {
		if e := metadataQuota(ctx, tx, tenant, "groups"); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO groups (id,tenant_id,slug,display_name) VALUES (?,?,?,?)", in.ID, tenant, in.Slug, in.Name); e != nil {
			return e
		}
		return auditAs(ctx, tx, a, tenant, "group.created", "group", in.ID, Row{"before": nil, "after": Row{"display_name": in.Name, "state": stateActive}})
	})
	if e != nil {
		return Ref{}, e
	}
	return m.run.ref(ctx, "groups", tenant, in.ID)
}

func (m *membership) UpdateTeam(ctx context.Context, a Actor, tenant, team string, in teamPatch, revision int64) (Ref, error) {
	if revision < 1 {
		return Ref{}, revisionRequired()
	}
	e := m.run.mutate(ctx, a, "team.manage", Resource{Tenant: tenant, Team: team}, func(tx *sql.Tx) error {
		old, e := teamSnapshotOf(ctx, tx, tenant, team)
		if e != nil {
			return e
		}
		if e = checkRevision(old.Revision, revision); e != nil {
			return e
		}
		if in.Name != nil && (strings.TrimSpace(*in.Name) == "" || len(*in.Name) > 160) {
			return bad("Invalid name.")
		}
		if in.State != nil && !enabledState(*in.State) {
			return bad("Invalid state.")
		}
		if _, e = tx.ExecContext(ctx, "UPDATE groups SET display_name=COALESCE(?,display_name),state=COALESCE(?,state),revision=revision+1 WHERE id=? AND tenant_id=?", in.Name, in.State, team, tenant); e != nil {
			return e
		}
		return auditChangeAs(ctx, tx, a, tenant, "group.updated", "groups", "group", team, old)
	})
	if e != nil {
		return Ref{}, e
	}
	return m.run.ref(ctx, "groups", tenant, team)
}

func (m *membership) AddTeamMember(ctx context.Context, a Actor, tenant, team, user string, revision int64) (done, error) {
	return m.changeTeamMember(ctx, a, tenant, team, user, revision, true)
}

// RemoveTeamMember also deletes the user's team-scoped role assignments.
func (m *membership) RemoveTeamMember(ctx context.Context, a Actor, tenant, team, user string, revision int64) (done, error) {
	return m.changeTeamMember(ctx, a, tenant, team, user, revision, false)
}

// changeTeamMember adds or removes a member under the team's revision, which
// every membership change advances.
func (m *membership) changeTeamMember(ctx context.Context, a Actor, tenant, team, user string, revision int64, add bool) (done, error) {
	if e := id(user); e != nil {
		return done{}, e
	}
	if revision < 1 {
		return done{}, revisionRequired()
	}
	e := m.run.mutate(ctx, a, "team.manage", Resource{Tenant: tenant, Team: team}, func(tx *sql.Tx) error {
		old, e := lockRecord(ctx, tx, "groups", tenant, team)
		if e != nil {
			return e
		}
		if e = checkRevision(old.Revision, revision); e != nil {
			return e
		}
		var wasMember int
		if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM group_memberships WHERE tenant_id=? AND group_id=? AND user_id=?", tenant, team, user).Scan(&wasMember); e != nil {
			return e
		}
		if !add && wasMember == 0 {
			return missing()
		}
		if add {
			if e := activeTenantUser(ctx, tx, tenant, user); e != nil {
				return e
			}
		}
		if !add {
			removed, e := scanAll[grantRef](ctx, tx, "SELECT id,role_id FROM access_grants WHERE tenant_id=? AND scope_team_id=? AND user_id=?", tenant, team, user)
			if e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, "DELETE FROM access_grants WHERE tenant_id=? AND scope_team_id=? AND user_id=?", tenant, team, user); e != nil {
				return e
			}
			if len(removed) > 0 {
				if e = auditAs(ctx, tx, a, tenant, "team.roles.revoked_on_leave", "group", team, Row{"user_id": user, "assignments": removed}); e != nil {
					return e
				}
			}
		}
		query := "DELETE FROM group_memberships WHERE tenant_id=? AND group_id=? AND user_id=?"
		if add {
			if e := metadataQuota(ctx, tx, tenant, "group_memberships"); e != nil {
				return e
			}
			query = "INSERT INTO group_memberships (tenant_id,group_id,user_id) VALUES (?,?,?)"
		}
		if _, e = tx.ExecContext(ctx, query, tenant, team, user); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE groups SET revision=revision+1 WHERE id=?", team); e != nil {
			return e
		}
		return auditAs(ctx, tx, a, tenant, "group.members_changed", "group", team, Row{"before": Row{"user_id": user, "member": wasMember > 0}, "after": Row{"user_id": user, "member": add}})
	})
	if e != nil {
		return done{}, e
	}
	return done{OK: true}, nil
}

// SetTeamMemberState enables or disables a team member under the team's
// revision. Enabling requires an active Tenant member.
func (m *membership) SetTeamMemberState(ctx context.Context, a Actor, tenant, team, user, state string, revision int64) (done, error) {
	if !enabledState(state) {
		return done{}, bad("Invalid membership state.")
	}
	if revision < 1 {
		return done{}, revisionRequired()
	}
	e := m.run.mutate(ctx, a, "team.manage", Resource{Tenant: tenant, Team: team}, func(tx *sql.Tx) error {
		locked, e := lockRecord(ctx, tx, "groups", tenant, team)
		if e != nil {
			return e
		}
		if e = checkRevision(locked.Revision, revision); e != nil {
			return e
		}
		var before teamMemberState
		e = lookup(ctx, tx, "SELECT user_id,state FROM group_memberships WHERE tenant_id=? AND group_id=? AND user_id=?", []any{tenant, team, user}, &before.UserID, &before.State)
		if e != nil {
			return e
		}
		if state == stateActive {
			if e = activeTenantUser(ctx, tx, tenant, user); e != nil {
				return e
			}
		}
		if _, e = tx.ExecContext(ctx, "UPDATE group_memberships SET state=? WHERE tenant_id=? AND group_id=? AND user_id=?", state, tenant, team, user); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE groups SET revision=revision+1 WHERE id=?", team); e != nil {
			return e
		}
		return auditAs(ctx, tx, a, tenant, "team.membership.updated", "group", team, Row{"before": before, "after": Row{"user_id": user, "state": state}})
	})
	if e != nil {
		return done{}, e
	}
	return done{OK: true}, nil
}

// TeamMembers requires team management authority, not only team read.
func (m *membership) TeamMembers(ctx context.Context, a Actor, tenant, team string, page PageRequest) (Page[TeamMember], error) {
	if e := m.run.authorize(ctx, a, "team.manage", Resource{Tenant: tenant, Team: team}); e != nil {
		return Page[TeamMember]{}, e
	}
	var found string
	if e := lookup(ctx, m.db, "SELECT id FROM groups WHERE tenant_id=? AND id=?", []any{tenant, team}, &found); e != nil {
		return Page[TeamMember]{}, e
	}
	return pageScan[TeamMember](ctx, m.db, page, collectionFor("members"), "SELECT ga.id AS administrator_id,ga.revision AS administrator_revision,u.display_name,EXISTS(SELECT 1 FROM access_grants pg WHERE pg.user_id=u.id AND pg.role_id=?) AS platform_admin,gr.id AS reviewer_id,gr.revision AS reviewer_revision,gm.state,tm.state AS tenant_state,gm.user_id,u.state AS user_state FROM group_memberships gm JOIN users u ON u.id=gm.user_id JOIN tenant_memberships tm ON tm.tenant_id=gm.tenant_id AND tm.user_id=gm.user_id LEFT JOIN access_grants ga ON ga.tenant_id=gm.tenant_id AND ga.scope_team_id=gm.group_id AND ga.user_id=gm.user_id AND ga.role_id=? LEFT JOIN access_grants gr ON gr.tenant_id=gm.tenant_id AND gr.scope_team_id=gm.group_id AND gr.user_id=gm.user_id AND gr.role_id=? WHERE gm.tenant_id=? AND gm.group_id=? ORDER BY u.display_name,gm.user_id", SuperRole, authz.TeamAdminRole, authz.TeamReviewerRole, tenant, team)
}

func (s *Server) memberships(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.members.Memberships(r.Context(), httpActor(r, p), r.PathValue("tenant"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) addMembership(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in accountChoice
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.AddMembership(r.Context(), httpActor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		send(w, 201, v)
	}
	return e
}

func (s *Server) updateMembership(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		State string `json:"state"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.UpdateMembership(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("user"), in.State, ifMatch(r))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) groups(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.members.Teams(r.Context(), httpActor(r, p), r.PathValue("tenant"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) getGroup(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.members.Team(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in namedInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.CreateTeam(r.Context(), httpActor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

func (s *Server) updateGroup(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in teamPatch
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.UpdateTeam(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), in, ifMatch(r))
	if e == nil {
		sendRef(w, 200, v)
	}
	return e
}

func (s *Server) addGroupMember(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		User string `json:"user_id"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.AddTeamMember(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), in.User, ifMatch(r))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) removeGroupMember(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.members.RemoveTeamMember(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), r.PathValue("user"), ifMatch(r))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) updateGroupMember(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		State string `json:"state"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.SetTeamMemberState(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), r.PathValue("user"), in.State, ifMatch(r))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) groupMembers(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.members.TeamMembers(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}
