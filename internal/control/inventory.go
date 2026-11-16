package control

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
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

func (s *Server) spaces(w http.ResponseWriter, r *http.Request, p Principal) error {
	tenant := r.PathValue("tenant")
	sc, e := s.scopes(r.Context(), s.DB, p, tenant, "space.read")
	if e != nil {
		return e
	}
	filter, args := scopeSQL(sc, "sp.id", "")
	args = append([]any{tenant}, args...)
	v, e := pageRows(r.Context(), s.DB, r, "SELECT sp.id,sp.tenant_id,sp.slug,sp.display_name,sp.database_resource_id,sp.database_layout,sp.state,sp.revision,sp.created_at,d.database_name,d.state AS database_state,(SELECT COUNT(*) FROM apps a WHERE a.space_id=sp.id AND a.state<>'archived') AS app_count FROM spaces sp JOIN database_resources d ON d.id=sp.database_resource_id WHERE sp.tenant_id=? AND sp.workspace_id IS NULL AND "+filter+" ORDER BY sp.created_at DESC", args...)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}

func (s *Server) createSpace(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in spaceCreateInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if e := id(in.ID); e != nil {
		return e
	}
	if e := named(in.Slug, in.Name); e != nil {
		return e
	}
	tenant := r.PathValue("tenant")
	e := s.mutate(r.Context(), p, "space.manage", Resource{Tenant: tenant}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		// Detect a retried parent create before allocating any child database.
		var exists int
		if e := tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM spaces WHERE id=?", in.ID).Scan(&exists); e != nil {
			return e
		}
		if exists > 0 {
			return conflict("This Space ID already exists; refresh before retrying.")
		}
		policy, e := policyFor(r.Context(), tx, tenant)
		if e != nil {
			return e
		}
		if e = quota(r.Context(), tx, tenant, "spaces", "max_spaces", policy); e != nil {
			return e
		}
		resource, layout := in.Database.Resource, in.Database.Layout
		if resource != "" {
			var private int
			if err := tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM spaces WHERE database_resource_id=? AND workspace_id IS NOT NULL", resource).Scan(&private); err != nil {
				return err
			}
			if private > 0 {
				return bad("Development storage cannot be bound to a production Space.")
			}
			db, e := row(r.Context(), tx, "SELECT id,layout,state FROM database_resources WHERE tenant_id=? AND id=?", tenant, resource)
			if e != nil {
				return e
			}
			if str(db["state"]) == stateRetired {
				return conflict("A retired database cannot be bound.")
			}
			layout = str(db["layout"])
			if e = layoutAllowed(policy, layout); e != nil {
				return e
			}
		} else {
			if in.Database.Mode != "managed" {
				return bad("Select an existing database or request managed creation.")
			}
			if layout == "" {
				layout = str(policy["tenant_default_layout"])
				if layout == "" || layoutAllowed(policy, layout) != nil {
					layout = str(policy["default_layout"])
				}
			}
			if e = layoutAllowed(policy, layout); e != nil {
				return e
			}
			if layout == "shared" && str(policy["default_shared_database_id"]) != "" {
				resource = str(policy["default_shared_database_id"])
				db, e := row(r.Context(), tx, "SELECT target_id,state FROM database_resources WHERE id=? AND tenant_id=?", resource, tenant)
				if e != nil {
					return e
				}
				if str(db["state"]) == stateRetired {
					return conflict("The default database is retired.")
				}
				if in.Database.Target != "" && in.Database.Target != str(db["target_id"]) {
					return conflict("The selected target differs from the existing default shared database.")
				}
			} else {
				if in.Database.Target == "" {
					e := tx.QueryRowContext(r.Context(), "SELECT id FROM database_targets WHERE state='active' AND managed_enabled=1 ORDER BY created_at,id LIMIT 1").Scan(&in.Database.Target)
					if errors.Is(e, sql.ErrNoRows) {
						return conflict("Managed database creation is not configured. Ask a platform administrator to add a database server.")
					}
					if e != nil {
						return e
					}
				}
				resource = newID()
				dbIn := databaseInput{ID: resource, Mode: "managed", Layout: layout, Target: in.Database.Target}
				if e = s.allocateDatabase(r.Context(), tx, p, tenant, dbIn, policy, r.Header.Get("X-Request-ID")); e != nil {
					return e
				}
				if layout == "shared" {
					if _, e = tx.ExecContext(r.Context(), "UPDATE tenants SET default_shared_database_id=?,revision=revision+1 WHERE id=?", resource, tenant); e != nil {
						return e
					}
				}
			}
		}
		_, e = tx.ExecContext(r.Context(), "INSERT INTO spaces (id,tenant_id,slug,display_name,database_resource_id,database_layout) VALUES (?,?,?,?,?,?)", in.ID, tenant, in.Slug, in.Name, resource, layout)
		if e != nil {
			return e
		}
		if e := assignInitialManager(r.Context(), tx, p, tenant, in.ID, "", in.Manager, r.Header.Get("X-Request-ID")); e != nil {
			return e
		}
		return audit(r.Context(), tx, tenant, p.ID, "space.created", "space", in.ID, r.Header.Get("X-Request-ID"), Row{"database_resource_id": resource})
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "spaces", tenant, in.ID, 201)
}

