package control

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	mariadb "github.com/aantich/go-mariadb"
)

type credentials struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	AccountHost string `json:"account_host"`
}
type databaseInput struct {
	ID          string      `json:"id"`
	Mode        string      `json:"management_mode"`
	Layout      string      `json:"layout"`
	Target      string      `json:"target_id"`
	Database    string      `json:"database_name"`
	Credentials credentials `json:"credentials"`
}

const (
	targetColumns = "id,display_name,canonical_host,port,tls_mode,state,managed_enabled,revision,created_at"
	dbColumns     = "id,tenant_id,target_id,database_name,management_mode,layout,state,attempts,last_error_code,last_error_message,last_verified_at,revision,created_at"
)

// storage owns database targets and Tenant database resources. Console
// handlers and MCP tools both call these methods; the worker applies the
// queued work and rechecks its authorizer separately.
type storage struct {
	db           *sql.DB
	run          *runner
	seal         func(purpose, record, tenant string, value any) ([]byte, error)
	config       *Config
	registryName string
}

type targetPatch struct {
	Name        *string      `json:"display_name"`
	State       *string      `json:"state"`
	Managed     *bool        `json:"managed_enabled"`
	Credentials *credentials `json:"credentials"`
}

// databaseChange is a registered credential replacement or retirement; a
// retry carries neither.
type databaseChange struct {
	State       string       `json:"state"`
	Credentials *credentials `json:"credentials"`
}

func (v *storage) Targets(ctx context.Context, a Actor, page PageRequest) (Page[Row], error) {
	if e := v.run.authorize(ctx, a, "platform.admin", Resource{}); e != nil {
		return Page[Row]{}, e
	}
	return pageQuery(ctx, v.db, page, collectionFor("database-targets"), "SELECT "+targetColumns+" FROM database_targets ORDER BY display_name")
}

// AvailableTargets lists the active targets a Tenant may allocate on.
func (v *storage) AvailableTargets(ctx context.Context, a Actor, tenant string, managed bool, page PageRequest) (Page[Row], error) {
	if e := v.run.authorize(ctx, a, "database.manage", Resource{Tenant: tenant}); e != nil {
		return Page[Row]{}, e
	}
	filter := ""
	if managed {
		filter = " AND managed_enabled=1"
	}
	return pageQuery(ctx, v.db, page, collectionFor("database-targets"), "SELECT id,display_name,managed_enabled FROM database_targets WHERE state='active'"+filter+" ORDER BY display_name")
}

type targetInput struct {
	ID          string       `json:"id"`
	Name        string       `json:"display_name"`
	Host        string       `json:"canonical_host"`
	Port        int          `json:"port"`
	TLS         string       `json:"tls_mode"`
	State       string       `json:"state"`
	Managed     bool         `json:"managed_enabled"`
	Credentials *credentials `json:"credentials"`
}

var hostRE = regexp.MustCompile(`^[a-zA-Z0-9.:\-]+$`)

