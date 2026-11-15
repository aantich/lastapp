// Package data contains MariaDB storage mechanics. Business semantics live in entity.
package data

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aantich/lastapp/internal/entity"
)

func Table(id string) string      { return "`le_" + strings.ReplaceAll(id, "-", "") + "`" }
func FieldColumn(k string) string { return "`f_" + k + "`" }
func Type(f entity.Field) string {
	switch f.Type {
	case "integer":
		return "BIGINT"
	case "decimal":
		return fmt.Sprintf("DECIMAL(%d,%d)", f.Precision, f.Scale)
	case "boolean":
		return "BOOLEAN"
	case "text":
		return fmt.Sprintf("VARCHAR(%d) COLLATE utf8mb4_nopad_bin", f.MaxLength)
	case "json":
		return "JSON"
	case "reference", "user", "team":
		return "CHAR(36) CHARACTER SET ascii COLLATE ascii_bin"
	default:
		return "VARCHAR(160) COLLATE utf8mb4_nopad_bin"
	}
}

func CreateSQL(id, marker string, d entity.Definition) string { return createSQL(id, marker, d, true) }

// SkeletonSQL prepares an empty batch before cyclic foreign keys can be installed.
func SkeletonSQL(id, marker string, d entity.Definition) string {
	return createSQL(id, marker, d, false)
}

func createSQL(id, marker string, d entity.Definition, references bool) string {
	cols := []string{"`id` CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL", "`tenant_id` CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL", "`space_id` CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL", "`revision` BIGINT UNSIGNED NOT NULL DEFAULT 1", "`created_at` VARCHAR(32) NOT NULL", "`updated_at` VARCHAR(32) NOT NULL", "`created_by` CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL", "`updated_by` CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL", "PRIMARY KEY (`tenant_id`,`space_id`,`id`)"}
	for _, f := range d.Fields {
		cols = append(cols, FieldColumn(f.Key)+" "+Type(f)+Nullability(f))
		for _, index := range Indexes(f, d) {
			if references || !strings.HasPrefix(index, "FOREIGN KEY") {
				cols = append(cols, index)
			}
		}
	}
	return "CREATE TABLE " + Table(id) + " (" + strings.Join(cols, ",") + ") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_nopad_bin COMMENT='" + marker + "'"
}

func Indexes(f entity.Field, d entity.Definition) []string {
	out := []string{}
	if f.Unique != "" {
		scope := "`tenant_id`,`space_id`,"
		if f.Unique == "owner" {
			scope += FieldColumn(d.Owner) + ","
		}
		if f.Unique == "team" {
			scope += FieldColumn(d.Team) + ","
		}
		out = append(out, "UNIQUE KEY `u_"+f.Key+"` ("+scope+FieldColumn(f.Key)+")")
	} else if f.Filterable || f.Sortable || f.Key == d.Owner || f.Key == d.Team || f.Type == "reference" {
		out = append(out, "KEY `i_"+f.Key+"` (`tenant_id`,`space_id`,"+FieldColumn(f.Key)+",`id`)")
	}
	if f.Type == "reference" {
		out = append(out, "FOREIGN KEY (`tenant_id`,`space_id`,"+FieldColumn(f.Key)+") REFERENCES "+Table(f.Target)+" (`tenant_id`,`space_id`,`id`)")
	}
	return out
}

