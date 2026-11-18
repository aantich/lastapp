package control

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/artifact"
	"github.com/aantich/lastapp/internal/delivery"
	"github.com/aantich/lastapp/internal/entities"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

// Stable identities of the Northstar demo content. The Tenant, Space, Apps and
// role are created by seedFoundationDemo.
const (
	demoTenant      = "30000000-0000-4000-8000-000000000001"
	demoSpace       = "40000000-0000-4000-8000-000000000001"
	demoDealReview  = "50000000-0000-4000-8000-000000000001"
	demoForecasting = "50000000-0000-4000-8000-000000000002"
	demoSalesRole   = "70000000-0000-4000-8000-000000000001"
	demoAccount     = "80000000-0000-4000-8000-000000000001"
	demoContact     = "80000000-0000-4000-8000-000000000002"
	demoDeal        = "80000000-0000-4000-8000-000000000003"
	demoWorkspace   = "90000000-0000-4000-8000-000000000001"
	// demoWorkspaceName names the workspace that only captures the releases; it
	// is archived once both Apps are deployed.
	demoWorkspaceName = "demo-release"
)

// errDemoSkipped stops the demo content without failing the seed.
var errDemoSkipped = errors.New("demo content skipped")

// seedNorthstarContent fills the Northstar demo the way an author would: it
// publishes CRM entities and ships each App through a static release and a
// production deployment. Releases are captured in a development workspace,
// which is archived once both Apps are deployed. Every change is a
// validated operation performed as the Tenant administrator, and the queued
// background work runs through the worker's own steps (worker.Leader.Settle).
// Each piece is created only while it is missing: a second run changes nothing,
// and nothing a reviewer changed or removed afterwards is restored.
func (s *Server) seedNorthstarContent(ctx context.Context) error {
	var owner string
	if e := s.DB.QueryRowContext(ctx, "SELECT id FROM users WHERE oidc_issuer=? AND oidc_subject='tenant-admin'", s.Config.OIDCIssuer).Scan(&owner); e != nil {
		return e
	}
	var spaces int
	if e := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM spaces sp JOIN tenants t ON t.id=sp.tenant_id WHERE sp.id=? AND sp.state='active' AND t.state='active'", demoSpace).Scan(&spaces); e != nil || spaces == 0 {
		return e
	}
	as := func() op.Actor { return op.ActorFrom(ctx, access.Principal{ID: owner}, registry.NewID()) }
	led, e := s.worker.Settle(ctx, func(drain func() error) error {
		if e := s.seedDemoEntities(ctx, as, drain); e != nil {
			return e
		}
		shipped := true
		for _, app := range []string{demoDealReview, demoForecasting} {
			done, e := s.demoDeployed(ctx, app)
			if e != nil {
				return e
			}
			shipped = shipped && done
		}
		if !shipped {
			devApps, e := s.seedDemoWorkspace(ctx, as, drain)
			if e != nil {
				return e
			}
			if e = s.seedDemoRelease(ctx, as, demoDealReview, devApps[demoDealReview], "Deal review", drain); e != nil {
				return e
			}
			if e = s.seedDemoRelease(ctx, as, demoForecasting, devApps[demoForecasting], "Forecasting", drain); e != nil {
				return e
			}
		}
		return s.archiveDemoWorkspace(ctx, as)
	})
	var ae *registry.APIError
	switch {
	case e == nil && !led:
		s.Log.Warn("Northstar demo content skipped: a running Lastapp worker owns the queues; stop it and seed again")
	case e == nil:
	case errors.Is(e, errDemoSkipped):
		s.Log.Warn("Northstar demo content left incomplete", "reason", e.Error())
	case errors.As(e, &ae) && ae.Status == 409:
		// A reviewer's own change (an unpublished draft, a pending publication)
		// legitimately blocks the demo; it is never overridden.
		s.Log.Warn("Northstar demo content left incomplete", "reason", ae.Message)
	default:
		return e
	}
	return nil
}

