// Package worker runs the background queue steps under one in-process leader.
// One registry connection is pinned for a leadership term: it holds the
// GET_LOCK that elects the leader and every step runs its work on it, in the
// fixed order the steps are listed. A step error ends the term (the lock is
// released and the connection discarded or returned) and leadership is retried
// a second later; the steps themselves recheck live authority for each job.
package worker

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/aantich/lastapp/internal/data"
)

// Step is one queue the leader drains. Run handles at most one job on the
// leader connection and reports whether it found work.
type Step interface {
	Name() string
	Run(ctx context.Context, conn *sql.Conn) (busy bool, err error)
}

// Starter is a step with per-term state or recovery. Start runs, in step
// order, each time leadership is acquired and before any step runs.
type Starter interface {
	Start(ctx context.Context, conn *sql.Conn) error
}

// Leader elects one worker per registry with GET_LOCK on a pinned connection
// and runs Steps in order while it holds the lock. When no step reports work
// it pauses for a second before the next round.
type Leader struct {
	DB    *sql.DB
	Lock  string
	Steps []Step
	// Warn logs a failure without its message (see httpapi.Diagnostic).
	Warn func(ctx context.Context, msg string, err error, attrs ...any)
	// term serializes leadership terms in this process, so step state is
	// never shared between two terms.
	term sync.Mutex
}

// Run keeps trying to lead until ctx ends.
func (l *Leader) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if e := l.lead(ctx); e != nil && ctx.Err() == nil {
			l.Warn(ctx, "provisioning leader disconnected", e)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// lead runs one leadership term. It returns nil at once when another session
// holds the lock.
func (l *Leader) lead(ctx context.Context) error {
	conn, e := l.DB.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	var acquired int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", l.Lock).Scan(&acquired); e != nil {
		return e
	}
	if acquired != 1 {
		return nil
	}
	defer data.ReleasePinnedLock(conn, l.Lock)
	l.term.Lock()
	defer l.term.Unlock()
	for _, step := range l.Steps {
		if s, ok := step.(Starter); ok {
			if e = s.Start(ctx, conn); e != nil {
				return e
			}
		}
	}
	for ctx.Err() == nil {
		busy := false
		for _, step := range l.Steps {
			found, e := step.Run(ctx, conn)
			if e != nil {
				return e
			}
			busy = busy || found
		}
		if !busy {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
	}
	return ctx.Err()
}
