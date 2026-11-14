package control

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"github.com/aantich/lastapp/internal/entity"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const appTokenLifetime = 30 * time.Minute
const launchLifetime = 2 * time.Minute

var verifierRE = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
var pkceRE = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func (s *Server) appRow(ctx context.Context, q Queryer, app string) (Row, error) {
	return row(ctx, q, "SELECT * FROM apps WHERE id=?", app)
}
func accessOf(v Row) (entity.AppAccess, error) {
	var a entity.AppAccess
	b, _ := json.Marshal(v["data_access_json"])
	e := json.Unmarshal(b, &a)
	return a, e
}
func (s *Server) appAccessGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.appRow(r.Context(), s.DB, r.PathValue("app"))
	if e != nil {
		return e
	}
	if e = s.authorize(r.Context(), s.DB, p, "app.manage", Resource{Tenant: str(v["tenant_id"]), App: str(v["id"])}, true); e != nil {
		return e
	}
	a, e := accessOf(v)
	if e == nil {
		send(w, 200, Row{"revision": v["revision"], "access": a})
	}
	return e
}
func (s *Server) appAccessSave(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in entity.AppAccess
	if e := decode(w, r, &in); e != nil {
		return e
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	v, e := s.appRow(r.Context(), s.DB, r.PathValue("app"))
	if e != nil {
		return e
	}
	res := Resource{Tenant: str(v["tenant_id"]), App: str(v["id"])}
	e = s.mutate(r.Context(), p, "app.manage", res, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		v, e = s.appRow(r.Context(), tx, str(v["id"]))
		if e != nil {
			return e
		}
		if e = checkRevision(integer(v["revision"]), rev); e != nil {
			return e
		}
		workspace := false
		var wid sql.NullString
		if e = tx.QueryRowContext(r.Context(), "SELECT workspace_id FROM spaces WHERE id=?", v["space_id"]).Scan(&wid); e != nil {
			return e
		}
		workspace = wid.Valid
		old, e := accessOf(v)
		if e != nil {
			return e
		}
		spaceManager := s.authorize(r.Context(), tx, p, "entity.manage", Resource{Tenant: res.Tenant, Space: str(v["space_id"])}, true) == nil
		if !spaceManager && (old.Origin != in.Origin || old.Inherit != in.Inherit || entity.Hash(old.Ceiling) != entity.Hash(in.Ceiling)) {
			return problem(403, "space_approval_required", "A Space manager must approve the origin and access ceiling.")
		}
		if in.Origin != "" {
			u, _ := url.Parse(in.Origin)
			admin, _ := url.Parse(s.Config.PublicURL)
			loopback := u != nil && (u.Hostname() == "localhost" || net.ParseIP(u.Hostname()) != nil && net.ParseIP(u.Hostname()).IsLoopback())
			if !validOrigin(in.Origin, false) || u == nil || len(in.Origin) > 500 || u.Scheme != "https" && !((s.Config.Development || workspace) && loopback) || u.Hostname() != strings.ToLower(u.Hostname()) || strings.EqualFold(strings.TrimSuffix(u.Hostname(), "."), strings.TrimSuffix(admin.Hostname(), ".")) {
				return bad("App origin must use a separate host from the console.")
			}
		}
		if len(in.Ceiling)+len(in.Restrictions) > 128 {
			return bad("Too many App access gates.")
		}
		for _, gs := range [][]entity.Gate{in.Ceiling, in.Restrictions} {
			for _, g := range gs {
				if e := id(g.Permission); e != nil {
					return e
				}
				pv, e := row(r.Context(), tx, "SELECT p.entity_id,v.definition_json FROM permissions p JOIN entity_versions v ON v.id=p.version_id WHERE p.id=? AND p.tenant_id=? AND p.space_id=? AND p.state='active' AND p.domain='data'", g.Permission, res.Tenant, v["space_id"])
				if e != nil {
					return bad("Choose an active permission from this Space.")
				}
				d, e := defOf(pv["definition_json"])
				if e != nil {
					return e
				}
				for _, f := range g.Fields {
					if _, ok := d.Field(f); !ok {
						return bad("Unknown gate field.")
					}
				}
				if g.When != nil {
					if e = d.CheckCondition(*g.When, false); e != nil {
						return bad(e.Error())
					}
				}
				for _, role := range g.Roles {
					var n int
					if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM roles WHERE id=? AND tenant_id=? AND app_id=? AND domain='data' AND state='active'", role, res.Tenant, v["id"]).Scan(&n); e != nil {
						return e
					}
					if n != 1 {
						return bad("App gates can reference only this App's business roles.")
					}
				}
			}
		}
		host := ""
		if in.Origin != "" && !workspace {
			parsed, _ := url.Parse(in.Origin)
			host = strings.ToLower(parsed.Hostname())
			var occupied int
			if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM apps WHERE origin_host=? AND id<>?", host, v["id"]).Scan(&occupied); e != nil {
				return e
			}
			if occupied > 0 {
				return conflict("Each App needs a separate host. This host is already approved for another App.")
			}
		}
		b, _ := json.Marshal(in)
		_, e = tx.ExecContext(r.Context(), "UPDATE apps SET data_access_json=?,approved_origin=?,origin_host=?,origin_generation=origin_generation+IF(?<>?,1,0),revision=revision+1 WHERE id=?", b, nullable(in.Origin), nullable(host), old.Origin, in.Origin, v["id"])
		if e != nil {
			return e
		}
		if e = projectAppPermissions(r.Context(), tx, str(v["id"]), in); e != nil {
			return e
		}
		return audit(r.Context(), tx, res.Tenant, p.ID, "app.data_access_changed", "app", str(v["id"]), r.Header.Get("X-Request-ID"), Row{})
	})
	if e != nil {
		return e
	}
	return s.appAccessGet(w, r, p)
}
func (s *Server) appLaunch(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		Challenge string `json:"challenge"`
		Nonce     string `json:"nonce"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if !pkceRE.MatchString(in.Challenge) || !pkceRE.MatchString(in.Nonce) {
		return bad("Valid S256 challenge and nonce required.")
	}
	v, e := s.appRow(r.Context(), s.DB, r.PathValue("app"))
	if e != nil {
		return e
	}
	if v["state"] != "active" {
		return missing()
	}
	if e = s.workspaceGuard(r.Context(), s.DB, p, str(v["space_id"])); e != nil {
		return e
	}
	if e = s.member(r.Context(), s.DB, p, str(v["tenant_id"]), true); e != nil {
		return e
	}
	var sp string
	if e = s.DB.QueryRowContext(r.Context(), "SELECT state FROM spaces WHERE id=?", v["space_id"]).Scan(&sp); e != nil || sp != "active" {
		return missing()
	}
	a, e := accessOf(v)
	if e != nil {
		return e
	}
	if a.Origin == "" {
		return conflict("A Space manager must approve the App origin first.")
	}
	cookie, e := r.Cookie(s.cookieName("lastapp_session"))
	if e != nil {
		return e
	}
	code := token(32)
	_, e = s.DB.ExecContext(r.Context(), "INSERT INTO sessions(token_hash,user_id,csrf_hash,purpose,parent_hash,tenant_id,space_id,app_id,origin,origin_generation,challenge,nonce,expires_at) VALUES(?,?,?,'launch',?,?,?,?,?,?,?,?,?)", hash(code), p.ID, hash(""), hash(cookie.Value), v["tenant_id"], v["space_id"], v["id"], a.Origin, v["origin_generation"], in.Challenge, in.Nonce, time.Now().Add(launchLifetime))
	if e != nil {
		return e
	}
	send(w, 200, Row{"code": code, "origin": a.Origin, "nonce": in.Nonce, "expires_in": int(launchLifetime.Seconds())})
	return nil
}
func (s *Server) exchange(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Code     string `json:"code"`
		Verifier string `json:"verifier"`
		Nonce    string `json:"nonce"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if !verifierRE.MatchString(in.Verifier) || len(in.Code) > 128 {
		return bad("Invalid launch exchange.")
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	v, e := row(r.Context(), tx, "SELECT l.* FROM sessions l JOIN users u ON u.id=l.user_id AND u.state='active' JOIN apps a ON a.id=l.app_id AND a.state='active' AND a.origin_generation=l.origin_generation JOIN spaces sp ON sp.id=l.space_id AND sp.state='active' JOIN tenants t ON t.id=l.tenant_id AND t.state='active' JOIN tenant_memberships m ON m.tenant_id=t.id AND m.user_id=u.id AND m.state='active' WHERE l.token_hash=? AND l.purpose='launch' AND l.state='active' AND l.expires_at>UTC_TIMESTAMP(6) FOR UPDATE", hash(in.Code))
	if e != nil {
		return problem(401, "invalid_exchange", "Launch code is invalid or expired.")
	}
	if e = s.appSessionParent(r.Context(), tx, v); e != nil {
		return e
	}
	challenge := base64.RawURLEncoding.EncodeToString(hash(in.Verifier))
	if subtle.ConstantTimeCompare([]byte(challenge), []byte(str(v["challenge"]))) != 1 || subtle.ConstantTimeCompare([]byte(in.Nonce), []byte(str(v["nonce"]))) != 1 || r.Header.Get("Origin") != str(v["origin"]) {
		return problem(401, "invalid_exchange", "Launch challenge or origin mismatch.")
	}
	raw := token(32)
	_, e = tx.ExecContext(r.Context(), "UPDATE sessions SET state='consumed' WHERE token_hash=?", hash(in.Code))
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(r.Context(), "INSERT INTO sessions(token_hash,user_id,csrf_hash,purpose,parent_hash,tenant_id,space_id,app_id,origin,origin_generation,agent_grant_id,agent_grant_revision,expires_at) VALUES(?,?,?,'app',?,?,?,?,?,?,?,?,?)", hash(raw), v["user_id"], hash(""), v["parent_hash"], v["tenant_id"], v["space_id"], v["app_id"], v["origin"], v["origin_generation"], v["agent_grant_id"], v["agent_grant_revision"], time.Now().Add(appTokenLifetime))
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	w.Header().Set("Access-Control-Allow-Origin", str(v["origin"]))
	w.Header().Add("Vary", "Origin")
	send(w, 200, Row{"access_token": raw, "expires_in": int(appTokenLifetime.Seconds()), "token_type": "Bearer"})
	return nil
}
func (s *Server) dataSession(ctx context.Context, r *http.Request) (Row, error) {
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if raw == "" || raw == r.Header.Get("Authorization") || len(raw) > 128 {
		return nil, problem(401, "unauthenticated", "An App bearer token is required.")
	}
	v, e := row(ctx, s.DB, "SELECT l.user_id,l.tenant_id,l.space_id,l.app_id,l.origin,l.origin_generation,l.parent_hash,l.agent_grant_id,l.agent_grant_revision,sp.database_resource_id FROM sessions l JOIN users u ON u.id=l.user_id AND u.state='active' LEFT JOIN local_accounts la ON la.user_id=u.id JOIN apps a ON a.id=l.app_id AND a.tenant_id=l.tenant_id AND a.space_id=l.space_id AND a.state='active' AND a.origin_generation=l.origin_generation JOIN spaces sp ON sp.id=l.space_id AND sp.state='active' JOIN tenants t ON t.id=l.tenant_id AND t.state='active' JOIN tenant_memberships m ON m.tenant_id=t.id AND m.user_id=u.id AND m.state='active' WHERE l.token_hash=? AND l.purpose='app' AND l.state='active' AND l.expires_at>UTC_TIMESTAMP(6)", hash(raw))
	if e != nil {
		return nil, problem(401, "unauthenticated", "App session expired or revoked.")
	}
	if e = s.appSessionParent(ctx, s.DB, v); e != nil {
		return nil, e
	}
	if r.Header.Get("Origin") != str(v["origin"]) {
		return nil, problem(403, "invalid_origin", "App origin mismatch.")
	}
	return v, nil
}

func projectAppPermissions(ctx context.Context, tx *sql.Tx, app string, a entity.AppAccess) error {
	if _, e := tx.ExecContext(ctx, "DELETE rp FROM role_permissions rp JOIN roles r ON r.id=rp.role_id WHERE r.app_id=? AND r.domain='data'", app); e != nil {
		return e
	}
	gates := []entity.Gate{}
	if !a.Inherit {
		gates = append(gates, a.Ceiling...)
	}
	if a.Restricted {
		gates = append(gates, a.Restrictions...)
	}
	for _, g := range gates {
		for _, role := range g.Roles {
			if _, e := tx.ExecContext(ctx, "INSERT IGNORE INTO role_permissions(role_id,permission_id,domain) VALUES(?,?,'data')", role, g.Permission); e != nil {
				return e
			}
		}
	}
	return nil
}

func (s *Server) appLaunchInfo(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := row(r.Context(), s.DB, "SELECT a.id,a.display_name,a.data_access_json,a.space_id,sp.display_name AS space_name,a.tenant_id FROM apps a JOIN spaces sp ON sp.id=a.space_id AND sp.state='active' WHERE a.id=? AND a.state='active'", r.PathValue("app"))
	if e != nil {
		return e
	}
	if e = s.workspaceGuard(r.Context(), s.DB, p, str(v["space_id"])); e != nil {
		return e
	}
	if e = s.member(r.Context(), s.DB, p, str(v["tenant_id"]), true); e != nil {
		return e
	}
	a, e := accessOf(v)
	if e != nil {
		return e
	}
	if a.Origin == "" {
		return conflict("A Space manager must approve this App's origin.")
	}
	send(w, 200, Row{"id": v["id"], "name": v["display_name"], "space": v["space_name"], "origin": a.Origin})
	return nil
}
