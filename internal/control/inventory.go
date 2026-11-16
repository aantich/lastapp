package control

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

type namedInput struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"display_name"`
}
type metadataPatch struct {
	Name     *string `json:"display_name"`
	State    *string `json:"state"`
	Compute  *string `json:"compute_mode"`
	Artifact *string `json:"artifact_ref"`
	Order    *int    `json:"navigation_order"`
}
type spaceCreateInput struct {
	namedInput
	Manager  string `json:"initial_manager_user_id,omitempty"`
	Database struct {
		Resource string `json:"resource_id,omitempty"`
		Mode     string `json:"mode,omitempty"`
		Layout   string `json:"layout,omitempty"`
		Target   string `json:"target_id,omitempty"`
	} `json:"database"`
}
type appCreateInput struct {
	namedInput
	Manager string `json:"initial_manager_user_id,omitempty"`
	Compute string `json:"compute_mode,omitempty"`
}
type microappCreateInput struct {
	namedInput
	Artifact string `json:"artifact_ref,omitempty"`
	Order    int    `json:"navigation_order,omitempty"`
}

// inventory owns the Tenant inventory operations: plans, Tenants, Spaces,
// Apps and micro-apps. Console handlers and MCP tools both call these methods.
type inventory struct {
	db  *sql.DB
	run *op.Runner
	// Collaborators of domains not yet extracted, passed explicitly.
	allocateDatabase func(ctx context.Context, tx *sql.Tx, a op.Actor, tenant string, in databaseInput, policy Policy, workspace *access.Resource) error
	prepareAccount   func(accountInput) (preparedAccount, error)
}

// Space is a Space read with its Tenant's name and its database binding.
type Space struct {
	DatabaseLayout   string  `json:"database_layout"`
	DatabaseName     string  `json:"database_name"`
	DatabaseResource string  `json:"database_resource_id"`
	DatabaseState    string  `json:"database_state"`
	Name             string  `json:"display_name"`
	ID               string  `json:"id"`
	Revision         int64   `json:"revision"`
	Slug             string  `json:"slug"`
	State            string  `json:"state"`
	TenantID         string  `json:"tenant_id"`
	TenantName       string  `json:"tenant_name"`
	WorkspaceID      *string `json:"workspace_id"`
}

func (v *Space) ScanFields() []any {
	return []any{&v.DatabaseLayout, &v.DatabaseName, &v.DatabaseResource, &v.DatabaseState, &v.Name, &v.ID, &v.Revision, &v.Slug, &v.State, &v.TenantID, &v.TenantName, &v.WorkspaceID}
}

// SpaceItem is a production Space collection row with its active App count.
type SpaceItem struct {
	AppCount         int64     `json:"app_count"`
	CreatedAt        time.Time `json:"created_at"`
	DatabaseLayout   string    `json:"database_layout"`
	DatabaseName     string    `json:"database_name"`
	DatabaseResource string    `json:"database_resource_id"`
	DatabaseState    string    `json:"database_state"`
	Name             string    `json:"display_name"`
	ID               string    `json:"id"`
	Revision         int64     `json:"revision"`
	Slug             string    `json:"slug"`
	State            string    `json:"state"`
	TenantID         string    `json:"tenant_id"`
}

func (v *SpaceItem) ScanFields() []any {
	return []any{&v.AppCount, &v.CreatedAt, &v.DatabaseLayout, &v.DatabaseName, &v.DatabaseResource, &v.DatabaseState, &v.Name, &v.ID, &v.Revision, &v.Slug, &v.State, &v.TenantID}
}

func (v *inventory) Spaces(ctx context.Context, a op.Actor, tenant string, page op.PageRequest) (op.Page[SpaceItem], error) {
	sc, e := v.run.Scopes(ctx, a, tenant, "space.read")
	if e != nil {
		return op.Page[SpaceItem]{}, e
	}
	filter, args := access.ScopeSQL(sc, "sp.id", "")
	args = append([]any{tenant}, args...)
	return op.PageScan[SpaceItem](ctx, v.db, page, op.CollectionFor("spaces"), "SELECT (SELECT COUNT(*) FROM apps a WHERE a.space_id=sp.id AND a.state<>'archived') AS app_count,sp.created_at,sp.database_layout,d.database_name,sp.database_resource_id,d.state AS database_state,sp.display_name,sp.id,sp.revision,sp.slug,sp.state,sp.tenant_id FROM spaces sp JOIN database_resources d ON d.id=sp.database_resource_id WHERE sp.tenant_id=? AND sp.workspace_id IS NULL AND "+filter+" ORDER BY sp.created_at DESC", args...)
}

func (v *inventory) Space(ctx context.Context, a op.Actor, tenant, space string) (Space, error) {
	if e := v.run.Authorize(ctx, a, "space.read", access.Resource{Tenant: tenant, Space: space}); e != nil {
		return Space{}, e
	}
	return registry.ScanOne[Space](ctx, v.db, "SELECT sp.database_layout,d.database_name,sp.database_resource_id,d.state AS database_state,sp.display_name,sp.id,sp.revision,sp.slug,sp.state,sp.tenant_id,t.display_name AS tenant_name,sp.workspace_id FROM spaces sp JOIN tenants t ON t.id=sp.tenant_id JOIN database_resources d ON d.id=sp.database_resource_id WHERE sp.tenant_id=? AND sp.id=?", tenant, space)
}

// CreateSpace binds a new Space to an existing database or allocates managed
// storage (reusing the Tenant's default shared database), then assigns its
// initial manager, in one transaction.
func (v *inventory) CreateSpace(ctx context.Context, a op.Actor, tenant string, in spaceCreateInput) (op.Ref, error) {
	if e := registry.ID(in.ID); e != nil {
		return op.Ref{}, e
	}
	if e := registry.Named(in.Slug, in.Name); e != nil {
		return op.Ref{}, e
	}
	e := v.run.Mutate(ctx, a, "space.manage", access.Resource{Tenant: tenant}, func(tx *sql.Tx) error {
		// Detect a retried parent create before allocating any child database.
		var exists int
		if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM spaces WHERE id=?", in.ID).Scan(&exists); e != nil {
			return e
		}
		if exists > 0 {
			return registry.Conflict("This Space ID already exists; refresh before retrying.")
		}
		policy, e := policyFor(ctx, tx, tenant)
		if e != nil {
			return e
		}
		if e = quota(ctx, tx, tenant, "spaces", policy.MaxSpaces); e != nil {
			return e
		}
		resource, layout := in.Database.Resource, in.Database.Layout
		if resource != "" {
			var private int
			if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM spaces WHERE database_resource_id=? AND workspace_id IS NOT NULL", resource).Scan(&private); err != nil {
				return err
			}
			if private > 0 {
				return registry.Bad("Development storage cannot be bound to a production Space.")
			}
			var state string
			if e := registry.Lookup(ctx, tx, "SELECT layout,state FROM database_resources WHERE tenant_id=? AND id=?", []any{tenant, resource}, &layout, &state); e != nil {
				return e
			}
			if state == registry.StateRetired {
				return registry.Conflict("A retired database cannot be bound.")
			}
			if e = layoutAllowed(policy.Plan, layout); e != nil {
				return e
			}
		} else {
			if in.Database.Mode != "managed" {
				return registry.Bad("Select an existing database or request managed creation.")
			}
			if layout == "" {
				layout = policy.TenantLayout
				if layout == "" || layoutAllowed(policy.Plan, layout) != nil {
					layout = policy.DefaultLayout
				}
			}
			if e = layoutAllowed(policy.Plan, layout); e != nil {
				return e
			}
			if layout == "shared" && policy.DefaultSharedDatabase != "" {
				resource = policy.DefaultSharedDatabase
				var target, state string
				if e := registry.Lookup(ctx, tx, "SELECT target_id,state FROM database_resources WHERE id=? AND tenant_id=?", []any{resource, tenant}, &target, &state); e != nil {
					return e
				}
				if state == registry.StateRetired {
					return registry.Conflict("The default database is retired.")
				}
				if in.Database.Target != "" && in.Database.Target != target {
					return registry.Conflict("The selected target differs from the existing default shared database.")
				}
			} else {
				if in.Database.Target == "" {
					e := tx.QueryRowContext(ctx, "SELECT id FROM database_targets WHERE state='active' AND managed_enabled=1 ORDER BY created_at,id LIMIT 1").Scan(&in.Database.Target)
					if errors.Is(e, sql.ErrNoRows) {
						return registry.Conflict("Managed database creation is not configured. Ask a platform administrator to add a database server.")
					}
					if e != nil {
						return e
					}
				}
				resource = registry.NewID()
				dbIn := databaseInput{ID: resource, Mode: "managed", Layout: layout, Target: in.Database.Target}
				if e = v.allocateDatabase(ctx, tx, a, tenant, dbIn, policy, nil); e != nil {
					return e
				}
				if layout == "shared" {
					if _, e = tx.ExecContext(ctx, "UPDATE tenants SET default_shared_database_id=?,revision=revision+1 WHERE id=?", resource, tenant); e != nil {
						return e
					}
				}
			}
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO spaces (id,tenant_id,slug,display_name,database_resource_id,database_layout) VALUES (?,?,?,?,?,?)", in.ID, tenant, in.Slug, in.Name, resource, layout)
		if e != nil {
			return e
		}
		if e := assignInitialManager(ctx, tx, a, tenant, in.ID, "", in.Manager); e != nil {
			return e
		}
		return op.AuditAs(ctx, tx, a, tenant, "space.created", "space", in.ID, registry.Row{"database_resource_id": resource})
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.run.Ref(ctx, "spaces", tenant, in.ID)
}

// App is an App read with its Tenant and Space names.
type App struct {
	ActiveRelease *string `json:"active_release_id"`
	Compute       string  `json:"compute_mode"`
	Name          string  `json:"display_name"`
	ID            string  `json:"id"`
	Revision      int64   `json:"revision"`
	Slug          string  `json:"slug"`
	SpaceID       string  `json:"space_id"`
	SpaceName     string  `json:"space_name"`
	State         string  `json:"state"`
	TenantID      string  `json:"tenant_id"`
	TenantName    string  `json:"tenant_name"`
	WorkspaceID   *string `json:"workspace_id"`
}

func (v *App) ScanFields() []any {
	return []any{&v.ActiveRelease, &v.Compute, &v.Name, &v.ID, &v.Revision, &v.Slug, &v.SpaceID, &v.SpaceName, &v.State, &v.TenantID, &v.TenantName, &v.WorkspaceID}
}

// AppItem is an App collection row with its active micro-app count.
type AppItem struct {
	Compute       string    `json:"compute_mode"`
	CreatedAt     time.Time `json:"created_at"`
	Name          string    `json:"display_name"`
	ID            string    `json:"id"`
	MicroappCount int64     `json:"microapp_count"`
	Revision      int64     `json:"revision"`
	Slug          string    `json:"slug"`
	SpaceID       string    `json:"space_id"`
	SpaceName     string    `json:"space_name"`
	State         string    `json:"state"`
	TenantID      string    `json:"tenant_id"`
	WorkspaceID   *string   `json:"workspace_id"`
}

func (v *AppItem) ScanFields() []any {
	return []any{&v.Compute, &v.CreatedAt, &v.Name, &v.ID, &v.MicroappCount, &v.Revision, &v.Slug, &v.SpaceID, &v.SpaceName, &v.State, &v.TenantID, &v.WorkspaceID}
}

// Apps pages the Apps a may read in tenant, optionally of one Space. Without a
// Space filter, development workspace Apps are excluded.
func (v *inventory) Apps(ctx context.Context, a op.Actor, tenant, space string, page op.PageRequest) (op.Page[AppItem], error) {
	sc, e := v.run.Scopes(ctx, a, tenant, "app.read")
	if e != nil {
		return op.Page[AppItem]{}, e
	}
	filter, args := access.ScopeSQL(sc, "a.space_id", "a.id")
	args = append([]any{tenant}, args...)
	query := "SELECT a.compute_mode,a.created_at,a.display_name,a.id,(SELECT COUNT(*) FROM microapp_installations m WHERE m.app_id=a.id AND m.state<>'archived') AS microapp_count,a.revision,a.slug,a.space_id,sp.display_name AS space_name,a.state,a.tenant_id,sp.workspace_id FROM apps a JOIN spaces sp ON sp.id=a.space_id WHERE a.tenant_id=? AND " + filter
	if space != "" {
		if e = v.run.Authorize(ctx, a, "app.read", access.Resource{Tenant: tenant, Space: space}); e != nil {
			return op.Page[AppItem]{}, e
		}
		query += " AND a.space_id=?"
		args = append(args, space)
	} else {
		query += " AND sp.workspace_id IS NULL"
	}
	return op.PageScan[AppItem](ctx, v.db, page, op.CollectionFor("apps"), query+" ORDER BY a.created_at DESC", args...)
}

func (v *inventory) App(ctx context.Context, a op.Actor, tenant, app string) (App, error) {
	if e := v.run.Authorize(ctx, a, "app.read", access.Resource{Tenant: tenant, App: app}); e != nil {
		return App{}, e
	}
	return registry.ScanOne[App](ctx, v.db, "SELECT a.active_release_id,a.compute_mode,a.display_name,a.id,a.revision,a.slug,a.space_id,sp.display_name AS space_name,a.state,a.tenant_id,t.display_name AS tenant_name,sp.workspace_id FROM apps a JOIN tenants t ON t.id=a.tenant_id JOIN spaces sp ON sp.id=a.space_id WHERE a.tenant_id=? AND a.id=?", tenant, app)
}

func (v *inventory) CreateApp(ctx context.Context, a op.Actor, tenant, space string, in appCreateInput) (op.Ref, error) {
	if e := registry.ID(in.ID); e != nil {
		return op.Ref{}, e
	}
	if e := registry.Named(in.Slug, in.Name); e != nil {
		return op.Ref{}, e
	}
	if in.Compute == "" {
		in.Compute = "shared"
	}
	if in.Compute != "shared" && in.Compute != "dedicated" {
		return op.Ref{}, registry.Bad("Invalid compute mode.")
	}
	e := v.run.Mutate(ctx, a, "app.manage", access.Resource{Tenant: tenant, Space: space}, func(tx *sql.Tx) error {
		var parent string
		if e := registry.Lookup(ctx, tx, "SELECT state FROM spaces WHERE id=? AND tenant_id=?", []any{space, tenant}, &parent); e != nil {
			return e
		}
		if parent != registry.StateActive {
			return registry.Conflict("The parent Space must be active.")
		}
		policy, e := policyFor(ctx, tx, tenant)
		if e != nil {
			return e
		}
		if e = appQuota(ctx, tx, tenant, space, policy, true); e != nil {
			return e
		}
		if in.Compute == "dedicated" && !policy.AllowDedicatedCompute {
			return registry.Problem(403, "plan_restriction", "Dedicated compute is not permitted by this plan.")
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO apps (id,tenant_id,space_id,slug,display_name,compute_mode) VALUES (?,?,?,?,?,?)", in.ID, tenant, space, in.Slug, in.Name, in.Compute)
		if e != nil {
			return e
		}
		if e := assignInitialManager(ctx, tx, a, tenant, space, in.ID, in.Manager); e != nil {
			return e
		}
		return op.AuditAs(ctx, tx, a, tenant, "app.created", "app", in.ID, registry.Row{"space_id": space})
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.run.Ref(ctx, "apps", tenant, in.ID)
}

func (s *Server) spaces(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Spaces(r.Context(), httpActor(r, p), r.PathValue("tenant"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) getSpace(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Space(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("space"))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createSpace(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in spaceCreateInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.CreateSpace(r.Context(), httpActor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

func (s *Server) apps(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Apps(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.URL.Query().Get("space_id"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) getApp(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.App(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("app"))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createApp(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in appCreateInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.CreateApp(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("space"), in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

// Microapp is an installed micro-app. AppName, CreatedAt and SpaceID are read
// only in collections (microappItem).
type Microapp struct {
	AppID     string     `json:"app_id"`
	AppName   *string    `json:"app_name,omitempty"`
	Artifact  *string    `json:"artifact_ref"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	Name      string     `json:"display_name"`
	ID        string     `json:"id"`
	Order     int64      `json:"navigation_order"`
	Revision  int64      `json:"revision"`
	Slug      string     `json:"slug"`
	SpaceID   *string    `json:"space_id,omitempty"`
	State     string     `json:"state"`
	TenantID  string     `json:"tenant_id"`
}

