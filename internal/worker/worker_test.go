package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/config"
	"github.com/aantich/lastapp/internal/registry"
)

// lockFixture opens the local MariaDB named by LASTAPP_TEST_CONFIG and returns
// a pool plus a lock name no other test uses. The leader touches no tables, so
// no registry schema is needed.
func lockFixture(t *testing.T) (*sql.DB, string) {
	t.Helper()
	file := os.Getenv("LASTAPP_TEST_CONFIG")
	if file == "" {
		t.Skip("set LASTAPP_TEST_CONFIG for the installed local MariaDB")
	}
	b, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	var c config.Config
	if e = json.Unmarshal(b, &c); e != nil {
		t.Fatal(e)
	}
	cfg, e := mariadb.ParseDSN(c.RegistryDSN)
	if e != nil {
		t.Fatal("invalid registry DSN")
	}
	connector, e := mariadb.NewConnector(cfg)
	if e != nil {
		t.Fatal(e)
	}
	db := sql.OpenDB(connector)
	t.Cleanup(func() { db.Close() })
	return db, "lastapp:worker-test:" + registry.NewID()
}

// fakeStep records its starts and runs. Starts must happen on the connection
// holding the leader lock; runs use the leader's pool, never that connection.
type fakeStep struct {
	name    string
	starter bool
	busy    bool
	fail    func(run int) error
	rec     *recorder
	mu      sync.Mutex
	runs    int
}

type recorder struct {
	mu     sync.Mutex
	events []string
	db     *sql.DB
	lock   string
	t      *testing.T
}

func (r *recorder) add(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

// onLock reports whether the connection conn runs the query on holds the
// leader lock.
func (r *recorder) onLock(ctx context.Context, conn interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (bool, error) {
	var self, owner sql.NullInt64
	if e := conn.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&self); e != nil {
		return false, e
	}
	if e := r.db.QueryRowContext(ctx, "SELECT IS_USED_LOCK(?)", r.lock).Scan(&owner); e != nil {
		return false, e
	}
	return owner.Valid && owner.Int64 == self.Int64, nil
}

func (f *fakeStep) Name() string { return f.name }

func (f *fakeStep) Run(ctx context.Context, db *sql.DB) (bool, error) {
	f.mu.Lock()
	f.runs++
	run := f.runs
	f.mu.Unlock()
	f.rec.add("run:" + f.name)
	held, e := f.rec.onLock(ctx, db)
	if e != nil {
		return false, e
	}
	if held {
		f.rec.t.Errorf("step %s ran on the leader lock connection", f.name)
	}
	if f.fail != nil {
		if e := f.fail(run); e != nil {
			return false, e
		}
	}
	return f.busy, nil
}

func (f *fakeStep) runCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runs
}

// startingStep is a fakeStep that also implements Starter.
type startingStep struct{ *fakeStep }

func (s startingStep) Start(ctx context.Context, conn *sql.Conn) error {
	s.rec.add("start:" + s.name)
	held, e := s.rec.onLock(ctx, conn)
	if e != nil {
		return e
	}
	if !held {
		s.rec.t.Errorf("starter %s did not run on the leader lock connection", s.name)
	}
	return nil
}

func newLeader(t *testing.T, steps ...*fakeStep) (*Leader, *recorder, *warnings) {
	db, lock := lockFixture(t)
	rec := &recorder{db: db, lock: lock, t: t}
	w := &warnings{}
	l := &Leader{DB: db, Lock: lock, Warn: w.add}
	for _, f := range steps {
		f.rec = rec
		if f.starter {
			l.Steps = append(l.Steps, startingStep{f})
		} else {
			l.Steps = append(l.Steps, f)
		}
	}
	return l, rec, w
}

type warnings struct {
	mu   sync.Mutex
	msgs []string
	errs []error
}

func (w *warnings) add(_ context.Context, msg string, err error, _ ...any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.msgs = append(w.msgs, msg)
	w.errs = append(w.errs, err)
}

func (w *warnings) len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.msgs)
}

func run(t *testing.T, l *Leader) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); l.Run(ctx) }()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("leader did not stop")
		}
	}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Starters run once per term in step order on the lock connection before any