func (s *Server) apps(w http.ResponseWriter, r *http.Request, p Principal) error {
	tenant := r.PathValue("tenant")
	sc, e := s.scopes(r.Context(), s.DB, p, tenant, "app.read")
	if e != nil {
		return e
	}
	filter, args := scopeSQL(sc, "a.space_id", "a.id")
	args = append([]any{tenant}, args...)
	query := "SELECT a.id,a.tenant_id,a.space_id,sp.workspace_id,a.slug,a.display_name,a.state,a.compute_mode,a.revision,a.created_at,sp.display_name AS space_name,(SELECT COUNT(*) FROM microapp_installations m WHERE m.app_id=a.id AND m.state<>'archived') AS microapp_count FROM apps a JOIN spaces sp ON sp.id=a.space_id WHERE a.tenant_id=? AND " + filter
	if space := r.URL.Query().Get("space_id"); space != "" {
		if e = s.authorize(r.Context(), s.DB, p, "app.read", Resource{Tenant: tenant, Space: space}, false); e != nil {
			return e
		}
		query += " AND a.space_id=?"
		args = append(args, space)
	} else {
		query += " AND sp.workspace_id IS NULL"
	}
	v, e := pageRows(r.Context(), s.DB, r, query+" ORDER BY a.created_at DESC", args...)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}

func (s *Server) createApp(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in appCreateInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if e := id(in.ID); e != nil {
		return e
	}
	if e := named(in.Slug, in.Name); e != nil {
		return e
	}
	if in.Compute == "" {
		in.Compute = "shared"
	}
	if in.Compute != "shared" && in.Compute != "dedicated" {
		return bad("Invalid compute mode.")
	}
	tenant, space := r.PathValue("tenant"), r.PathValue("space")
	e := s.mutate(r.Context(), p, "app.manage", Resource{Tenant: tenant, Space: space}, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		parent, e := row(r.Context(), tx, "SELECT state FROM spaces WHERE id=? AND tenant_id=?", space, tenant)
		if e != nil {
			return e
		}
		if str(parent["state"]) != stateActive {
			return conflict("The parent Space must be active.")
		}
		policy, e := policyFor(r.Context(), tx, tenant)
		if e != nil {
			return e
		}
		if e = appQuota(r.Context(), tx, tenant, space, policy, true); e != nil {
			return e
		}
		if in.Compute == "dedicated" && integer(policy["allow_dedicated_compute"]) != 1 {
			return problem(403, "plan_restriction", "Dedicated compute is not permitted by this plan.")
		}
		_, e = tx.ExecContext(r.Context(), "INSERT INTO apps (id,tenant_id,space_id,slug,display_name,compute_mode) VALUES (?,?,?,?,?,?)", in.ID, tenant, space, in.Slug, in.Name, in.Compute)
		if e != nil {
			return e
		}
		if e := assignInitialManager(r.Context(), tx, p, tenant, space, in.ID, in.Manager, r.Header.Get("X-Request-ID")); e != nil {
			return e
		}
		return audit(r.Context(), tx, tenant, p.ID, "app.created", "app", in.ID, r.Header.Get("X-Request-ID"), Row{"space_id": space})
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "apps", tenant, in.ID, 201)
}