const microappColumns = "m.app_id,m.artifact_ref,m.display_name,m.id,m.navigation_order,m.revision,m.slug,m.state,m.tenant_id"

func (m *Microapp) ScanFields() []any {
	return []any{&m.AppID, &m.Artifact, &m.Name, &m.ID, &m.Order, &m.Revision, &m.Slug, &m.State, &m.TenantID}
}

// microappItem is a micro-app collection row: with its App's name and Space.
type microappItem struct{ Microapp }

func (m *microappItem) ScanFields() []any {
	return append(m.Microapp.ScanFields(), &m.AppName, &m.CreatedAt, &m.SpaceID)
}

// Microapps pages the micro-apps a may read in tenant, optionally of one App.
// Without an App filter, development workspace Apps are excluded.
func (v *inventory) Microapps(ctx context.Context, a op.Actor, tenant, app string, page op.PageRequest) (op.Page[microappItem], error) {
	sc, e := v.run.Scopes(ctx, a, tenant, "microapp.read")
	if e != nil {
		return op.Page[microappItem]{}, e
	}
	filter, args := access.ScopeSQL(sc, "a.space_id", "m.app_id")
	args = append([]any{tenant}, args...)
	query := "SELECT " + microappColumns + ",a.display_name AS app_name,m.created_at,a.space_id FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE m.tenant_id=? AND " + filter
	if app != "" {
		if e = v.run.Authorize(ctx, a, "microapp.read", access.Resource{Tenant: tenant, App: app}); e != nil {
			return op.Page[microappItem]{}, e
		}
		query += " AND m.app_id=?"
		args = append(args, app)
	} else {
		query += " AND NOT EXISTS(SELECT 1 FROM spaces dsp WHERE dsp.id=a.space_id AND dsp.workspace_id IS NOT NULL)"
	}
	return op.PageScan[microappItem](ctx, v.db, page, op.CollectionFor("microapps"), query+" ORDER BY m.navigation_order,m.created_at DESC", args...)
}

