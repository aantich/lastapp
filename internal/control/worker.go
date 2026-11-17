package control

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/databases"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

// A pinned registry connection owns leadership and publishes every worker result.
func (s *Server) RunWorker(ctx context.Context) {
	for ctx.Err() == nil {
		if e := s.workerLeader(ctx); e != nil && ctx.Err() == nil {
			httpapi.Logger(ctx, s.Log).Warn("provisioning leader disconnected", httpapi.Diagnostic(e)...)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (s *Server) workerLeader(ctx context.Context) error {
	conn, e := s.DB.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	lock := "lastapp:" + s.registryName
	var acquired int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", lock).Scan(&acquired); e != nil {
		return e
	}
	if acquired != 1 {
		return nil
	}
	defer data.ReleasePinnedLock(conn, lock)
	if _, e = conn.ExecContext(ctx, "UPDATE database_resources SET state='pending',revision=revision+1 WHERE state='provisioning'"); e != nil {
		return e
	}
	lastTenant := ""
	lastPublicationTenant := ""
	cleanedAt := time.Time{}
	for ctx.Err() == nil {
		if time.Since(cleanedAt) > time.Minute {
			if _, e = cleanupSessions(ctx, conn); e != nil {
				return e
			}
			if _, e = conn.ExecContext(ctx, "DELETE FROM agent_token_sessions WHERE grant_id IN (SELECT id FROM agent_grants WHERE state='revoked' OR expires_at<=UTC_TIMESTAMP(6)) LIMIT 1000"); e != nil {
				return e
			}
			if _, e = conn.ExecContext(ctx, "DELETE FROM agent_oauth_requests WHERE expires_at<=UTC_TIMESTAMP(6) LIMIT 1000"); e != nil {
				return e
			}
			if _, e = conn.ExecContext(ctx, "DELETE FROM artifact_uploads WHERE expires_at<=UTC_TIMESTAMP(6) LIMIT 1000"); e != nil {
				return e
			}
			if e = cleanupAgentClients(ctx, conn); e != nil {
				return e
			}
			cleanedAt = time.Now()
		}
		if e = s.deploymentJob(ctx, conn); e != nil {
			return e
		}
		if e = s.workspaceJob(ctx, conn); e != nil {
			return e
		}
		if _, err := s.publicationJob(ctx, conn, &lastPublicationTenant); err != nil {
			return err
		}
		job, e := nextJob(ctx, conn, lastTenant)
		if e != nil {
			var ae *registry.APIError
			if !errors.As(e, &ae) || ae.Status != 404 {
				return e
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
			continue
		}
		record, tenant := job.ID, job.TenantID
		jobCtx := op.AgentAuditContext(ctx, job.AgentGrant, job.AgentClient)
		lastTenant = tenant
		tx, e := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if e != nil {
			return e
		}
		var active string
		e = tx.QueryRowContext(ctx, "SELECT state FROM tenants WHERE id=? FOR UPDATE", tenant).Scan(&active)
		if e == nil && job.AgentGrant != "" {
			res, permission := access.Resource{Tenant: tenant}, "databases.provision"
			var production string
			if registry.Lookup(ctx, tx, "SELECT w.production_space_id FROM development_workspaces w JOIN spaces sp ON sp.id=w.backing_space_id WHERE sp.database_resource_id=? AND w.state='preparing'", []any{record}, &production) == nil {
				res.Space = production
				permission = "workspaces.write"
			}
			if denied := s.AgentJobAllowed(ctx, tx, job.AgentGrant, job.AgentUser, res, permission, job.TargetID); denied != nil {
				var ae *registry.APIError
				if !errors.As(denied, &ae) {
					tx.Rollback()
					return denied
				}
				_, e = tx.ExecContext(ctx, "UPDATE database_resources SET state='failed',last_error_code='agent_access_revoked',last_error_message='Agent authority expired or was revoked before this job started.',revision=revision+1 WHERE id=? AND state='pending'", record)
				if e != nil {
					tx.Rollback()
					return e
				}
				if e = op.Audit(jobCtx, tx, tenant, job.AgentUser, "database.failed", "database", record, registry.NewID(), registry.Row{"error_code": "agent_access_revoked"}); e != nil {
					tx.Rollback()
					return e
				}
				if e = tx.Commit(); e != nil {
					return e
				}
				continue
			}
		}
		if e == nil && active == registry.StateActive {
			_, e = tx.ExecContext(ctx, "UPDATE database_resources SET state='provisioning',attempts=attempts+1,revision=revision+1 WHERE id=? AND state='pending'", record)
		}
		if e != nil || active != registry.StateActive {
			tx.Rollback()
			if e != nil {
				return e
			}
			continue
		}
		if e = tx.Commit(); e != nil {
			return e
		}
		attempt, cancel := context.WithTimeout(ctx, 45*time.Second)
		err := s.provision(attempt, record)
		cancel()
		code, message, state := "", "", "ready"
		if err != nil {
			httpapi.Logger(ctx, s.Log).Warn("database provisioning failed", append([]any{"resource_id", record, "tenant_id", tenant}, httpapi.Diagnostic(err)...)...)
			code, message = provisionError(err)
			state = "failed"
		}
		tx, e = conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE database_resources SET state=?,last_error_code=?,last_error_message=?,last_verified_at=IF(?='ready',UTC_TIMESTAMP(6),last_verified_at),revision=revision+1 WHERE id=? AND state='provisioning'", state, registry.Nullable(code), registry.Nullable(message), state, record); e == nil {
			e = op.Audit(jobCtx, tx, tenant, job.AgentUser, "database."+state, "database", record, registry.NewID(), registry.Row{"error_code": code})
		}
		if e != nil {
			tx.Rollback()
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
	}
	return ctx.Err()
}

func provisionError(e error) (string, string) {
	var ae *registry.APIError
	if errors.As(e, &ae) {
		return ae.Code, ae.Message
	}
	var me *mariadb.MySQLError
	if errors.As(e, &me) {
		switch me.Number {
		case 1045:
			return "authentication_failed", "The database rejected the supplied credentials."
		case 1044, 1142, 1227:
			return "insufficient_privileges", "The database account does not have the required privileges."
		case 1049:
			return "database_missing", "The registered database does not exist."
		case 1396:
			return "account_conflict", "The generated account cannot be verified."
		}
	}
	return "connection_or_provisioning_failed", "Could not connect or provision. Check the target, credentials, TLS and network policy, then retry."
}

var (
	generatedRE   = regexp.MustCompile(`^la_[a-f0-9]{32}$`)
	accountHostRE = regexp.MustCompile(`^[a-zA-Z0-9.%:_\-]+$`)
)

func (s *Server) provision(ctx context.Context, record string) error {
	job, e := registry.ScanOne[databases.ResourceAccess](ctx, s.DB, "SELECT "+databases.ResourceAccessColumns+" FROM database_resources d JOIN database_targets t ON t.id=d.target_id WHERE d.id=?", record)
	if e != nil {
		return e
	}
	tenant, name := job.TenantID, job.Name
	var runtime databases.Credentials
	if e = s.sealer.Unseal("database", record, tenant, job.Runtime, &runtime); e != nil {
		return e
	}
	if job.Mode == "registered" {
		db, e := s.targetDB(ctx, job.DatabaseEndpoint, runtime, name)
		if e != nil {
			return e
		}
		defer db.Close()
		return validateRuntime(ctx, db, name, false)
	}
	var admin databases.Credentials
	if e = s.sealer.Unseal("target", job.TargetID, "", job.Provisioning, &admin); e != nil {
		return e
	}
	db, e := s.targetDB(ctx, job.DatabaseEndpoint, admin, "")
	if e != nil {
		return e
	}
	defer db.Close()
	conn, e := db.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	var got int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,5)", "lastapp:resource:"+record).Scan(&got); e != nil {
		return e
	}
	if got != 1 {
		return registry.Problem(409, "provisioning_busy", "Another worker still owns the resource; retry shortly.")
	}
	defer data.ReleasePinnedLock(conn, "lastapp:resource:"+record)
	if !generatedRE.MatchString(name) || runtime.Username != name || !accountHostRE.MatchString(runtime.AccountHost) {
		return databases.OwnershipConflict()
	}
	marker := "lastapp:" + record + ":" + hex.EncodeToString(job.Ownership)
	var comment string
	e = conn.QueryRowContext(ctx, "SELECT SCHEMA_COMMENT FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", name).Scan(&comment)
	if errors.Is(e, sql.ErrNoRows) {
		_, e = conn.ExecContext(ctx, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci COMMENT='"+marker+"'")
		if e != nil {
			return e
		}
	} else if e != nil {
		return e
	} else if comment != marker {
		return databases.OwnershipConflict()
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_\-]+$`).MatchString(runtime.Password) {
		return databases.OwnershipConflict()
	}
	principal := "'" + runtime.Username + "'@'" + runtime.AccountHost + "'"
	_, e = conn.ExecContext(ctx, "CREATE USER "+principal+" IDENTIFIED BY '"+runtime.Password+"'")
	var me *mariadb.MySQLError
	if e != nil && (!errors.As(e, &me) || me.Number != 1396) {
		return e
	}
	verify, e := s.targetDB(ctx, job.DatabaseEndpoint, runtime, "")
	if e != nil {
		return databases.OwnershipConflict()
	}
	if e = checkGrants(ctx, verify, name, true, false); e != nil {
		verify.Close()
		return e
	}
	verify.Close()
	pattern := strings.ReplaceAll(name, "_", `\_`)
	if _, e = conn.ExecContext(ctx, "GRANT SELECT,INSERT,UPDATE,DELETE ON `"+pattern+"`.* TO "+principal); e != nil {
		return e
	}
	verify, e = s.targetDB(ctx, job.DatabaseEndpoint, runtime, name)
	if e != nil {
		return e
	}
	defer verify.Close()
	return validateRuntime(ctx, verify, name, true)
}

func validateRuntime(ctx context.Context, db *sql.DB, name string, strict bool) error {
	var version string
	if e := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); e != nil {
		return e
	}
	if !strings.Contains(version, "MariaDB") {
		return registry.Problem(400, "unsupported_database", "This release supports MariaDB only.")
	}
	parts := strings.Split(version, ".")
	major, _ := strconv.Atoi(parts[0])
	minor := 0
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if major < 10 || (major == 10 && minor < 11) {
		return registry.Problem(400, "unsupported_version", "MariaDB 10.11 or newer is required.")
	}
	return checkGrants(ctx, db, name, strict, true)
}

func checkGrants(ctx context.Context, db *sql.DB, name string, strict, required bool) error {
	rs, e := db.QueryContext(ctx, "SHOW GRANTS")
	if e != nil {
		return e
	}
	defer rs.Close()
	have := map[string]bool{}
	scope := "`" + strings.ReplaceAll(name, "_", `\_`) + "`.*"

	for rs.Next() {
		var grant string
		if e = rs.Scan(&grant); e != nil {
			return e
		}
		if strings.Contains(grant, "WITH GRANT OPTION") {
			return registry.Problem(400, "unsafe_grants", "Runtime credentials must not have GRANT OPTION.")
		}
		if strings.HasPrefix(grant, "GRANT USAGE ON *.* TO ") {
			continue
		}
		on, to := strings.Index(grant, " ON "), strings.Index(grant, " TO ")
		if on < 0 || to < on {
			return registry.Problem(400, "unsupported_grants", "Use explicit schema grants, rather than database roles, for runtime accounts.")
		}
		privs := strings.TrimPrefix(grant[:on], "GRANT ")
		target := grant[on+4 : to]
		if target != scope {
			return registry.Problem(400, "unsafe_grants", "Runtime credentials must be restricted to this database.")
		}
		if strings.Contains(grant, "WITH GRANT OPTION") {
			return registry.Problem(400, "unsafe_grants", "Runtime credentials must not have GRANT OPTION.")
		}
		if privs == "ALL PRIVILEGES" && !strict {
			for _, p := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
				have[p] = true
			}
			continue
		}
		for _, v := range strings.Split(privs, ", ") {
			switch v {
			case "SELECT", "INSERT", "UPDATE", "DELETE":
				have[v] = true
			default:
				if strict {
					return databases.OwnershipConflict()
				}
			}
		}
	}
	if e = rs.Err(); e != nil {
		return e
	}
	if required {
		for _, v := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
			if !have[v] {
				return registry.Problem(400, "insufficient_privileges", "Runtime credentials require SELECT, INSERT, UPDATE and DELETE on the complete database.")
			}
		}
	}
	return nil
}

// Round-robin Tenants, oldest job within each Tenant. A slow Tenant can consume
// at most one 45-second attempt before another waiting Tenant gets a turn.
// databaseJob is a pending database resource and the delegation, if any,
// that requested it.
type databaseJob struct {
	ID, TenantID, TargetID             string
	AgentGrant, AgentUser, AgentClient string // empty for a console request
}

func (v *databaseJob) ScanFields() []any {
	return []any{&v.ID, &v.TenantID, &v.TargetID, &v.AgentGrant, &v.AgentUser, &v.AgentClient}
}

func nextJob(ctx context.Context, q registry.Queryer, lastTenant string) (databaseJob, error) {
	return registry.ScanOne[databaseJob](ctx, q, "SELECT d.id,d.tenant_id,d.target_id,COALESCE(d.agent_grant_id,''),COALESCE(d.agent_user_id,''),COALESCE(ag.client_id,'') FROM database_resources d JOIN tenants t ON t.id=d.tenant_id JOIN database_targets dt ON dt.id=d.target_id LEFT JOIN agent_grants ag ON ag.id=d.agent_grant_id WHERE d.state='pending' AND t.state='active' AND dt.state='active' ORDER BY (d.tenant_id>?) DESC,d.tenant_id,d.created_at,d.id LIMIT 1", lastTenant)
}

func cleanupSessions(ctx context.Context, q registry.Queryer) (sql.Result, error) {
	return q.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at<=UTC_TIMESTAMP(6) LIMIT 1000")
}
