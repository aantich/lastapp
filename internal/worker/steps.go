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

func (c *Cleanup) Run(ctx context.Context, conn *sql.Conn) (bool, error) {
	if time.Since(c.at) <= time.Minute {
		return false, nil
	}
	if _, e := CleanupSessions(ctx, conn); e != nil {
		return false, e
	}
	if e := access.CleanupAgentSessions(ctx, conn); e != nil {
		return false, e
	}
	if _, e := conn.ExecContext(ctx, "DELETE FROM artifact_uploads WHERE expires_at<=UTC_TIMESTAMP(6) LIMIT 1000"); e != nil {
		return false, e
	}
	if e := access.CleanupClients(ctx, conn); e != nil {
		return false, e
	}
	c.at = time.Now()
	return false, nil
}

// CleanupSessions deletes up to 1000 expired console sessions.
func CleanupSessions(ctx context.Context, q registry.Queryer) (sql.Result, error) {
	return q.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at<=UTC_TIMESTAMP(6) LIMIT 1000")
}

// Job is a queue step that reports only failure (the delivery deployment and
// workspace jobs). It never reports busy, so it does not shorten the pause.
type Job struct {
	Label string
	Do    func(ctx context.Context, q registry.Queryer) error
}

func (j Job) Name() string { return j.Label }

func (j Job) Run(ctx context.Context, conn *sql.Conn) (bool, error) {
	return false, j.Do(ctx, conn)
}

// Publication runs entity publication jobs, round-robin over Tenants within a
// term. Like Job it never reports busy, so it does not shorten the pause.
type Publication struct {
	Job  func(ctx context.Context, q registry.Queryer, cursor ...*string) (bool, error)
	last string
}

func (*Publication) Name() string { return "publication" }

func (p *Publication) Start(context.Context, *sql.Conn) error {
	p.last = ""
	return nil
}

func (p *Publication) Run(ctx context.Context, conn *sql.Conn) (bool, error) {
	_, e := p.Job(ctx, conn, &p.last)
	return false, e
}