func (v *inventory) Microapp(ctx context.Context, a op.Actor, tenant, record string) (Microapp, error) {
	m, e := registry.ScanOne[Microapp](ctx, v.db, "SELECT "+microappColumns+" FROM microapp_installations m WHERE m.tenant_id=? AND m.id=?", tenant, record)
	if e != nil {
		return Microapp{}, e
	}
	if e = v.run.Authorize(ctx, a, "microapp.read", access.Resource{Tenant: tenant, App: m.AppID}); e != nil {
		return Microapp{}, e
	}
	return m, nil
}

func (v *inventory) CreateMicroapp(ctx context.Context, a op.Actor, tenant, app string, in microappCreateInput) (op.Ref, error) {
	if e := registry.ID(in.ID); e != nil {
		return op.Ref{}, e
	}
	if e := registry.Named(in.Slug, in.Name); e != nil {
		return op.Ref{}, e
	}
	if len(in.Artifact) > 1024 {
		return op.Ref{}, registry.Bad("Artifact reference is too long.")
	}
	res := access.Resource{Tenant: tenant, App: app}
	if e := registry.Lookup(ctx, v.db, "SELECT space_id FROM apps WHERE id=? AND tenant_id=?", []any{app, tenant}, &res.Space); e != nil {
		return op.Ref{}, e
	}
	e := v.run.Mutate(ctx, a, "microapp.manage", res, func(tx *sql.Tx) error {
		if _, e := policyFor(ctx, tx, tenant); e != nil {
			return e
		}
		if e := metadataQuota(ctx, tx, tenant, "microapp_installations"); e != nil {
			return e
		}
		var appState, spaceState string
		e := registry.Lookup(ctx, tx, "SELECT a.state,sp.state FROM apps a JOIN spaces sp ON sp.id=a.space_id WHERE a.id=? AND a.tenant_id=?", []any{app, tenant}, &appState, &spaceState)
		if e != nil {
			return e
		}
		if appState != registry.StateActive || spaceState != registry.StateActive {
			return registry.Conflict("The parent App and Space must be active.")
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO microapp_installations (id,tenant_id,app_id,slug,display_name,artifact_ref,navigation_order) VALUES (?,?,?,?,?,?,?)", in.ID, tenant, app, in.Slug, in.Name, registry.Nullable(in.Artifact), in.Order)
		if e != nil {
			return e
		}
		return op.AuditAs(ctx, tx, a, tenant, "microapp.created", "microapp", in.ID, registry.Row{"app_id": app})
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.run.Ref(ctx, "microapp_installations", tenant, in.ID)
}

func (v *inventory) UpdateSpace(ctx context.Context, a op.Actor, tenant, record string, in metadataPatch, revision int64) (op.Ref, error) {
	return v.patch(ctx, a, "spaces", "space.manage", access.Resource{Tenant: tenant, Space: record}, record, in, revision)
}

func (v *inventory) UpdateApp(ctx context.Context, a op.Actor, tenant, record string, in metadataPatch, revision int64) (op.Ref, error) {
	res := access.Resource{Tenant: tenant, App: record}
	if e := registry.Lookup(ctx, v.db, "SELECT space_id FROM apps WHERE id=? AND tenant_id=?", []any{record, tenant}, &res.Space); e != nil {
		return op.Ref{}, e
	}
	return v.patch(ctx, a, "apps", "app.manage", res, record, in, revision)
}

func (v *inventory) UpdateMicroapp(ctx context.Context, a op.Actor, tenant, record string, in metadataPatch, revision int64) (op.Ref, error) {
	res := access.Resource{Tenant: tenant}
	if e := registry.Lookup(ctx, v.db, "SELECT a.space_id,m.app_id FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE m.id=? AND m.tenant_id=?", []any{record, tenant}, &res.Space, &res.App); e != nil {
		return op.Ref{}, e
	}
	return v.patch(ctx, a, "microapp_installations", "microapp.manage", res, record, in, revision)
}

// patch changes Space, App or micro-app metadata and lifecycle state under the
// record's revision. Restoring an archived Space or App rechecks plan quotas.
func (v *inventory) patch(ctx context.Context, a op.Actor, table, action string, res access.Resource, record string, in metadataPatch, revision int64) (op.Ref, error) {
	if revision < 1 {
		return op.Ref{}, op.RevisionRequired()
	}
	e := v.run.Mutate(ctx, a, action, res, func(tx *sql.Tx) error {
		old, e := lockRecord(ctx, tx, table, res.Tenant, record)
		if e != nil {
			return e
		}
		if e = op.CheckRevision(old.Revision, revision); e != nil {
			return e
		}
		if table != "spaces" {
			var parent string
			if e := registry.Lookup(ctx, tx, "SELECT state FROM spaces WHERE id=? AND tenant_id=?", []any{res.Space, res.Tenant}, &parent); e != nil {
				return e
			}
			if parent != registry.StateActive {
				return registry.Conflict("The parent Space must be active.")
			}
		}
		if table == "microapp_installations" {
			var app string
			if e := registry.Lookup(ctx, tx, "SELECT state FROM apps WHERE id=? AND tenant_id=?", []any{res.App, res.Tenant}, &app); e != nil {
				return e
			}
			if app != registry.StateActive {
				return registry.Conflict("The parent App must be active.")
			}
		}
		sets := []string{"revision=revision+1"}
		args := []any{}
		if in.Name != nil {
			if strings.TrimSpace(*in.Name) == "" || len(*in.Name) > 160 {
				return registry.Bad("Invalid name.")
			}
			sets = append(sets, "display_name=?")
			args = append(args, *in.Name)
		}
		if in.State != nil {
			if !registry.ValidState(*in.State) {
				return registry.Bad("Invalid state.")
			}
			if old.State == registry.StateArchived && *in.State != registry.StateArchived && (table == "apps" || table == "spaces") {
				policy, e := policyFor(ctx, tx, res.Tenant)
				if e != nil {
					return e
				}
				if table == "apps" {
					e = appQuota(ctx, tx, res.Tenant, res.Space, policy, false)
				} else {
					e = quota(ctx, tx, res.Tenant, table, policy.MaxSpaces)
				}
				if e != nil {
					return e
				}
			}
			sets = append(sets, "state=?")
			args = append(args, *in.State)
		}
		if in.Compute != nil {
			if table != "apps" {
				return registry.Bad("Compute mode applies only to Apps.")
			}
			if *in.Compute != "shared" && *in.Compute != "dedicated" {
				return registry.Bad("Invalid compute mode.")
			}
			var previous string
			if e = tx.QueryRowContext(ctx, "SELECT compute_mode FROM apps WHERE id=? AND tenant_id=?", record, res.Tenant).Scan(&previous); e != nil {
				return e
			}
			if *in.Compute == "dedicated" && previous != *in.Compute {
				policy, e := policyFor(ctx, tx, res.Tenant)
				if e != nil {
					return e
				}
				if !policy.AllowDedicatedCompute {
					return registry.Problem(403, "plan_restriction", "Dedicated compute is not permitted.")
				}
			}
			sets = append(sets, "compute_mode=?")
			args = append(args, *in.Compute)
		}
		if in.Artifact != nil || in.Order != nil {
			if table != "microapp_installations" {
				return registry.Bad("Artifact/navigation fields apply only to micro-apps.")
			}
			if in.Artifact != nil {
				if len(*in.Artifact) > 1024 {
					return registry.Bad("Artifact reference is too long.")
				}
				sets = append(sets, "artifact_ref=?")
				args = append(args, registry.Nullable(*in.Artifact))
			}
			if in.Order != nil {
				sets = append(sets, "navigation_order=?")
				args = append(args, *in.Order)
			}
		}
		args = append(args, record, res.Tenant)
		if _, e = tx.ExecContext(ctx, "UPDATE "+table+" SET "+strings.Join(sets, ",")+" WHERE id=? AND tenant_id=?", args...); e != nil {
			return e
		}
		return op.AuditAs(ctx, tx, a, res.Tenant, table+".updated", table, record, registry.Row{"state": in.State})
	})
	if e != nil {
		return op.Ref{}, e
	}
	return v.run.Ref(ctx, table, res.Tenant, record)
}

func (s *Server) microapps(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Microapps(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.URL.Query().Get("app_id"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createMicroapp(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in microappCreateInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.CreateMicroapp(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("app"), in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

func (s *Server) updateSpace(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	return updateInventory(w, r, p, s.inv.UpdateSpace)
}

func (s *Server) updateApp(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	return updateInventory(w, r, p, s.inv.UpdateApp)
}

func (s *Server) updateMicroapp(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	return updateInventory(w, r, p, s.inv.UpdateMicroapp)
}

// updateInventory is the console adapter of the Space, App and micro-app
// metadata and lifecycle operations.
func updateInventory(w http.ResponseWriter, r *http.Request, p access.Principal, update func(context.Context, op.Actor, string, string, metadataPatch, int64) (op.Ref, error)) error {
	var in metadataPatch
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := update(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), in, ifMatch(r))
	if e == nil {
		sendRef(w, 200, v)
	}
	return e
}