// demoEntities are the CRM entities, in reference order. Sales reviewers read
// everything; members create records they then own.
func demoEntities() []entity.Definition {
	name := entity.Field{Key: "name", Label: "Name", Type: "text", MaxLength: 200, Required: true, Filterable: true, Sortable: true}
	owner := entity.Field{Key: "owner", Label: "Owner", Type: "user", Filterable: true, Default: &entity.Expr{Context: "actor.id"}}
	define := func(key, label, description string, fields []entity.Field, projection []string, editable []string) entity.Definition {
		var keys []string
		for _, f := range fields {
			keys = append(keys, f.Key)
		}
		return entity.Definition{Format: entity.FormatVersion, Key: key, Label: label, Description: description, Fields: fields, Owner: "owner", Projection: projection, Rules: []entity.Rule{}, Permissions: []entity.Permission{
			{Key: "sales_read", Label: "Sales reviewers read", Operation: "read", Roles: []string{demoSalesRole}, Fields: keys},
			{Key: "owner_read", Label: "Owners read", Operation: "read", Relationship: "owner", Fields: keys},
			{Key: "member_create", Label: "Members create", Operation: "create", Relationship: "member", Fields: editable},
			{Key: "owner_update", Label: "Owners update", Operation: "update", Relationship: "owner", Fields: editable},
		}}
	}
	account := define("account", "Account", "A customer organization.", []entity.Field{
		name,
		{Key: "industry", Label: "Industry", Type: "choice", Values: []string{"software", "manufacturing", "retail", "healthcare", "finance"}, ValueLabels: map[string]string{"software": "Software", "manufacturing": "Manufacturing", "retail": "Retail", "healthcare": "Healthcare", "finance": "Financial services"}, Filterable: true},
		{Key: "website", Label: "Website", Type: "text", MaxLength: 500},
		{Key: "annual_revenue", Label: "Annual revenue", Type: "decimal", Precision: 18, Scale: 2, Minimum: "0", Sortable: true},
		owner,
	}, []string{"name", "industry"}, []string{"name", "industry", "website", "annual_revenue"})
	contact := define("contact", "Contact", "A person at a customer account.", []entity.Field{
		name,
		{Key: "email", Label: "Email", Type: "text", MaxLength: 320, Filterable: true},
		{Key: "title", Label: "Job title", Type: "text", MaxLength: 160},
		{Key: "phone", Label: "Phone", Type: "text", MaxLength: 40},
		{Key: "account", Label: "Account", Type: "reference", Target: demoAccount, Filterable: true},
		owner,
	}, []string{"name", "email", "account"}, []string{"name", "email", "title", "phone", "account"})
	deal := define("deal", "Deal", "A sales opportunity with an account.", []entity.Field{
		name,
		{Key: "account", Label: "Account", Type: "reference", Target: demoAccount, Required: true, Filterable: true},
		{Key: "primary_contact", Label: "Primary contact", Type: "reference", Target: demoContact},
		{Key: "amount", Label: "Amount", Type: "decimal", Precision: 18, Scale: 2, Required: true, Minimum: "0", Filterable: true, Sortable: true},
		{Key: "stage", Label: "Stage", Type: "choice", Required: true, Values: []string{"prospecting", "qualification", "proposal", "negotiation", "closed_won", "closed_lost"}, ValueLabels: map[string]string{"prospecting": "Prospecting", "qualification": "Qualification", "proposal": "Proposal", "negotiation": "Negotiation", "closed_won": "Closed won", "closed_lost": "Closed lost"}, Default: &entity.Expr{Literal: "prospecting"}, Filterable: true},
		{Key: "probability", Label: "Probability (%)", Type: "integer", Minimum: "0", Maximum: "100", Default: &entity.Expr{Literal: 10}, Sortable: true},
		{Key: "close_date", Label: "Expected close", Type: "date", Filterable: true, Sortable: true},
		owner,
	}, []string{"name", "account", "amount", "stage"}, []string{"name", "account", "primary_contact", "amount", "stage", "probability", "close_date"})
	return []entity.Definition{account, contact, deal}
}

var demoEntityIDs = []string{demoAccount, demoContact, demoDeal}

