package worker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests use only Leader, Step and Starter, so the same file also
// compiles against the sequential leader it replaced (where they fail).

// journal is an ordered, shared event log.
type journal struct {
	mu     sync.Mutex
	events []string
	term   int
}

func (j *journal) add(format string, args ...any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.events = append(j.events, fmt.Sprintf(format, args...))
}

func (j *journal) list() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return slices.Clone(j.events)
}

func (j *journal) count(prefix string) int {
	n := 0
	for _, v := range j.list() {
		if strings.HasPrefix(v, prefix) {
			n++
		}
	}
	return n
}

func (j *journal) currentTerm() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.term
}

// probe is a Step whose Run is supplied by the test; it journals the term each
// run began in and when it ended.
type probe struct {
	name string
	j    *journal
	do   func(ctx context.Context, conn *sql.Conn) (bool, error)
}

func (p *probe) Name() string { return p.name }

func (p *probe) Run(ctx context.Context, conn *sql.Conn) (bool, error) {
	term := p.j.currentTerm()
	p.j.add("begin:%s:%d", p.name, term)
	defer p.j.add("end:%s:%d", p.name, term)
	return p.do(ctx, conn)
}

// recovery is a Starter standing in for term-start recovery (the provisioning
// reset): it opens a new term in the journal.
type recovery struct{ probe }

func (r *recovery) Start(context.Context, *sql.Conn) error {
	r.j.mu.Lock()
	r.j.term++
	r.j.events = append(r.j.events, fmt.Sprintf("start:%d", r.j.term))
	r.j.mu.Unlock()
	return nil
}

func throttled(busy bool, err error) func(context.Context, *sql.Conn) (bool, error) {
	return func(ctx context.Context, _ *sql.Conn) (bool, error) {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
		return busy, err
	}
}

// warnLog records the leader's warnings with their attributes.
type warnLog struct {
	mu    sync.Mutex
	msgs  []string
	errs  []error
	attrs [][]any
}

func (w *warnLog) add(_ context.Context, msg string, err error, attrs ...any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.msgs, w.errs, w.attrs = append(w.msgs, msg), append(w.errs, err), append(w.attrs, attrs)
}

func (w *warnLog) len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.msgs)
}

func leaderWith(t *testing.T, steps ...Step) (*Leader, *warnLog) {
	db, lock := lockFixture(t)
	w := &warnLog{}
	return &Leader{DB: db, Lock: lock, Warn: w.add, Steps: steps}, w
}

func killLeaderConnection(t *testing.T, l *Leader) {
	t.Helper()
	var id sql.NullInt64
	if e := l.DB.QueryRow("SELECT IS_USED_LOCK(?)", l.Lock).Scan(&id); e != nil || !id.Valid {
		t.Fatal("no leader connection", e)
	}
	if _, e := l.DB.Exec(fmt.Sprintf("KILL CONNECTION %d", id.Int64)); e != nil {
		t.Fatal(e)
	}
}