// Microapps pages the micro-apps a may read in tenant, optionally of one App.
// Without an App filter, development workspace Apps are excluded.
func (v *inventory) Microapps(ctx context.Context, a Actor, tenant, app string, page PageRequest) (Page[Row], error) {
	sc, e := v.run.scopes(ctx, a, tenant, "microapp.read")
	if e != nil {
		return Page[Row]{}, e
	}
	filter, args := scopeSQL(sc, "a.space_id", "m.app_id")
	args = append([]any{tenant}, args...)
	query := "SELECT m.id,m.tenant_id,m.app_id,m.slug,m.display_name,m.artifact_ref,m.navigation_order,m.state,m.revision,m.created_at,a.display_name AS app_name,a.space_id FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE m.tenant_id=? AND " + filter
	if app != "" {
		if e = v.run.authorize(ctx, a, "microapp.read", Resource{Tenant: tenant, App: app}); e != nil {
			return Page[Row]{}, e
		}
		query += " AND m.app_id=?"
		args = append(args, app)
	} else {
		query += " AND NOT EXISTS(SELECT 1 FROM spaces dsp WHERE dsp.id=a.space_id AND dsp.workspace_id IS NOT NULL)"
	}
	return pageQuery(ctx, v.db, page, collectionFor("microapps"), query+" ORDER BY m.navigation_order,m.created_at DESC", args...)
}

func (v *inventory) Microapp(ctx context.Context, a Actor, tenant, record string) (Row, error) {
	m, e := row(ctx, v.db, "SELECT id,tenant_id,app_id,slug,display_name,artifact_ref,navigation_order,state,revision FROM microapp_installations WHERE tenant_id=? AND id=?", tenant, record)
	if e != nil {
		return nil, e
	}
	if e = v.run.authorize(ctx, a, "microapp.read", Resource{Tenant: tenant, App: str(m["app_id"])}); e != nil {
		return nil, e
	}
	return m, nil
}