var SupportDDL = []string{
	"CREATE TABLE `_lastapp_entity_state` (entity_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,space_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,version_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,gate_open BOOLEAN NOT NULL DEFAULT 0,layout_hash CHAR(64) NOT NULL,attempt_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL) ENGINE=InnoDB",
	"CREATE TABLE `_lastapp_record_acl` (tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,space_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,entity_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,record_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,permission_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,security_revision BIGINT UNSIGNED NOT NULL,recipient_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,recipient_kind VARCHAR(8) NOT NULL,grantor_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,created_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP(6),PRIMARY KEY(entity_id,record_id,permission_id,recipient_kind,recipient_id),KEY ix_acl_recipient(tenant_id,space_id,recipient_kind,recipient_id,entity_id,permission_id,record_id),CHECK(recipient_kind IN ('user','team'))) ENGINE=InnoDB",
	"CREATE TABLE `_lastapp_data_audit` (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,space_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,entity_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,record_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,actor_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,action VARCHAR(32) NOT NULL,revision BIGINT UNSIGNED NOT NULL,fields_json JSON NOT NULL,request_id CHAR(36) NOT NULL,created_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP(6),KEY ix_audit_entity(entity_id,created_at)) ENGINE=InnoDB",
}

type poolEntry struct {
	db   *sql.DB
	key  string
	used time.Time
	refs int
}
type Pools struct {
	mu      sync.Mutex
	entries map[string]*poolEntry
	opening map[string]chan struct{}
	closed  bool
}

func (p *Pools) Acquire(ctx context.Context, key string, open func() (*sql.DB, error)) (*sql.DB, func(), error) {
	for {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return nil, nil, fmt.Errorf("Database pools are closed.")
		}
		if p.entries == nil {
			p.entries = map[string]*poolEntry{}
			p.opening = map[string]chan struct{}{}
		}
		for k, v := range p.entries {
			if v.refs == 0 && time.Since(v.used) > time.Minute {
				v.db.Close()
				delete(p.entries, k)
			}
		}
		if v := p.entries[key]; v != nil {
			v.refs++
			v.used = time.Now()
			p.mu.Unlock()
			return v.db, p.release(v), nil
		}
		if ready := p.opening[key]; ready != nil {
			p.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-ready:
				continue
			}
		}
		ready := make(chan struct{})
		p.opening[key] = ready
		p.mu.Unlock()
		// Network establishment must never hold the cache lock for other resources.
		db, e := open()
		p.mu.Lock()
		delete(p.opening, key)
		close(ready)
		if e != nil {
			p.mu.Unlock()
			return nil, nil, e
		}
		if p.closed {
			p.mu.Unlock()
			db.Close()
			return nil, nil, fmt.Errorf("Database pools are closed.")
		}
		if len(p.entries) >= 16 {
			var oldest *poolEntry
			for _, v := range p.entries {
				if v.refs == 0 && (oldest == nil || v.used.Before(oldest.used)) {
					oldest = v
				}
			}
			if oldest == nil {
				p.mu.Unlock()
				db.Close()
				return nil, nil, fmt.Errorf("Database connection capacity reached.")
			}
			oldest.db.Close()
			delete(p.entries, oldest.key)
		}
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(1)
		db.SetConnMaxIdleTime(time.Minute)
		v := &poolEntry{db: db, key: key, refs: 1, used: time.Now()}
		p.entries[key] = v
		p.mu.Unlock()
		return db, p.release(v), nil
	}
}

func (p *Pools) release(v *poolEntry) func() {
	return func() { p.mu.Lock(); v.refs--; v.used = time.Now(); p.mu.Unlock() }
}

func (p *Pools) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	for k, v := range p.entries {
		v.db.Close()
		delete(p.entries, k)
	}
}

// StoragePlan is produced only by trusted compilation and stored with a version.
// No author API accepts SQL or permits editing this projection.
type StoragePlan struct {
	Compiler   int               `json:"compiler"`
	Create     string            `json:"create"`
	FieldsHash string            `json:"fields_hash"`
	Columns    map[string]string `json:"columns"`
}

func Plan(id, marker string, d entity.Definition) StoragePlan {
	p := StoragePlan{Compiler: 1, Create: CreateSQL(id, marker, d), FieldsHash: entity.Hash(d.Fields), Columns: map[string]string{}}
	for _, f := range d.Fields {
		p.Columns[f.Key] = Type(f)
	}
	return p
}

func Nullability(f entity.Field) string {
	if f.Required {
		return " NOT NULL"
	}
	return " NULL"
}