// step runs; every step then runs on a connection of its own, and the lock is
// released when the term ends.
func TestLeaderStartsInOrderOnLockConnection(t *testing.T) {
	a := &fakeStep{name: "a", starter: true, busy: true}
	b := &fakeStep{name: "b"}
	c := &fakeStep{name: "c", starter: true}
	l, rec, w := newLeader(t, a, b, c)
	stop := run(t, l)
	waitFor(t, "every step", func() bool { return a.runCount() > 0 && b.runCount() > 0 && c.runCount() > 0 })
	stop()
	events := rec.snapshot()
	if !slices.Equal(events[:2], []string{"start:a", "start:c"}) || slices.Contains(events[2:], "start:a") || slices.Contains(events[2:], "start:c") {
		t.Fatalf("events %v: want both starts first, once", events)
	}
	if w.len() != 0 {
		t.Fatalf("unexpected warnings %v %v", w.msgs, w.errs)
	}
	var free int
	if e := l.DB.QueryRow("SELECT IS_FREE_LOCK(?)", l.Lock).Scan(&free); e != nil || free != 1 {
		t.Fatal("lock not released after the term", e)
	}
}

// While another session holds the lock, no step starts or runs; leadership is
// taken once the lock is free.
func TestLeaderStandsByWhileLockHeld(t *testing.T) {
	a := &fakeStep{name: "a", starter: true, busy: true}
	l, rec, _ := newLeader(t, a)
	held, e := l.DB.Conn(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer held.Close()
	var got int
	if e = held.QueryRowContext(context.Background(), "SELECT GET_LOCK(?,0)", l.Lock).Scan(&got); e != nil || got != 1 {
		t.Fatal("cannot hold lock", e)
	}
	stop := run(t, l)
	defer stop()
	time.Sleep(300 * time.Millisecond)
	if events := rec.snapshot(); len(events) != 0 {
		t.Fatalf("follower ran steps: %v", events)
	}
	if _, e = held.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", l.Lock); e != nil {
		t.Fatal(e)
	}
	waitFor(t, "leadership", func() bool { return slices.Contains(rec.snapshot(), "run:a") })
}

// A step that finds no work pauses a second before its next run; a busy step
// runs again at once.
func TestLeaderPausesOnlyWhenIdle(t *testing.T) {
	idle := &fakeStep{name: "idle"}
	busy := &fakeStep{name: "busy", busy: true}
	l, _, _ := newLeader(t, idle, busy)
	stop := run(t, l)
	waitFor(t, "first runs", func() bool { return idle.runCount() >= 1 && busy.runCount() >= 1 })
	time.Sleep(400 * time.Millisecond)
	stop()
	if n := idle.runCount(); n != 1 {
		t.Fatalf("idle step ran %d times in 400ms", n)
	}
	if n := busy.runCount(); n < 3 {
		t.Fatalf("busy step ran only %d times in 400ms", n)
	}
}

// A deadlock or lock wait timeout is retried shortly without a warning or a
// failure backoff.
func TestLeaderRetriesTransientErrors(t *testing.T) {
	for _, number := range []uint16{1205, 1213} {
		a := &fakeStep{name: "a", fail: func(run int) error {
			if run == 1 {
				return &mariadb.MySQLError{Number: number}
			}
			return nil
		}}
		l, _, w := newLeader(t, a)
		began := time.Now()
		stop := run(t, l)
		waitFor(t, "retry", func() bool { return a.runCount() >= 2 })
		elapsed := time.Since(began)
		stop()
		if elapsed >= minBackoff {
			t.Fatalf("%d: retried after %v, want the short retry pause", number, elapsed)
		}
		if w.len() != 0 {
			t.Fatalf("%d: transient error logged as a failure: %v", number, w.msgs)
		}
	}
}

func TestBackoffDoublesToCap(t *testing.T) {
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	for i, d := range want {
		if got := backoff(i + 1); got != d {
			t.Fatalf("backoff(%d)=%v, want %v", i+1, got, d)
		}
	}
	if got := backoff(100); got != maxBackoff {
		t.Fatalf("backoff not capped: %v", got)
	}
	if errors.Is(errLockLost, context.Canceled) {
		t.Fatal("lock loss must not look like cancellation")
	}
}
