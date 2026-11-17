package worker

import (
	"context"
	"database/sql"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/registry"
)

// Cleanup deletes expired console sessions, agent token sessions and OAuth
// requests, build upload tickets and stale OAuth clients, at most once a
// minute and first when a term starts.
type Cleanup struct{ at time.Time }

func (*Cleanup) Name() string { return "cleanup" }

func (c *Cleanup) Start(context.Context, *sql.Conn) error {
	c.at = time.Time{}
	return nil
}

func (c *Cleanup) Run(ctx context.Context, db *sql.DB) (bool, error) {
	if time.Since(c.at) <= time.Minute {
		return false, nil
	}
	if _, e := CleanupSessions(ctx, db); e != nil {
		return false, e
	}
	if e := access.CleanupAgentSessions(ctx, db); e != nil {
		return false, e
	}
	if _, e := db.ExecContext(ctx, "DELETE FROM artifact_uploads WHERE expires_at<=UTC_TIMESTAMP(6) LIMIT 1000"); e != nil {
		return false, e
	}
	if e := access.CleanupClients(ctx, db); e != nil {
		return false, e
	}
	c.at = time.Now()
	return false, nil
}

// CleanupSessions deletes up to 1000 expired console sessions.
func CleanupSessions(ctx context.Context, q registry.Queryer) (sql.Result, error) {
	return q.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at<=UTC_TIMESTAMP(6) LIMIT 1000")
}

// TenantJob advances the head job of the first Tenant after `after` (Tenant
// ID order) that has one. It returns that Tenant ("" when no later Tenant has
// a job) and whether the job progressed (false: it is waiting). On an error it
// still returns the Tenant it tried, if any.
type TenantJob func(ctx context.Context, q registry.Queryer, after string) (tenant string, progressed bool, err error)

// Fair serves a TenantJob queue (the delivery deployment and workspace jobs)
// round-robin over Tenants. One run visits each Tenant with work at most once,
// starting after the Tenant served last, and stops at the first job that
// progresses. A Tenant whose head job is waiting is skipped, so it never
// blocks another Tenant; within one Tenant the job order is the TenantJob's.
type Fair struct {
	Label string
	Job   TenantJob
	last  string // Tenant visited last in this leadership term
}

func (f *Fair) Name() string { return f.Label }

func (f *Fair) Start(context.Context, *sql.Conn) error {
	f.last = ""
	return nil
}

func (f *Fair) Run(ctx context.Context, db *sql.DB) (bool, error) {
	start, after, wrapped := f.last, f.last, false
	for ctx.Err() == nil {
		if wrapped && after != "" && after >= start {
			return false, nil // Every Tenant with work was visited once.
		}
		tenant, progressed, e := f.Job(ctx, db, after)
		if tenant != "" {
			// A failing Tenant is passed over next time, so it cannot starve others.
			f.last = tenant
		}
		switch {
		case e != nil:
			return false, e
		case progressed:
			return true, nil
		case tenant == "" && (wrapped || start == ""):
			return false, nil
		case tenant == "":
			after, wrapped = "", true
			continue
		case wrapped && tenant > start:
			return false, nil // The served Tenant has no job left; the pass is over.
		}
		after = tenant
	}
	return false, ctx.Err()
}

// Publication runs entity publication jobs, round-robin over Tenants within a
// term. It reports busy when it ran a job, so queued publications run back to
// back without the idle pause.
type Publication struct {
	Job  func(ctx context.Context, q registry.Queryer, cursor ...*string) (bool, error)
	last string
}

func (*Publication) Name() string { return "publication" }

func (p *Publication) Start(context.Context, *sql.Conn) error {
	p.last = ""
	return nil
}

func (p *Publication) Run(ctx context.Context, db *sql.DB) (bool, error) {
	return p.Job(ctx, db, &p.last)
}