// (b) A step that keeps failing is logged with its name and backed off, while
// the other steps keep running and the leader keeps the lock (no new term).
func TestLeaderIsolatesStepErrors(t *testing.T) {
	j := &journal{}
	boom := errors.New("step failed")
	ok := &recovery{probe{name: "ok", j: j, do: throttled(true, nil)}}
	bad := &probe{name: "bad", j: j, do: throttled(false, boom)}
	l, w := leaderWith(t, ok, bad)
	stop := run(t, l)
	waitFor(t, "first failure", func() bool { return w.len() > 0 })
	okAtFailure := j.count("end:ok:")
	time.Sleep(2500 * time.Millisecond)
	var owner sql.NullInt64
	if e := l.DB.QueryRow("SELECT IS_USED_LOCK(?)", l.Lock).Scan(&owner); e != nil || !owner.Valid {
		t.Fatal("leader released the lock after a step error", e)
	}
	stop()
	if n := j.count("start:"); n != 1 {
		t.Fatalf("leadership restarted %d times after step errors", n-1)
	}
	if n := j.count("end:ok:") - okAtFailure; n < 20 {
		t.Fatalf("healthy step ran only %d times after another step failed", n)
	}
	// Backoff: failures at about 0s, 1s and 3s, not a spin.
	if n := j.count("end:bad:"); n < 2 || n > 3 {
		t.Fatalf("failing step ran %d times in ~2.5s, want backoff", n)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for i, msg := range w.msgs {
		if msg != "worker step failed" || !errors.Is(w.errs[i], boom) || !slices.Contains(w.attrs[i], any("bad")) {
			t.Fatalf("warning %q %v %v does not name the failing step", msg, w.errs[i], w.attrs[i])
		}
	}
}

// (c) A slow step (provisioning waits up to 45s plus a 5s target lock) does not
// delay other kinds of work.
func TestLeaderSlowStepDoesNotDelayOthers(t *testing.T) {
	j := &journal{}
	slowRun := make(chan struct{}, 1)
	slow := &probe{name: "slow", j: j, do: func(ctx context.Context, _ *sql.Conn) (bool, error) {
		select {
		case slowRun <- struct{}{}:
		default:
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(3 * time.Second):
		}
		return true, nil
	}}
	fast := &probe{name: "fast", j: j, do: throttled(true, nil)}
	l, _ := leaderWith(t, slow, fast)
	stop := run(t, l)
	defer stop()
	select {
	case <-slowRun:
	case <-time.After(5 * time.Second):
		t.Fatal("slow step never ran")
	}
	deadline := time.Now().Add(2 * time.Second)
	for j.count("end:fast:") < 20 {
		if time.Now().After(deadline) {
			t.Fatalf("fast step ran %d times while the slow step held its run", j.count("end:fast:"))
		}
		time.Sleep(10 * time.Millisecond)
	}
	if j.count("end:slow:") != 0 {
		t.Fatal("slow step finished before the check; the test proves nothing")
	}
}

// (d, e) Recovery runs once per term before any step of that term, and when the
// lock connection is lost every step of the old term stops (even one blocked in
// its run) before the next term's recovery; stopping the leader stops them all.
func TestLeaderTermLifecycle(t *testing.T) {
	j := &journal{}
	rec := &recovery{probe{name: "rec", j: j, do: throttled(false, nil)}}
	blocked := &probe{name: "blocked", j: j, do: func(ctx context.Context, _ *sql.Conn) (bool, error) {
		<-ctx.Done()
		return false, ctx.Err()
	}}
	busy := &probe{name: "busy", j: j, do: throttled(true, nil)}
	l, _ := leaderWith(t, rec, blocked, busy)
	stop := run(t, l)
	waitFor(t, "first term", func() bool { return j.count("begin:blocked:1") == 1 && j.count("end:busy:1") > 0 })
	killLeaderConnection(t, l)
	waitFor(t, "second term", func() bool { return j.count("begin:blocked:2") == 1 && j.count("end:busy:2") > 0 })
	stop()
	events := j.list()
	settled := len(events)
	time.Sleep(300 * time.Millisecond)
	if after := j.list(); len(after) != settled {
		t.Fatalf("steps ran after the leader stopped: %v", after[settled:])
	}
	if j.count("start:") != 2 {
		t.Fatalf("recovery ran %d times for two terms", j.count("start:"))
	}
	second := slices.Index(events, "start:2")
	if events[0] != "start:1" {
		t.Fatalf("a step ran before recovery: %v", events[:3])
	}
	open := map[string]bool{}
	for i, v := range events {
		parts := strings.Split(v, ":")
		if len(parts) != 3 {
			continue
		}
		if parts[2] == "1" && i > second {
			t.Fatalf("term 1 event %q after the second term began", v)
		}
		if parts[2] == "2" && i < second {
			t.Fatalf("term 2 event %q before its recovery", v)
		}
		key := parts[1] + ":" + parts[2]
		open[key] = parts[0] == "begin"
	}
	for key, running := range open {
		if running {
			t.Fatalf("step run %s never ended", key)
		}
	}
}
