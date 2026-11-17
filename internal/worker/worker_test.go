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

// fakeStep records its starts and runs and checks, on every run, that it was
// handed the connection that holds the leader lock.
type fakeStep struct {
	name    string
	starter bool
	busy    bool
	fail    func(run int) error
	rec     *recorder
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

func (f *fakeStep) Name() string { return f.name }

func (f *fakeStep) Run(ctx context.Context, conn *sql.Conn) (bool, error) {
	f.runs++
	f.rec.add("run:" + f.name)
	var self, owner sql.NullInt64
	if e := conn.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&self); e != nil {
		return false, e
	}
	if e := f.rec.db.QueryRowContext(ctx, "SELECT IS_USED_LOCK(?)", f.rec.lock).Scan(&owner); e != nil {
		return false, e
	}
	if !owner.Valid || owner.Int64 != self.Int64 {
		f.rec.t.Errorf("step %s ran on connection %d, lock held by %v", f.name, self.Int64, owner)
	}
	if f.fail != nil {
		if e := f.fail(f.runs); e != nil {
			return false, e
		}
	}
	return f.busy, nil
}

// startingStep is a fakeStep that also implements Starter.
type startingStep struct{ *fakeStep }

func (s startingStep) Start(ctx context.Context, conn *sql.Conn) error {
	s.rec.add("start:" + s.name)
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

// Starters run once per term in step order, then every round runs all steps
// in the listed order on the pinned connection that holds the lock.
func TestLeaderRunsStepsInOrderOnLockConnection(t *testing.T) {
	a := &fakeStep{name: "a", starter: true, busy: true}
	b := &fakeStep{name: "b"}
	c := &fakeStep{name: "c", starter: true}
	l, rec, w := newLeader(t, a, b, c)
	stop := run(t, l)
	waitFor(t, "two rounds", func() bool { return len(rec.snapshot()) >= 8 })
	stop()
	want := []string{"start:a", "start:c", "run:a", "run:b", "run:c", "run:a", "run:b", "run:c"}
	if got := rec.snapshot()[:8]; !slices.Equal(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
	if len(w.msgs) != 0 {
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

// A step error ends the term at once: later steps do not run, the lock is
// released, the failure is logged and a new term starts (Start again) a second
// later.
func TestLeaderDropsLeadershipOnStepError(t *testing.T) {
	boom := errors.New("step failed")
	a := &fakeStep{name: "a", starter: true, busy: true}
	b := &fakeStep{name: "b", busy: true, fail: func(run int) error {
		if run == 1 {
			return boom
		}
		return nil
	}}
	c := &fakeStep{name: "c"}
	l, rec, w := newLeader(t, a, b, c)
	var freeAtWarn int
	w0 := l.Warn
	l.Warn = func(ctx context.Context, msg string, err error, attrs ...any) {
		_ = l.DB.QueryRow("SELECT IS_FREE_LOCK(?)", l.Lock).Scan(&freeAtWarn)
		w0(ctx, msg, err, attrs...)
	}
	began := time.Now()
	stop := run(t, l)
	waitFor(t, "second term", func() bool {
		n := 0
		for _, v := range rec.snapshot() {
			if v == "start:a" {
				n++
			}
		}
		return n == 2
	})
	stop()
	if elapsed := time.Since(began); elapsed < time.Second {
		t.Fatalf("new term after %v, want a one-second pause", elapsed)
	}
	if got, want := rec.snapshot()[:4], []string{"start:a", "run:a", "run:b", "start:a"}; !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.msgs) != 1 || w.msgs[0] != "provisioning leader disconnected" || !errors.Is(w.errs[0], boom) {
		t.Fatalf("warnings %v %v", w.msgs, w.errs)
	}
	if freeAtWarn != 1 {
		t.Fatal("lock still held when the term ended")
	}
}

// When no step finds work the leader pauses a second between rounds; a busy
// step makes it poll again at once.
func TestLeaderPausesOnlyWhenIdle(t *testing.T) {
	for _, busy := range []bool{false, true} {
		a := &fakeStep{name: "a", busy: busy}
		l, rec, _ := newLeader(t, a)
		stop := run(t, l)
		waitFor(t, "first round", func() bool { return len(rec.snapshot()) >= 1 })
		time.Sleep(400 * time.Millisecond)
		stop()
		n := len(rec.snapshot())
		if !busy && n != 1 {
			t.Fatalf("idle leader ran %d rounds in 400ms", n)
		}
		if busy && n < 3 {
			t.Fatalf("busy leader ran only %d rounds in 400ms", n)
		}
	}
}