// seedDemoEntities creates and publishes each CRM entity that does not exist
// yet. An existing entity is left as the reviewer keeps it.
func (s *Server) seedDemoEntities(ctx context.Context, as func() op.Actor, drain func() error) error {
	for i, d := range demoEntities() {
		id := demoEntityIDs[i]
		var revision int64
		var active, pending *string
		e := s.DB.QueryRowContext(ctx, "SELECT revision,active_version_id,pending_version_id FROM entity_definitions WHERE id=?", id).Scan(&revision, &active, &pending)
		switch {
		case e == nil && active != nil:
			continue
		case e == nil && pending != nil:
			// An interrupted earlier run queued the first publication.
			if e = drain(); e != nil {
				return e
			}
			continue
		case e == nil:
			// An interrupted earlier run created the entity but did not publish it.
			if e = s.demoPublish(ctx, as, id, revision, drain); e != nil {
				return e
			}
			continue
		case !errors.Is(e, sql.ErrNoRows):
			return e
		}
		var taken int
		if e := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM entity_definitions WHERE space_id=? AND entity_key=?", demoSpace, d.Key).Scan(&taken); e != nil {
			return e
		}
		if taken > 0 {
			return fmt.Errorf("%w: CRM already has its own %q entity", errDemoSkipped, d.Key)
		}
		created, e := s.authoring.CreateEntity(ctx, as(), demoSpace, entities.EntityCreateInput{ID: id, Definition: d})
		if e != nil {
			return e
		}
		if e = s.demoPublish(ctx, as, id, created.Revision, drain); e != nil {
			return e
		}
	}
	return nil
}

// demoPublish publishes an entity's saved draft and waits for the publication.
func (s *Server) demoPublish(ctx context.Context, as func() op.Actor, id string, revision int64, drain func() error) error {
	if _, e := s.authoring.PublishEntity(ctx, as(), id, revision); e != nil {
		return e
	}
	if e := drain(); e != nil {
		return e
	}
	var state, message string
	if e := s.DB.QueryRowContext(ctx, "SELECT state,COALESCE(error_message,'') FROM entity_versions WHERE entity_id=? ORDER BY created_at DESC,id DESC LIMIT 1", id).Scan(&state, &message); e != nil {
		return e
	}
	if state != "published" {
		return fmt.Errorf("demo entity %s publication %s: %s", id, state, message)
	}
	return nil
}

// seedDemoWorkspace creates the Tenant administrator's release workspace once
// and returns the development App of each production App.
func (s *Server) seedDemoWorkspace(ctx context.Context, as func() op.Actor, drain func() error) (map[string]string, error) {
	var exists int
	if e := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM development_workspaces WHERE id=?", demoWorkspace).Scan(&exists); e != nil {
		return nil, e
	}
	if exists == 0 {
		if _, e := s.delivery.CreateWorkspace(ctx, as(), demoSpace, delivery.WorkspaceCreateInput{ID: demoWorkspace, Name: demoWorkspaceName}); e != nil {
			return nil, e
		}
	}
	if e := drain(); e != nil {
		return nil, e
	}
	var state string
	if e := s.DB.QueryRowContext(ctx, "SELECT state FROM development_workspaces WHERE id=?", demoWorkspace).Scan(&state); e != nil {
		return nil, e
	}
	if state != "ready" {
		return nil, fmt.Errorf("%w: workspace %s is %s", errDemoSkipped, demoWorkspaceName, state)
	}
	apps := map[string]string{}
	rows, e := registry.QueryRows(ctx, s.DB, "SELECT production_id,development_id FROM workspace_items WHERE workspace_id=? AND kind='app'", demoWorkspace)
	if e != nil {
		return nil, e
	}
	for _, r := range rows {
		apps[registry.Str(r["production_id"])] = registry.Str(r["development_id"])
	}
	return apps, nil
}

// demoDeployed reports whether a production App already has a deployment
// (whatever its outcome): the demo then never ships that App again.
func (s *Server) demoDeployed(ctx context.Context, app string) (bool, error) {
	var n int
	e := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM release_deployments WHERE target_app_id=?", app).Scan(&n)
	return n > 0, e
}

// demoRevision is an App's current revision, the expected revision of the
// next change to it.
func (s *Server) demoRevision(ctx context.Context, app string) (int64, error) {
	var revision int64
	return revision, s.DB.QueryRowContext(ctx, "SELECT revision FROM apps WHERE id=?", app).Scan(&revision)
}

