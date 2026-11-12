package control

import (
	"database/sql"
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

func (s *Server) spaces(w http.ResponseWriter, r *http.Request, p Principal) error {
	tenant := r.PathValue("tenant")
	sc, e := s.scopes(r.Context(), s.DB, p, tenant, "space.read")
	if e != nil {
		return e
	}
	filter, args := scopeSQL(sc, "sp.id", "")
	args = append([]any{tenant}, args...)
	v, e := pageRows(r.Context(), s.DB, r, "SELECT sp.id,sp.tenant_id,sp.slug,sp.display_name,sp.database_resource_id,sp.database_layout,sp.state,sp.revision,sp.created_at,d.database_name,d.state AS database_state,(SELECT COUNT(*) FROM apps a WHERE a.space_id=sp.id AND a.state<>'archived') AS app_count FROM spaces sp JOIN database_resources d ON d.id=sp.database_resource_id WHERE sp.tenant_id=? AND "+filter+" ORDER BY sp.created_at DESC", args...)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}
func (s *Server) createSpace(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		namedInput
		Manager  string `json:"initial_manager_user_id"`
		Database struct {
			Resource string `json:"resource_id"`
			Mode     string `json:"mode"`
			Layout   string `json:"layout"`
			Target   string `json:"target_id"`
		} `json:"database"`
	}
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
			db, e := row(r.Context(), tx, "SELECT id,layout,state FROM database_resources WHERE tenant_id=? AND id=?", tenant, resource)
			if e != nil {
				return e
			}
			if str(db["state"]) == "retired" {
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
				if str(db["state"]) == "retired" {
					return conflict("The default database is retired.")
				}
				if in.Database.Target != "" && in.Database.Target != str(db["target_id"]) {
					return conflict("The selected target differs from the existing default shared database.")
				}
			} else {
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
	query := "SELECT a.id,a.tenant_id,a.space_id,a.slug,a.display_name,a.state,a.compute_mode,a.revision,a.created_at,sp.display_name AS space_name,(SELECT COUNT(*) FROM microapp_installations m WHERE m.app_id=a.id AND m.state<>'archived') AS microapp_count FROM apps a JOIN spaces sp ON sp.id=a.space_id WHERE a.tenant_id=? AND " + filter
	if space := r.URL.Query().Get("space_id"); space != "" {
		query += " AND a.space_id=?"
		args = append(args, space)
	}
	v, e := pageRows(r.Context(), s.DB, r, query+" ORDER BY a.created_at DESC", args...)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}
func (s *Server) createApp(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		namedInput
		Manager string `json:"initial_manager_user_id"`
		Compute string `json:"compute_mode"`
	}
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
		if str(parent["state"]) != "active" {
			return conflict("The parent Space must be active.")
		}
		policy, e := policyFor(r.Context(), tx, tenant)
		if e != nil {
			return e
		}
		if e = quota(r.Context(), tx, tenant, "apps", "max_apps", policy); e != nil {
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
func (s *Server) microapps(w http.ResponseWriter, r *http.Request, p Principal) error {
	tenant := r.PathValue("tenant")
	sc, e := s.scopes(r.Context(), s.DB, p, tenant, "microapp.read")
	if e != nil {
		return e
	}
	filter, args := scopeSQL(sc, "a.space_id", "m.app_id")
	args = append([]any{tenant}, args...)
	query := "SELECT m.id,m.tenant_id,m.app_id,m.slug,m.display_name,m.artifact_ref,m.navigation_order,m.state,m.revision,m.created_at,a.display_name AS app_name,a.space_id FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE m.tenant_id=? AND " + filter
	if app := r.URL.Query().Get("app_id"); app != "" {
		query += " AND m.app_id=?"
		args = append(args, app)
	}
	v, e := pageRows(r.Context(), s.DB, r, query+" ORDER BY m.navigation_order,m.created_at DESC", args...)
	if e == nil {
		sendPage(w, r, v)
	}
	return e
}
func (s *Server) createMicroapp(w http.ResponseWriter, r *http.Request, p Principal) error {
	var in struct {
		namedInput
		Artifact string `json:"artifact_ref"`
		Order    int    `json:"navigation_order"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	if e := id(in.ID); e != nil {
		return e
	}
	if e := named(in.Slug, in.Name); e != nil {
		return e
	}
	if len(in.Artifact) > 1024 {
		return bad("Artifact reference is too long.")
	}
	tenant, app := r.PathValue("tenant"), r.PathValue("app")
	parent, e := row(r.Context(), s.DB, "SELECT space_id FROM apps WHERE id=? AND tenant_id=?", app, tenant)
	if e != nil {
		return e
	}
	res := Resource{Tenant: tenant, Space: str(parent["space_id"]), App: app}
	e = s.mutate(r.Context(), p, "microapp.manage", res, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		if _, e := policyFor(r.Context(), tx, tenant); e != nil {
			return e
		}
		if e := metadataQuota(r.Context(), tx, tenant, "microapp_installations"); e != nil {
			return e
		}
		a, e := row(r.Context(), tx, "SELECT a.state,sp.state AS space_state FROM apps a JOIN spaces sp ON sp.id=a.space_id WHERE a.id=? AND a.tenant_id=?", app, tenant)
		if e != nil {
			return e
		}
		if str(a["state"]) != "active" || str(a["space_state"]) != "active" {
			return conflict("The parent App and Space must be active.")
		}
		_, e = tx.ExecContext(r.Context(), "INSERT INTO microapp_installations (id,tenant_id,app_id,slug,display_name,artifact_ref,navigation_order) VALUES (?,?,?,?,?,?,?)", in.ID, tenant, app, in.Slug, in.Name, nullable(in.Artifact), in.Order)
		if e != nil {
			return e
		}
		return audit(r.Context(), tx, tenant, p.ID, "microapp.created", "microapp", in.ID, r.Header.Get("X-Request-ID"), Row{"app_id": app})
	})
	if e != nil {
		return e
	}
	return result(w, r, s, "microapp_installations", tenant, in.ID, 201)
}
func (s *Server) updateSpace(w http.ResponseWriter, r *http.Request, p Principal) error {
	return s.patchInventory(w, r, p, "spaces", "space.manage", Resource{Tenant: r.PathValue("tenant"), Space: r.PathValue("id")})
}
func (s *Server) updateApp(w http.ResponseWriter, r *http.Request, p Principal) error {
	parent, e := row(r.Context(), s.DB, "SELECT space_id FROM apps WHERE id=? AND tenant_id=?", r.PathValue("id"), r.PathValue("tenant"))
	if e != nil {
		return e
	}
	return s.patchInventory(w, r, p, "apps", "app.manage", Resource{Tenant: r.PathValue("tenant"), Space: str(parent["space_id"]), App: r.PathValue("id")})
}
func (s *Server) updateMicroapp(w http.ResponseWriter, r *http.Request, p Principal) error {
	parent, e := row(r.Context(), s.DB, "SELECT a.space_id,m.app_id FROM microapp_installations m JOIN apps a ON a.id=m.app_id WHERE m.id=? AND m.tenant_id=?", r.PathValue("id"), r.PathValue("tenant"))
	if e != nil {
		return e
	}
	return s.patchInventory(w, r, p, "microapp_installations", "microapp.manage", Resource{Tenant: r.PathValue("tenant"), Space: str(parent["space_id"]), App: str(parent["app_id"])})
}
func (s *Server) patchInventory(w http.ResponseWriter, r *http.Request, p Principal, table, action string, res Resource) error {
	var in metadataPatch
	if e := decode(w, r, &in); e != nil {
		return e
	}
	rev, e := expectedRevision(r)
	if e != nil {
		return e
	}
	record := r.PathValue("id")
	e = s.mutate(r.Context(), p, action, res, r.Header.Get("X-Request-ID"), func(tx *sql.Tx) error {
		old, e := lockRecord(r.Context(), tx, table, res.Tenant, record)
		if e != nil {
			return e
		}
		if e = checkRevision(integer(old["revision"]), rev); e != nil {
			return e
		}
		if table != "spaces" {
			parent, e := row(r.Context(), tx, "SELECT state FROM spaces WHERE id=? AND tenant_id=?", res.Space, res.Tenant)
			if e != nil {
				return e
			}
			if str(parent["state"]) != "active" {
				return conflict("The parent Space must be active.")
			}
		}
		if table == "microapp_installations" {
			a, e := row(r.Context(), tx, "SELECT state FROM apps WHERE id=? AND tenant_id=?", res.App, res.Tenant)
			if e != nil {
				return e
			}
			if str(a["state"]) != "active" {
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
			if str(old["state"]) == "archived" && *in.State != "archived" && (table == "apps" || table == "spaces") {
				policy, e := policyFor(r.Context(), tx, res.Tenant)
				if e != nil {
					return e
				}
				if e = quota(r.Context(), tx, res.Tenant, table, "max_"+table, policy); e != nil {
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
			if e = tx.QueryRowContext(r.Context(), "SELECT compute_mode FROM apps WHERE id=? AND tenant_id=?", record, res.Tenant).Scan(&previous); e != nil {
				return e
			}
			if *in.Compute == "dedicated" && previous != *in.Compute {
				policy, e := policyFor(r.Context(), tx, res.Tenant)
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
		_, e = tx.ExecContext(r.Context(), "UPDATE "+table+" SET "+strings.Join(sets, ",")+" WHERE id=? AND tenant_id=?", args...)
		if e != nil {
			return e
		}
		return audit(r.Context(), tx, res.Tenant, p.ID, table+".updated", table, record, r.Header.Get("X-Request-ID"), Row{"state": in.State})
	})
	if e != nil {
		return e
	}
	return result(w, r, s, table, res.Tenant, record, 200)
}