func (v *storage) validateTarget(in *targetInput) error {
	in.Host = strings.ToLower(strings.TrimSuffix(in.Host, "."))
	if !hostRE.MatchString(in.Host) || len(in.Host) > 253 || in.Port < 1 || in.Port > 65535 {
		return bad("Invalid database endpoint.")
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 160 {
		return bad("Invalid target name.")
	}
	if in.TLS != "verify_identity" && !(in.TLS == "disabled" && v.config.Development) {
		return bad("TLS must verify the server identity; disabled is allowed only in local development.")
	}
	if in.State == "" {
		in.State = stateActive
	}
	if !enabledState(in.State) {
		return bad("Invalid target state.")
	}
	return nil
}

func (v *storage) CreateTarget(ctx context.Context, a Actor, in targetInput) (Ref, error) {
	if e := id(in.ID); e != nil {
		return Ref{}, e
	}
	if e := v.validateTarget(&in); e != nil {
		return Ref{}, e
	}
	if in.Managed && (in.Credentials == nil || in.Credentials.Username == "" || in.Credentials.Password == "") {
		return Ref{}, bad("Managed targets require provisioning credentials.")
	}
	e := v.run.mutate(ctx, a, "platform.admin", Resource{}, func(tx *sql.Tx) error {
		var enc any
		if in.Credentials != nil {
			b, e := v.seal("target", in.ID, "", in.Credentials)
			if e != nil {
				return e
			}
			enc = b
		}
		_, e := tx.ExecContext(ctx, "INSERT INTO database_targets (id,display_name,canonical_host,port,tls_mode,state,managed_enabled,provisioning_credentials_enc) VALUES (?,?,?,?,?,?,?,?)", in.ID, in.Name, in.Host, in.Port, in.TLS, in.State, in.Managed, enc)
		if e != nil {
			return e
		}
		return auditAs(ctx, tx, a, "", "target.created", "database_target", in.ID, Row{})
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "database_targets", "", in.ID)
}

func (v *storage) UpdateTarget(ctx context.Context, a Actor, record string, in targetPatch, revision int64) (Ref, error) {
	if revision < 1 {
		return Ref{}, revisionRequired()
	}
	e := v.run.mutate(ctx, a, "platform.admin", Resource{}, func(tx *sql.Tx) error {
		old, e := lockRecord(ctx, tx, "database_targets", "", record)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(old["revision"]), revision); e != nil {
			return e
		}
		sets := []string{"revision=revision+1"}
		args := []any{}
		if in.Name != nil {
			if strings.TrimSpace(*in.Name) == "" || len(*in.Name) > 160 {
				return bad("Invalid name.")
			}
			sets = append(sets, "display_name=?")
			args = append(args, *in.Name)
		}
		if in.State != nil {
			if !enabledState(*in.State) {
				return bad("Invalid state.")
			}
			sets = append(sets, "state=?")
			args = append(args, *in.State)
		}
		if in.Managed != nil {
			sets = append(sets, "managed_enabled=?")
			args = append(args, *in.Managed)
		}
		if in.Credentials != nil {
			if in.Credentials.Username == "" || in.Credentials.Password == "" {
				return bad("Credentials cannot be empty.")
			}
			b, e := v.seal("target", record, "", in.Credentials)
			if e != nil {
				return e
			}
			sets = append(sets, "provisioning_credentials_enc=?")
			args = append(args, b)
		}
		args = append(args, record)
		if _, e = tx.ExecContext(ctx, "UPDATE database_targets SET "+strings.Join(sets, ",")+" WHERE id=?", args...); e != nil {
			return e
		}
		return auditAs(ctx, tx, a, "", "target.updated", "database_target", record, Row{})
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "database_targets", "", record)
}

func (v *storage) Databases(ctx context.Context, a Actor, tenant string, page PageRequest) (Page[Row], error) {
	if e := v.run.authorize(ctx, a, "database.read", Resource{Tenant: tenant}); e != nil {
		return Page[Row]{}, e
	}
	return pageQuery(ctx, v.db, page, collectionFor("database-resources"), "SELECT "+dbColumns+" FROM database_resources WHERE tenant_id=? ORDER BY created_at DESC", tenant)
}

func (v *storage) Database(ctx context.Context, a Actor, tenant, record string) (Row, error) {
	if e := v.run.authorize(ctx, a, "database.read", Resource{Tenant: tenant}); e != nil {
		return nil, e
	}
	return row(ctx, v.db, "SELECT "+dbColumns+" FROM database_resources WHERE tenant_id=? AND id=?", tenant, record)
}

// The Tenant mutation lock serializes allocation counts, including retries that
// claim an existing resource. Retrying the same resource does not use a new slot.
func agentDatabaseProvision(ctx context.Context, tx *sql.Tx, p Principal, resource, mode, target string) error {
	if p.Agent == nil {
		return nil
	}
	g := p.Agent
	if !agentHas(g, "databases.provision") || mode != "managed" || !slices.Contains(g.Limits.TargetIDs, target) {
		return problem(403, "needs_authorization", "Managed provisioning requires an approved target and databases.provision.")
	}
	var allocated int
	if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM database_resources WHERE agent_grant_id=? AND id<>?", g.ID, resource).Scan(&allocated); e != nil {
		return e
	}
	if allocated >= g.Limits.MaxDatabases {
		return problem(403, "agent_limit", "This agent has reached its database allocation limit.")
	}
	return nil
}

// allocate requests a database resource inside the caller's transaction. A
// workspace allocation is authorized by workspace.manage on its scope and is
// not counted against an agent's database allocation.
func (v *storage) allocate(ctx context.Context, tx *sql.Tx, a Actor, tenant string, in databaseInput, policy Row, workspace *Resource) error {
	action, res := "database.manage", Resource{Tenant: tenant}
	if workspace != nil {
		action, res = "workspace.manage", *workspace
	}
	if e := v.run.authorizeTx(ctx, tx, a, action, res); e != nil {
		return e
	}
	if e := id(in.ID); e != nil {
		return e
	}
	if e := id(in.Target); e != nil {
		return e
	}
	p := a.Principal
	if workspace == nil {
		if e := agentDatabaseProvision(ctx, tx, p, in.ID, in.Mode, in.Target); e != nil {
			return e
		}
	}
	if e := layoutAllowed(policy, in.Layout); e != nil {
		return e
	}
	if in.Mode != "managed" && in.Mode != "registered" {
		return bad("Select managed or registered database mode.")
	}
	if integer(policy["allow_"+in.Mode+"_database"]) != 1 {
		return problem(403, "plan_restriction", "This database method is not included in the plan.")
	}
	if e := quota(ctx, tx, tenant, "database_resources", "max_databases", policy); e != nil {
		return e
	}
	if e := pendingDatabaseQuota(ctx, tx, tenant); e != nil {
		return e
	}
	target, e := row(ctx, tx, "SELECT state,managed_enabled FROM database_targets WHERE id=?", in.Target)
	if e != nil {
		return e
	}
	if str(target["state"]) != stateActive {
		return conflict("The database target is disabled.")
	}
	var ownership any
	cred := in.Credentials
	if in.Mode == "managed" {
		if integer(target["managed_enabled"]) != 1 {
			return bad("Target does not support managed creation.")
		}
		in.Database = "la_" + strings.ReplaceAll(in.ID, "-", "")
		cred = credentials{Username: in.Database, Password: token(32), AccountHost: v.config.RuntimeAccountHost}
		if cred.AccountHost == "" {
			return bad("Runtime database account host is not configured.")
		}
		b := make([]byte, 16)
		if _, e = rand.Read(b); e != nil {
			return e
		}
		ownership = b
	} else {
		if !regexp.MustCompile(`^[a-zA-Z0-9_\-]{1,64}$`).MatchString(in.Database) || strings.EqualFold(in.Database, v.registryName) {
			return bad("Invalid or protected database name.")
		}
		if cred.Username == "" || cred.Password == "" {
			return bad("Existing database credentials are required.")
		}
	}
	enc, e := v.seal("database", in.ID, tenant, cred)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO database_resources (id,tenant_id,target_id,database_name,management_mode,layout,runtime_credentials_enc,ownership_token) VALUES (?,?,?,?,?,?,?,?)", in.ID, tenant, in.Target, in.Database, in.Mode, in.Layout, enc, ownership)
	if e != nil {
		return e
	}
	if p.Agent != nil {
		if _, e = tx.ExecContext(ctx, "UPDATE database_resources SET agent_grant_id=?,agent_user_id=? WHERE id=?", p.Agent.ID, p.ID, in.ID); e != nil {
			return e
		}
	}
	return auditAs(ctx, tx, a, tenant, "database.requested", "database", in.ID, Row{"management_mode": in.Mode, "layout": in.Layout})
}

// CreateDatabase queues a managed allocation or registers an existing database.
func (v *storage) CreateDatabase(ctx context.Context, a Actor, tenant string, in databaseInput) (Ref, error) {
	e := v.run.mutate(ctx, a, "database.manage", Resource{Tenant: tenant}, func(tx *sql.Tx) error {
		policy, e := policyFor(ctx, tx, tenant)
		if e != nil {
			return e
		}
		return v.allocate(ctx, tx, a, tenant, in, policy, nil)
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "database_resources", tenant, in.ID)
}

// RetryDatabase requeues a failed or ready resource under its revision.
func (v *storage) RetryDatabase(ctx context.Context, a Actor, tenant, record string, revision int64) (Ref, error) {
	return v.changeDatabase(ctx, a, tenant, record, databaseChange{}, true, revision)
}

// UpdateDatabase retires a resource or replaces registered credentials.
func (v *storage) UpdateDatabase(ctx context.Context, a Actor, tenant, record string, in databaseChange, revision int64) (Ref, error) {
	return v.changeDatabase(ctx, a, tenant, record, in, false, revision)
}

func (v *storage) changeDatabase(ctx context.Context, a Actor, tenant, record string, in databaseChange, retry bool, revision int64) (Ref, error) {
	if revision < 1 {
		return Ref{}, revisionRequired()
	}
	p := a.Principal
	e := v.run.mutate(ctx, a, "database.manage", Resource{Tenant: tenant}, func(tx *sql.Tx) error {
		old, e := row(ctx, tx, "SELECT revision,state,management_mode,target_id FROM database_resources WHERE tenant_id=? AND id=? FOR UPDATE", tenant, record)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(old["revision"]), revision); e != nil {
			return e
		}
		state := str(old["state"])
		if state == "provisioning" || state == "pending" || state == stateRetired {
			return conflict("Wait for provisioning to finish; retired resources cannot be changed.")
		}
		next := "pending"
		if !retry && in.Credentials == nil {
			if in.State != stateRetired {
				return bad("Only retirement or registered credential replacement is supported.")
			}
			var n int
			if e = tx.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM spaces WHERE database_resource_id=?)+(SELECT COUNT(*) FROM tenants WHERE default_shared_database_id=?)", record, record).Scan(&n); e != nil {
				return e
			}
			if n > 0 {
				return conflict("A bound or default database cannot be retired.")
			}
			next = stateRetired
		}
		if next == "pending" {
			if e := agentDatabaseProvision(ctx, tx, p, record, str(old["management_mode"]), str(old["target_id"])); e != nil {
				return e
			}
			if e := pendingDatabaseQuota(ctx, tx, tenant); e != nil {
				return e
			}
		}
		if in.Credentials != nil {
			if str(old["management_mode"]) != "registered" || in.Credentials.Username == "" || in.Credentials.Password == "" {
				return bad("Only registered database credentials can be replaced.")
			}
			enc, e := v.seal("database", record, tenant, in.Credentials)
			if e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, "UPDATE database_resources SET runtime_credentials_enc=?,credential_generation=credential_generation+1 WHERE id=?", enc, record); e != nil {
				return e
			}
		}
		if next == "pending" {
			var grant, user any
			if p.Agent != nil {
				grant, user = p.Agent.ID, p.ID
			}
			// Requeued work belongs to its current authorizer. A human retry
			// explicitly takes ownership from any old, possibly revoked grant.
			if _, e = tx.ExecContext(ctx, "UPDATE database_resources SET agent_grant_id=?,agent_user_id=? WHERE id=?", grant, user, record); e != nil {
				return e
			}
		}
		_, e = tx.ExecContext(ctx, "UPDATE database_resources SET state=?,last_error_code=NULL,last_error_message=NULL,revision=revision+1 WHERE id=?", next, record)
		if e != nil {
			return e
		}
		return auditAs(ctx, tx, a, tenant, "database."+next, "database", record, Row{})
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "database_resources", tenant, record)
}

