package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/entities"
	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/registry"
)

// versionNumbersMigration is the migration that adds entity version numbers.
const versionNumbersMigration = 15

// The version-number migration numbers existing versions per entity in
// creation order (created_at, then id), whatever their state.
func TestVersionNumberBackfill(t *testing.T) {
	q, db := registryAt(t, versionNumbersMigration-1)
	ctx := context.Background()
	first, second := registry.NewID(), registry.NewID()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	insert := func(id, entityID, state string, at time.Time) {
		exec(t, q, "INSERT INTO entity_versions(id,tenant_id,space_id,entity_id,definition_json,definition_hash,created_by,state,created_at) VALUES(?,?,?,?,'{}',?,?,?,?)", id, registry.NewID(), registry.NewID(), entityID, entity.Hash(entity.Definition{}), registry.NewID(), state, at)
	}
	// Inserted out of order; two share a timestamp and fall back to id order.
	insert("00000000-0000-4000-8000-000000000003", first, "published", base.Add(2*time.Hour))
	insert("00000000-0000-4000-8000-000000000002", first, "failed", base.Add(time.Hour))
	insert("00000000-0000-4000-8000-000000000001", first, "published", base.Add(time.Hour))
	insert("00000000-0000-4000-8000-000000000004", first, "queued", base.Add(3*time.Hour))
	insert("00000000-0000-4000-8000-000000000005", second, "published", base)
	if e := goose.UpToContext(ctx, db, migrationsDir, versionNumbersMigration); e != nil {
		t.Fatal(e)
	}
	want := map[string]int64{
		"00000000-0000-4000-8000-000000000001": 1,
		"00000000-0000-4000-8000-000000000002": 2,
		"00000000-0000-4000-8000-000000000003": 3,
		"00000000-0000-4000-8000-000000000004": 4,
		"00000000-0000-4000-8000-000000000005": 1,
	}
	for id, n := range want {
		var got int64
		if e := q.QueryRowContext(ctx, "SELECT version_number FROM entity_versions WHERE id=?", id).Scan(&got); e != nil || got != n {
			t.Fatalf("version %s numbered %d, want %d (%v)", id, got, n, e)
		}
	}
	if _, e := q.ExecContext(ctx, "INSERT INTO entity_versions(id,tenant_id,space_id,entity_id,version_number,definition_json,definition_hash,created_by) VALUES(?,?,?,?,2,'{}','',?)", registry.NewID(), registry.NewID(), registry.NewID(), first, registry.NewID()); e == nil {
		t.Fatal("a version number was reused")
	}
}

// Concurrent captures of one entity get distinct, increasing numbers in the
// order they commit, even when each transaction read the entity's versions
// before waiting. A failed version keeps its number, so the next one skips it.
func TestVersionNumbersConcurrent(t *testing.T) {
	q, db := registryAt(t, 0)
	ctx := context.Background()
	d := entity.Templates()[0]
	draft, _ := json.Marshal(d)
	tenant, space, id := registry.NewID(), registry.NewID(), registry.NewID()
	exec(t, q, "INSERT INTO entity_definitions(id,tenant_id,space_id,entity_key,label,draft_json) VALUES(?,?,?,?,?,?)", id, tenant, space, d.Key, d.Label, draft)
	var stored registry.JSONValue
	if e := stored.Scan(draft); e != nil {
		t.Fatal(e)
	}
	s := &Service{RegistryName: "versions_test"}
	queue := func(before func()) (int64, error) {
		// Read committed, as op.Runner.Mutate (the authoring endpoint and MCP) runs it.
		tx, e := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if e != nil {
			return 0, e
		}
		defer func() { _ = tx.Rollback() }()
		if _, e = tx.ExecContext(ctx, "SET SESSION foreign_key_checks=0"); e != nil {
			return 0, e
		}
		// Read the entity's versions before waiting.
		var seen int64
		if e = tx.QueryRowContext(ctx, entities.NextVersionNumber, id).Scan(&seen); e != nil {
			return 0, e
		}
		before()
		version := registry.NewID()
		if e = s.QueueDefinition(ctx, tx, access.Principal{ID: registry.NewID()}, entities.EntityDefinition{ID: id, TenantID: tenant, SpaceID: space, Draft: stored}, version, nil); e != nil {
			return 0, e
		}
		var n int64
		if e = tx.QueryRowContext(ctx, "SELECT version_number FROM entity_versions WHERE id=?", version).Scan(&n); e != nil {
			return 0, e
		}
		return n, tx.Commit()
	}
	const workers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	numbers := make([]int64, workers)
	errs := make([]error, workers)
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			numbers[i], errs[i] = queue(func() { <-start })
		}()
	}
	close(start)
	wg.Wait()
	seen := map[int64]bool{}
	for i, n := range numbers {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if n < 1 || n > workers || seen[n] {
			t.Fatalf("concurrent captures numbered %v", numbers)
		}
		seen[n] = true
	}
	var ordered int
	if e := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM entity_versions a JOIN entity_versions b ON a.entity_id=b.entity_id AND a.version_number<b.version_number AND a.created_at>b.created_at WHERE a.entity_id=?", id).Scan(&ordered); e != nil || ordered != 0 {
		t.Fatalf("a later number was created first (%d, %v)", ordered, e)
	}
	exec(t, q, "UPDATE entity_versions SET state='failed' WHERE entity_id=? AND version_number=?", id, workers)
	if n, e := queue(func() {}); e != nil || n != workers+1 {
		t.Fatalf("after a failed v%d the next version is v%d (%v)", workers, n, e)
	}
}