func (v *inventory) CreateMicroapp(ctx context.Context, a Actor, tenant, app string, in microappCreateInput) (Ref, error) {
	if e := id(in.ID); e != nil {
		return Ref{}, e
	}
	if e := named(in.Slug, in.Name); e != nil {
		return Ref{}, e
	}
	if len(in.Artifact) > 1024 {
		return Ref{}, bad("Artifact reference is too long.")
	}
	parent, e := row(ctx, v.db, "SELECT space_id FROM apps WHERE id=? AND tenant_id=?", app, tenant)
	if e != nil {
		return Ref{}, e
	}
	res := Resource{Tenant: tenant, Space: str(parent["space_id"]), App: app}
	e = v.run.mutate(ctx, a, "microapp.manage", res, func(tx *sql.Tx) error {
		if _, e := policyFor(ctx, tx, tenant); e != nil {
			return e
		}
		if e := metadataQuota(ctx, tx, tenant, "microapp_installations"); e != nil {
			return e
		}
		parent, e := row(ctx, tx, "SELECT a.state,sp.state AS space_state FROM apps a JOIN spaces sp ON sp.id=a.space_id WHERE a.id=? AND a.tenant_id=?", app, tenant)
		if e != nil {
			return e
		}
		if str(parent["state"]) != stateActive || str(parent["space_state"]) != stateActive {
			return conflict("The parent App and Space must be active.")
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO microapp_installations (id,tenant_id,app_id,slug,display_name,artifact_ref,navigation_order) VALUES (?,?,?,?,?,?,?)", in.ID, tenant, app, in.Slug, in.Name, nullable(in.Artifact), in.Order)
		if e != nil {
			return e
		}
		return auditAs(ctx, tx, a, tenant, "microapp.created", "microapp", in.ID, Row{"app_id": app})
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, "microapp_installations", tenant, in.ID)
}

func (v *inventory) UpdateSpace(ctx context.Context, a Actor, tenant, record string, in metadataPatch, revision int64) (Ref, error) {
	return v.patch(ctx, a, "spaces", "space.manage", Resource{Tenant: tenant, Space: record}, record, in, revision)
}

func (v *inventory) UpdateApp(ctx context.Context, a Actor, tenant, record string, in metadataPatch, revision int64) (Ref, error) {
	parent, e := row(ctx, v.db, "SELECT space_id FROM apps WHERE id=? AND tenant_id=?", record, tenant)
	if e != nil {
		return Ref{}, e
	}
	return v.patch(ctx, a, "apps", "app.manage", Resource{Tenant: tenant, Space: str(parent["space_id"]), App: record}, record, in, revision)
}

func (v *inventory) UpdateMicroapp(ctx context.Context, a Actor, tenant, record string, in metadataPatch, revision int64) (Ref, error) {
	parent, e := row(ctx, v.db, "SELECT a.space_id,m.app_id FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE m.id=? AND m.tenant_id=?", record, tenant)
	if e != nil {
		return Ref{}, e
	}
	return v.patch(ctx, a, "microapp_installations", "microapp.manage", Resource{Tenant: tenant, Space: str(parent["space_id"]), App: str(parent["app_id"])}, record, in, revision)
}

// patch changes Space, App or micro-app metadata and lifecycle state under the
// record's revision. Restoring an archived Space or App rechecks plan quotas.
func (v *inventory) patch(ctx context.Context, a Actor, table, action string, res Resource, record string, in metadataPatch, revision int64) (Ref, error) {
	if revision < 1 {
		return Ref{}, revisionRequired()
	}
	e := v.run.mutate(ctx, a, action, res, func(tx *sql.Tx) error {
		old, e := lockRecord(ctx, tx, table, res.Tenant, record)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(old["revision"]), revision); e != nil {
			return e
		}
		if table != "spaces" {
			parent, e := row(ctx, tx, "SELECT state FROM spaces WHERE id=? AND tenant_id=?", res.Space, res.Tenant)
			if e != nil {
				return e
			}
			if str(parent["state"]) != stateActive {
				return conflict("The parent Space must be active.")
			}
		}
		if table == "microapp_installations" {
			app, e := row(ctx, tx, "SELECT state FROM apps WHERE id=? AND tenant_id=?", res.App, res.Tenant)
			if e != nil {
				return e
			}
			if str(app["state"]) != stateActive {
				return conflict("The parent App must be active.")
			}
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
			if !validState(*in.State) {
				return bad("Invalid state.")
			}
			if str(old["state"]) == stateArchived && *in.State != stateArchived && (table == "apps" || table == "spaces") {
				policy, e := policyFor(ctx, tx, res.Tenant)
				if e != nil {
					return e
				}
				if table == "apps" {
					e = appQuota(ctx, tx, res.Tenant, res.Space, policy, false)
				} else {
					e = quota(ctx, tx, res.Tenant, table, "max_"+table, policy)
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
				return bad("Compute mode applies only to Apps.")
			}
			if *in.Compute != "shared" && *in.Compute != "dedicated" {
				return bad("Invalid compute mode.")
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
				if integer(policy["allow_dedicated_compute"]) != 1 {
					return problem(403, "plan_restriction", "Dedicated compute is not permitted.")
				}
			}
			sets = append(sets, "compute_mode=?")
			args = append(args, *in.Compute)
		}
		if in.Artifact != nil || in.Order != nil {
			if table != "microapp_installations" {
				return bad("Artifact/navigation fields apply only to micro-apps.")
			}
			if in.Artifact != nil {
				if len(*in.Artifact) > 1024 {
					return bad("Artifact reference is too long.")
				}
				sets = append(sets, "artifact_ref=?")
				args = append(args, nullable(*in.Artifact))
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
		return auditAs(ctx, tx, a, res.Tenant, table+".updated", table, record, Row{"state": in.State})
	})
	if e != nil {
		return Ref{}, e
	}
	return v.run.ref(ctx, table, res.Tenant, record)
}

func (s *Server) microapps(w http.ResponseWriter, r *http.Request, p Principal) error {
	v, e := s.inv.Microapps(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.URL.Query().Get("app_id"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createMicroapp(w http.ResponseWriter, r *http.Request, p Principal) error {
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

func (s *Server) updateSpace(w http.ResponseWriter, r *http.Request, p Principal) error {
	return updateInventory(w, r, p, s.inv.UpdateSpace)
}

func (s *Server) updateApp(w http.ResponseWriter, r *http.Request, p Principal) error {
	return updateInventory(w, r, p, s.inv.UpdateApp)
}

func (s *Server) updateMicroapp(w http.ResponseWriter, r *http.Request, p Principal) error {
	return updateInventory(w, r, p, s.inv.UpdateMicroapp)
}

// updateInventory is the console adapter of the Space, App and micro-app
// metadata and lifecycle operations.
func updateInventory(w http.ResponseWriter, r *http.Request, p Principal, update func(context.Context, Actor, string, string, metadataPatch, int64) (Ref, error)) error {
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