func (s *Server) targets(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.storage.Targets(r.Context(), httpActor(r, p), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) availableTargets(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.storage.AvailableTargets(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.URL.Query().Get("managed") == "true", pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createTarget(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in targetInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.storage.CreateTarget(r.Context(), httpActor(r, p), in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

func (s *Server) updateTarget(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in targetPatch
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.storage.UpdateTarget(r.Context(), httpActor(r, p), r.PathValue("id"), in, ifMatch(r))
	if e == nil {
		sendRef(w, 200, v)
	}
	return e
}

func (s *Server) databases(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.storage.Databases(r.Context(), httpActor(r, p), r.PathValue("tenant"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) getDatabase(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.storage.Database(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createDatabase(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in databaseInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.storage.CreateDatabase(r.Context(), httpActor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		sendRef(w, 202, v)
	}
	return e
}

func (s *Server) retryDatabase(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.storage.RetryDatabase(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), ifMatch(r))
	if e == nil {
		sendRef(w, 202, v)
	}
	return e
}

func (s *Server) updateDatabase(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in databaseChange
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.storage.UpdateDatabase(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), in, ifMatch(r))
	if e == nil {
		sendRef(w, 202, v)
	}
	return e
}

func (s *Server) targetDB(ctx context.Context, target Row, cred credentials, name string) (*sql.DB, error) {
	host := str(target["canonical_host"])
	port := strconv.Itoa(int(integer(target["port"])))
	cfg := mariadb.NewConfig()
	cfg.User = cred.Username
	cfg.Passwd = cred.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(host, port)
	cfg.DBName = name
	cfg.ParseTime = true
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 15 * time.Second
	cfg.WriteTimeout = 15 * time.Second
	if str(target["tls_mode"]) == "verify_identity" {
		cfg.TLS = &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	} else if !s.Config.Development {
		return nil, fmt.Errorf("TLS is required")
	}
	cfg.DialFunc = func(c context.Context, network, address string) (net.Conn, error) {
		ips, e := net.DefaultResolver.LookupIPAddr(c, host)
		if e != nil {
			return nil, e
		}
		for _, ip := range ips {
			allowed := !ip.IP.IsPrivate() && !ip.IP.IsLoopback() && !ip.IP.IsUnspecified() && !ip.IP.IsLinkLocalUnicast() && !ip.IP.IsMulticast()
			if s.Config.Development {
				allowed = ip.IP.IsLoopback()
			}
			for _, cidr := range s.Config.AllowedNetworks {
				_, sub, _ := net.ParseCIDR(cidr)
				if sub != nil && sub.Contains(ip.IP) {
					allowed = true
				}
			}
			if !allowed {
				return nil, fmt.Errorf("database address is not allowed")
			}
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no database address")
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(c, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
	connector, e := mariadb.NewConnector(cfg)
	if e != nil {
		return nil, e
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, e
	}
	return db, nil
}

func pendingDatabaseQuota(ctx context.Context, q Queryer, tenant string) error {
	var pending int
	if e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM database_resources WHERE tenant_id=? AND state IN ('pending','provisioning')", tenant).Scan(&pending); e != nil {
		return e
	}
	if pending >= 8 {
		return problem(409, "pending_limit", "Wait for pending database requests before queuing more.")
	}
	return nil
}