// archiveDemoWorkspace archives the release workspace through the ordinary
// workspace operation once both Apps shipped, so no development workspace
// remains active. A missing or already archived workspace is left alone.
func (s *Server) archiveDemoWorkspace(ctx context.Context, as func() op.Actor) error {
	var state string
	var revision int64
	e := s.DB.QueryRowContext(ctx, "SELECT state,revision FROM development_workspaces WHERE id=?", demoWorkspace).Scan(&state, &revision)
	if errors.Is(e, sql.ErrNoRows) || (e == nil && state == registry.StateArchived) {
		return nil
	}
	if e != nil {
		return e
	}
	_, e = s.delivery.ChangeWorkspace(ctx, as(), demoWorkspace, false, revision)
	return e
}

// seedDemoRelease ships one App from the workspace: it uploads a small static
// build through an upload ticket, captures release 1.0.0 and deploys it to
// production, unless that App was deployed before.
func (s *Server) seedDemoRelease(ctx context.Context, as func() op.Actor, app, devApp, title string, drain func() error) error {
	if devApp == "" {
		return nil
	}
	if done, e := s.demoDeployed(ctx, app); e != nil || done {
		return e
	}
	var release string
	if e := s.DB.QueryRowContext(ctx, "SELECT id FROM app_releases WHERE app_id=? AND display_name='1.0.0'", devApp).Scan(&release); e != nil && !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	if release == "" {
		revision, e := s.demoRevision(ctx, devApp)
		if e != nil {
			return e
		}
		// Deployed Apps inherit the Space's permission paths for their data.
		if _, e = s.authoring.SaveAppAccess(ctx, as(), devApp, entity.AppAccess{Inherit: true}, revision); e != nil {
			return e
		}
		build, e := s.demoBuild(ctx, as, devApp, title)
		if e != nil {
			return e
		}
		if revision, e = s.demoRevision(ctx, devApp); e != nil {
			return e
		}
		created, e := s.delivery.CreateRelease(ctx, as(), devApp, delivery.ReleaseCreateInput{Name: "1.0.0", Build: build, Manifest: delivery.ReleaseManifest{Microapps: map[string]string{}, Dependencies: map[string]string{"react": "19.2.0", "react-dom": "19.2.0"}}}, revision)
		if e != nil {
			return e
		}
		release = created.ID
	}
	revision, e := s.demoRevision(ctx, app)
	if e != nil {
		return e
	}
	queued, e := s.delivery.DeployRelease(ctx, as(), release, revision)
	if e != nil {
		return e
	}
	if e = drain(); e != nil {
		return e
	}
	var state, message string
	if e = s.DB.QueryRowContext(ctx, "SELECT state,COALESCE(error_message,'') FROM release_deployments WHERE id=?", queued["id"]).Scan(&state, &message); e != nil {
		return e
	}
	if state != registry.StateActive {
		return fmt.Errorf("demo deployment of %s %s: %s", title, state, message)
	}
	return nil
}

// demoBuild stores the App's static build through the same ticket-checked
// operations as the upload endpoint, reusing a build an earlier run stored.
func (s *Server) demoBuild(ctx context.Context, as func() op.Actor, devApp, title string) (string, error) {
	var build string
	if e := s.DB.QueryRowContext(ctx, "SELECT id FROM app_builds WHERE app_id=? ORDER BY created_at DESC,id DESC LIMIT 1", devApp).Scan(&build); e == nil || !errors.Is(e, sql.ErrNoRows) {
		return build, e
	}
	ticket, e := s.delivery.UploadTicket(ctx, as(), devApp)
	if e != nil {
		return "", e
	}
	upload, e := s.delivery.AuthorizeUpload(ctx, registry.Str(ticket["upload_token"]))
	if e != nil {
		return "", e
	}
	page := "<!doctype html>\n<html lang=\"en\">\n<head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>" + title + "</title><link rel=\"stylesheet\" href=\"assets/app.css\"></head>\n<body><main><h1>" + title + "</h1><p>Northstar demo release 1.0.0.</p></main></body>\n</html>\n"
	css := "body{margin:0;font-family:system-ui,sans-serif;background:#f6f7f9;color:#1d2433}main{max-width:40rem;margin:4rem auto;padding:0 1rem}\n"
	stored, _, e := s.delivery.StoreBuild(ctx, upload, []artifact.File{{Path: "index.html", Content: []byte(page)}, {Path: "assets/app.css", Content: []byte(css)}}, "index.html", registry.NewID())
	if e != nil {
		return "", e
	}
	return registry.Str(stored["id"]), nil
}
