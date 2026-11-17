// Package worker runs the background queue steps under one in-process leader.
//
// One registry connection is pinned for a leadership term and holds the
// GET_LOCK that elects the leader. When a term starts, every Starter runs in
// step order on that connection (single-leader recovery, e.g. interrupted
// provisioning back to pending) before any step runs. Then each step runs in
// its own goroutine loop with its own registry connection per run, so a slow
// or failing step never delays another kind of work. A step error is logged
// with the step name and the step backs off; only losing the lock connection
// (checked every heartbeat) or the end of ctx ends the term. The term waits
// for every step goroutine before it releases the lock, and leadership is
// retried a second later. The steps recheck live authority for each job.
package worker

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/op"
)

// Step is one queue the leader drains. Run handles at most one job (or one
// fairness round) on the given connection and reports whether it did work;
// a step that did no work pauses before its next run.
type Step interface {
	Name() string
	Run(ctx context.Context, conn *sql.Conn) (busy bool, err error)
}

// Starter is a step with per-term state or recovery. Start runs, in step
// order on the lock connection, each time leadership is acquired and before
// any step runs.
type Starter interface {
	Start(ctx context.Context, conn *sql.Conn) error
}

// Pacing of the leader and its step loops. Variables only so tests can shorten
// them.
var (
	// retryLeader is the pause before trying to lead again.
	retryLeader = time.Second
	// heartbeat is how often the leader confirms it still holds the lock.
	heartbeat = time.Second
	// idlePause is the pause of a step that found no work.
	idlePause = time.Second
	// retryPause is the pause before retrying after a deadlock or lock wait
	// timeout; it is not a step failure.
	retryPause = 100 * time.Millisecond
	// minBackoff and maxBackoff bound the doubling pause after consecutive
	// failures of one step.
	minBackoff = time.Second
	maxBackoff = time.Minute
)

// errLockLost ends a term whose connection no longer holds the leader lock.
var errLockLost = errors.New("worker leader lock lost")

// Leader elects one worker per registry with GET_LOCK on a pinned connection
// and runs every Step concurrently while it holds the lock.
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
			l.Warn(ctx, "worker leader disconnected", e)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryLeader):
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
	term, end := context.WithCancel(ctx)
	var steps sync.WaitGroup
	for _, step := range l.Steps {
		steps.Add(1)
		go func() {
			defer steps.Done()
			l.loop(term, step)
		}()
	}
	e = l.hold(term, conn)
	// Every step stops before the lock is released (deferred above).
	end()
	steps.Wait()
	return e
}

// hold returns when ctx ends or when conn no longer holds the lock (including
// a broken connection).
func (l *Leader) hold(ctx context.Context, conn *sql.Conn) error {
	tick := time.NewTicker(heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		var mine sql.NullInt64
		if e := conn.QueryRowContext(ctx, "SELECT IS_USED_LOCK(?)=CONNECTION_ID()", l.Lock).Scan(&mine); e != nil {
			return e
		}
		if !mine.Valid || mine.Int64 != 1 {
			return errLockLost
		}
	}
}

// loop runs one step until the term ends. Each run borrows its own registry
// connection, so steps never share a session. A deadlock or lock wait timeout
// is retried shortly; any other error is logged with the step name and backs
// the step off, doubling up to maxBackoff, without affecting other steps.
func (l *Leader) loop(ctx context.Context, step Step) {
	failures := 0
	for ctx.Err() == nil {
		busy, e := l.runOnce(ctx, step)
		pause := time.Duration(0)
		switch {
		case ctx.Err() != nil:
			return
		case e == nil:
			failures = 0
			if !busy {
				pause = idlePause
			}
		case op.Transient(e):
			pause = retryPause
		default:
			failures++
			l.Warn(ctx, "worker step failed", e, "step", step.Name(), "consecutive_failures", failures)
			pause = backoff(failures)
		}
		if pause > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(pause):
			}
		}
	}
}

func (l *Leader) runOnce(ctx context.Context, step Step) (bool, error) {
	conn, e := l.DB.Conn(ctx)
	if e != nil {
		return false, e
	}
	defer conn.Close()
	return step.Run(ctx, conn)
}

// backoff is the pause after the given number of consecutive failures.
func backoff(failures int) time.Duration {
	d := minBackoff
	for i := 1; i < failures && d < maxBackoff; i++ {
		d *= 2
	}
	return min(d, maxBackoff)
}
