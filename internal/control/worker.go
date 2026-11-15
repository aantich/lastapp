package control

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	mariadb "github.com/aantich/go-mariadb"
)

// A pinned registry connection owns leadership and publishes every worker result.
func (s *Server) RunWorker(ctx context.Context) {
	for ctx.Err() == nil {
		if e := s.workerLeader(ctx); e != nil && ctx.Err() == nil {
			s.Log.Warn("provisioning leader disconnected", diagnostic(e)...)
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
	defer releasePinnedLock(conn, lock)
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
			var ae *APIError
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
		record, tenant := str(job["id"]), str(job["tenant_id"])
		jobCtx := agentAuditContext(ctx, str(job["agent_grant_id"]), str(job["agent_client_id"]))
		lastTenant = tenant
		tx, e := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if e != nil {
			return e
		}
		var active string
		e = tx.QueryRowContext(ctx, "SELECT state FROM tenants WHERE id=? FOR UPDATE", tenant).Scan(&active)
		if e == nil && str(job["agent_grant_id"]) != "" {
			res, permission := Resource{Tenant: tenant}, "databases.provision"
			ws, _ := row(ctx, tx, "SELECT w.production_space_id FROM development_workspaces w JOIN spaces sp ON sp.id=w.backing_space_id WHERE sp.database_resource_id=? AND w.state='preparing'", record)
			if ws != nil {
				res.Space = str(ws["production_space_id"])
				permission = "workspaces.write"
			}
			if denied := s.agentJobAllowed(ctx, tx, str(job["agent_grant_id"]), str(job["agent_user_id"]), res, permission, str(job["target_id"])); denied != nil {
				var ae *APIError
				if !errors.As(denied, &ae) {
					tx.Rollback()
					return denied
				}
				_, e = tx.ExecContext(ctx, "UPDATE database_resources SET state='failed',last_error_code='agent_access_revoked',last_error_message='Agent authority expired or was revoked before this job started.',revision=revision+1 WHERE id=? AND state='pending'", record)
				if e != nil {
					tx.Rollback()
					return e
				}
				if e = audit(jobCtx, tx, tenant, str(job["agent_user_id"]), "database.failed", "database", record, newID(), Row{"error_code": "agent_access_revoked"}); e != nil {
					tx.Rollback()
					return e
				}
				if e = tx.Commit(); e != nil {
					return e
				}
				continue
			}
		}
		if e == nil && active == "active" {
			_, e = tx.ExecContext(ctx, "UPDATE database_resources SET state='provisioning',attempts=attempts+1,revision=revision+1 WHERE id=? AND state='pending'", record)
		}
		if e != nil || active != "active" {
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
			s.Log.Warn("database provisioning failed", append([]any{"resource_id", record, "tenant_id", tenant}, diagnostic(err)...)...)
			code, message = provisionError(err)
			state = "failed"
		}
		tx, e = conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE database_resources SET state=?,last_error_code=?,last_error_message=?,last_verified_at=IF(?='ready',UTC_TIMESTAMP(6),last_verified_at),revision=revision+1 WHERE id=? AND state='provisioning'", state, nullable(code), nullable(message), state, record); e == nil {
			e = audit(jobCtx, tx, tenant, str(job["agent_user_id"]), "database."+state, "database", record, newID(), Row{"error_code": code})
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
	var ae *APIError
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

func ownershipConflict() error {
	return problem(409, "ownership_conflict", "Existing database or account does not match Lastapp ownership. Nothing was replaced.")
}

var (
	generatedRE   = regexp.MustCompile(`^la_[a-f0-9]{32}$`)
	accountHostRE = regexp.MustCompile(`^[a-zA-Z0-9.%:_\-]+$`)
)

func (s *Server) provision(ctx context.Context, record string) error {
	job, e := row(ctx, s.DB, "SELECT d.*,t.canonical_host,t.port,t.tls_mode,t.provisioning_credentials_enc FROM database_resources d JOIN database_targets t ON t.id=d.target_id WHERE d.id=?", record)
	if e != nil {
		return e
	}
	tenant, name := str(job["tenant_id"]), str(job["database_name"])
	var runtime credentials
	if e = s.unseal("database", record, tenant, []byte(str(job["runtime_credentials_enc"])), &runtime); e != nil {
		return e
	}
	if str(job["management_mode"]) == "registered" {
		db, e := s.targetDB(ctx, job, runtime, name)
		if e != nil {
			return e
		}
		defer db.Close()
		return validateRuntime(ctx, db, name, false)
	}
	var admin credentials
	if e = s.unseal("target", str(job["target_id"]), "", []byte(str(job["provisioning_credentials_enc"])), &admin); e != nil {
		return e
	}
	db, e := s.targetDB(ctx, job, admin, "")
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
		return problem(409, "provisioning_busy", "Another worker still owns the resource; retry shortly.")
	}
	defer releasePinnedLock(conn, "lastapp:resource:"+record)
	if !generatedRE.MatchString(name) || runtime.Username != name || !accountHostRE.MatchString(runtime.AccountHost) {
		return ownershipConflict()
	}
	marker := "lastapp:" + record + ":" + hex.EncodeToString([]byte(str(job["ownership_token"])))
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
		return ownershipConflict()
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_\-]+$`).MatchString(runtime.Password) {
		return ownershipConflict()
	}
	principal := "'" + runtime.Username + "'@'" + runtime.AccountHost + "'"
	_, e = conn.ExecContext(ctx, "CREATE USER "+principal+" IDENTIFIED BY '"+runtime.Password+"'")
	var me *mariadb.MySQLError
	if e != nil && (!errors.As(e, &me) || me.Number != 1396) {
		return e
	}
	verify, e := s.targetDB(ctx, job, runtime, "")
	if e != nil {
		return ownershipConflict()
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
	verify, e = s.targetDB(ctx, job, runtime, name)
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
		return problem(400, "unsupported_database", "This release supports MariaDB only.")
	}
	parts := strings.Split(version, ".")
	major, _ := strconv.Atoi(parts[0])
	minor := 0
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if major < 10 || (major == 10 && minor < 11) {
		return problem(400, "unsupported_version", "MariaDB 10.11 or newer is required.")
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
			return problem(400, "unsafe_grants", "Runtime credentials must not have GRANT OPTION.")
		}
		if strings.HasPrefix(grant, "GRANT USAGE ON *.* TO ") {
			continue
		}
		on, to := strings.Index(grant, " ON "), strings.Index(grant, " TO ")
		if on < 0 || to < on {
			return problem(400, "unsupported_grants", "Use explicit schema grants, rather than database roles, for runtime accounts.")
		}
		privs := strings.TrimPrefix(grant[:on], "GRANT ")
		target := grant[on+4 : to]
		if target != scope {
			return problem(400, "unsafe_grants", "Runtime credentials must be restricted to this database.")
		}
		if strings.Contains(grant, "WITH GRANT OPTION") {
			return problem(400, "unsafe_grants", "Runtime credentials must not have GRANT OPTION.")
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
					return ownershipConflict()
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
				return problem(400, "insufficient_privileges", "Runtime credentials require SELECT, INSERT, UPDATE and DELETE on the complete database.")
			}
		}
	}
	return nil
}

// Never return a session with uncertain advisory-lock ownership to the pool.
func releasePinnedLock(conn *sql.Conn, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var released sql.NullInt64
	if e := conn.QueryRowContext(ctx, "SELECT RELEASE_LOCK(?)", name).Scan(&released); e != nil || !released.Valid || released.Int64 != 1 {
		conn.Raw(func(any) error { return driver.ErrBadConn })
	}
}

// Round-robin Tenants, oldest job within each Tenant. A slow Tenant can consume
// at most one 45-second attempt before another waiting Tenant gets a turn.
func nextJob(ctx context.Context, q Queryer, lastTenant string) (Row, error) {
	return row(ctx, q, "SELECT d.id,d.tenant_id,d.target_id,d.agent_grant_id,d.agent_user_id,ag.client_id AS agent_client_id FROM database_resources d JOIN tenants t ON t.id=d.tenant_id JOIN database_targets dt ON dt.id=d.target_id LEFT JOIN agent_grants ag ON ag.id=d.agent_grant_id WHERE d.state='pending' AND t.state='active' AND dt.state='active' ORDER BY (d.tenant_id>?) DESC,d.tenant_id,d.created_at,d.id LIMIT 1", lastTenant)
}

func cleanupSessions(ctx context.Context, q Queryer) (sql.Result, error) {
	return q.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at<=UTC_TIMESTAMP(6) LIMIT 1000")
}
