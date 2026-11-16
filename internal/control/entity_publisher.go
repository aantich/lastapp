package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/entity"
)

func (s *Server) businessDB(ctx context.Context, resource string, schema bool) (*sql.DB, error) {
	v, e := row(ctx, s.DB, "SELECT d.*,t.canonical_host,t.port,t.tls_mode,t.provisioning_credentials_enc,t.state AS target_state FROM database_resources d JOIN database_targets t ON t.id=d.target_id WHERE d.id=? AND d.state='ready' AND t.state='active'", resource)
	if e != nil {
		return nil, e
	}
	var cred credentials
	purpose, record, tenant, enc := "database", resource, str(v["tenant_id"]), v["runtime_credentials_enc"]
	if schema {
		if v["management_mode"] == "managed" {
			purpose, record, tenant, enc = "target", str(v["target_id"]), "", v["provisioning_credentials_enc"]
		} else {
			purpose, enc = "schema", v["schema_credentials_enc"]
			if enc == nil {
				return nil, bad("Storage setup required: a Tenant administrator must supply separate schema credentials.")
			}
		}
	}
	if e = s.unseal(purpose, record, tenant, []byte(str(enc)), &cred); e != nil {
		return nil, e
	}
	return s.targetDB(ctx, v, cred, str(v["database_name"]))
}

func (s *Server) runtimeDB(ctx context.Context, resource string) (*sql.DB, func(), error) {
	v, e := row(ctx, s.DB, "SELECT d.credential_generation FROM database_resources d JOIN database_targets t ON t.id=d.target_id AND t.state='active' WHERE d.id=? AND d.state='ready'", resource)
	if e != nil {
		return nil, nil, e
	}
	return s.dataPools.Acquire(ctx, resource+":"+str(v["credential_generation"]), func() (*sql.DB, error) { return s.businessDB(ctx, resource, false) })
}

func (s *Server) publicationJob(ctx context.Context, q Queryer, cursor ...*string) (bool, error) {
	last := ""
	if len(cursor) > 0 {
		last = *cursor[0]
	}
	// A newly created Space is authorable before managed provisioning finishes.
	// Do not fail its queued definition merely because storage is still starting;
	// skip it so this same worker can provision storage and return to publication.
	v, e := row(ctx, q, "SELECT v.*,ag.client_id AS agent_client_id,e.active_version_id,e.pending_version_id,sp.database_resource_id FROM entity_versions v JOIN entity_definitions e ON e.id=v.entity_id JOIN spaces sp ON sp.id=v.space_id AND sp.state='active' JOIN database_resources d ON d.id=sp.database_resource_id JOIN tenants t ON t.id=v.tenant_id AND t.state='active' LEFT JOIN agent_grants ag ON ag.id=v.agent_grant_id WHERE v.state IN ('queued','publishing') AND (v.state='publishing' OR d.state NOT IN ('pending','provisioning')) ORDER BY (v.tenant_id>?) DESC,v.tenant_id,v.created_at,v.id LIMIT 1", last)
	if e != nil {
		if ae, ok := e.(*APIError); ok && ae.Status == 404 {
			return false, nil
		}
		return false, e
	}
	ctx = agentAuditContext(ctx, str(v["agent_grant_id"]), str(v["agent_client_id"]))
	workspace, _ := row(ctx, q, "SELECT * FROM development_workspaces WHERE backing_space_id=?", v["space_id"])
	if workspace != nil && workspace["state"] == "preparing" {
		ctx = context.WithValue(ctx, workspaceOperationKey{}, str(workspace["id"]))
	}

	if len(cursor) > 0 {
		*cursor[0] = str(v["tenant_id"])
	}
	attempt := newID()
	if v["state"] == "queued" && (str(v["agent_grant_id"]) != "" || workspace != nil || v["deployment_id"] != nil) {
		tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			return true, err
		}
		defer tx.Rollback()
		var locked string
		if err = tx.QueryRowContext(ctx, "SELECT id FROM tenants WHERE id=? FOR UPDATE", v["tenant_id"]).Scan(&locked); err != nil {
			return true, err
		}
		permission := "entities.publish"
		if v["deployment_id"] != nil {
			permission = "production.deploy"
		}
		denied := s.agentJobAllowed(ctx, tx, str(v["agent_grant_id"]), str(v["created_by"]), Resource{Tenant: str(v["tenant_id"]), Space: str(v["space_id"])}, permission)
		if denied == nil && v["deployment_id"] != nil {
			job, err := row(ctx, tx, "SELECT * FROM release_deployments WHERE id=? AND state='publishing'", v["deployment_id"])
			denied = err
			if denied == nil {
				_, denied = s.deploymentPrincipal(ctx, tx, job)
			}
		}
		if denied == nil && workspace != nil {
			actor := Principal{ID: str(workspace["owner_user_id"])}
			if str(workspace["owner_agent_grant_id"]) != "" {
				actor.Agent, denied = s.readAgentGrant(ctx, tx, str(workspace["owner_agent_grant_id"]))
				if denied == nil {
					denied = requireAgent(actor.Agent, "workspaces.write")
				}
			}
			if denied == nil {
				denied = s.workspaceOwner(ctx, tx, actor, workspace)
			}
		}

		if denied != nil {
			var ae *APIError
			if !errors.As(denied, &ae) {
				return true, denied
			}
			if _, err = tx.ExecContext(ctx, "UPDATE entity_versions SET state='failed',error_message='Agent authority expired or was revoked before this job started.',finished_at=UTC_TIMESTAMP(6) WHERE id=? AND state='queued'", v["id"]); err != nil {
				return true, err
			}
			if _, err = tx.ExecContext(ctx, "UPDATE entity_definitions SET pending_version_id=NULL,revision=revision+1 WHERE id=? AND pending_version_id=?", v["entity_id"], v["id"]); err != nil {
				return true, err
			}
			if err = audit(ctx, tx, str(v["tenant_id"]), str(v["created_by"]), "entity.publication_failed", "entity", str(v["entity_id"]), newID(), Row{"version": v["id"], "error_code": "agent_access_revoked"}); err != nil {
				return true, err
			}
			return true, tx.Commit()
		}
		if _, err = tx.ExecContext(ctx, "UPDATE entity_versions SET state='publishing',attempt_id=?,error_message=NULL WHERE id=?", attempt, v["id"]); err != nil {
			return true, err
		}
		if err = tx.Commit(); err != nil {
			return true, err
		}
	} else if _, e = q.ExecContext(ctx, "UPDATE entity_versions SET state='publishing',attempt_id=?,error_message=NULL WHERE id=?", attempt, v["id"]); e != nil {
		return true, e
	}
	v["attempt_id"] = attempt
	run, cancel := context.WithTimeout(ctx, 45*time.Second)
	e = s.publishEntity(run, v)
	cancel()
	if e == nil {
		return true, nil
	}
	s.Log.Warn("entity publication failed", append([]any{"entity_id", v["entity_id"]}, diagnostic(e)...)...)
	message := "Publication failed. Check database/schema privileges and retry; existing data is preserved."
	if ae, ok := e.(*APIError); ok {
		message = ae.Message
	}
	var active string
	if err := s.DB.QueryRowContext(ctx, "SELECT COALESCE(active_version_id,'') FROM entity_definitions WHERE id=?", v["entity_id"]).Scan(&active); err != nil {
		return true, err
	}
	if active == str(v["id"]) { // Activation committed: keep the durable job recoverable until opening succeeds.
		_, err := s.DB.ExecContext(ctx, "UPDATE entity_versions SET error_message=? WHERE id=? AND attempt_id=?", message, v["id"], attempt)
		return true, err
	}
	s.restorePublicationBase(ctx, v)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return true, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE entity_versions SET state='failed',error_message=?,finished_at=UTC_TIMESTAMP(6) WHERE id=? AND attempt_id=? AND state<>'published'", message, v["id"], attempt); err != nil {
		return true, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE entity_definitions SET pending_version_id=NULL,revision=revision+1 WHERE id=? AND pending_version_id=? AND (active_version_id IS NULL OR active_version_id<>?)", v["entity_id"], v["id"], v["id"])
	if err != nil {
		return true, err
	}
	if err = audit(ctx, tx, str(v["tenant_id"]), str(v["created_by"]), "entity.publication_failed", "entity", str(v["entity_id"]), newID(), Row{"version": v["id"]}); err != nil {
		return true, err
	}
	return true, tx.Commit()
}

func (s *Server) publishEntity(ctx context.Context, v Row) error {
	d, e := defOf(v["definition_json"])
	if e != nil {
		return e
	}
	var plan data.StoragePlan
	if integer(v["compiler_version"]) != 1 || v["storage_plan_json"] == nil {
		return bad("Publication compiler/plan is unsupported; review and publish a new version.")
	}
	planJSON, _ := json.Marshal(v["storage_plan_json"])
	if e = json.Unmarshal(planJSON, &plan); e != nil {
		return e
	}
	if entity.Hash(plan) != str(v["storage_plan_hash"]) || entity.Hash(d) != str(v["definition_hash"]) {
		return ownershipConflict()
	}
	entityID, version, attempt := str(v["entity_id"]), str(v["id"]), str(v["attempt_id"])
	db, e := s.businessDB(ctx, str(v["database_resource_id"]), true)
	if e != nil {
		return e
	}
	defer db.Close()
	conn, e := db.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	lock := "lastapp:entity:" + entityID
	var got int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,1)", lock).Scan(&got); e != nil {
		return e
	}
	if got != 1 {
		return conflict("Entity storage is busy; retry shortly.")
	}
	defer releasePinnedLock(conn, lock)
	marker := "lastapp:" + entity.Hash(s.registryName)[:16]
	if e = s.ensureSupport(ctx, conn, marker); e != nil {
		return e
	}
	table := strings.Trim(data.Table(entityID), "`")
	tableMarker := marker + ":" + entityID
	var comment string
	e = conn.QueryRowContext(ctx, "SELECT TABLE_COMMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?", table).Scan(&comment)
	exists := e == nil
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	if exists && comment != tableMarker {
		return ownershipConflict()
	}
	_, e = conn.ExecContext(ctx, "INSERT INTO _lastapp_entity_state(entity_id,tenant_id,space_id,layout_hash) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE entity_id=entity_id", entityID, v["tenant_id"], v["space_id"], entity.Hash(d.Fields))
	if e != nil {
		return e
	}
	tx, e := conn.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	var tenant, space string
	e = tx.QueryRowContext(ctx, "SELECT tenant_id,space_id FROM _lastapp_entity_state WHERE entity_id=? FOR UPDATE", entityID).Scan(&tenant, &space)
	if e == nil && (tenant != str(v["tenant_id"]) || space != str(v["space_id"])) {
		e = ownershipConflict()
	}
	if e == nil {
		_, e = tx.ExecContext(ctx, "UPDATE _lastapp_entity_state SET gate_open=0,attempt_id=? WHERE entity_id=?", attempt, entityID)
	}
	if e != nil {
		tx.Rollback()
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	// Additive preparation is resumable; never drop an existing table/column.
	var old entity.Definition
	if v["base_version_id"] != nil {
		r, e := row(ctx, s.DB, "SELECT definition_json FROM entity_versions WHERE id=?", v["base_version_id"])
		if e != nil {
			return e
		}
		old, e = defOf(r["definition_json"])
		if e != nil {
			return e
		}
	}
	alreadyActivated := str(v["active_version_id"]) == version
	if exists && !alreadyActivated {
		var hasRows bool
		if e = conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM "+data.Table(entityID)+" LIMIT 1)").Scan(&hasRows); e != nil {
			return e
		}
		count := 0
		if hasRows {
			count = 1
		}
		oldKeys := map[string]bool{}
		for _, f := range old.Fields {
			oldKeys[f.Key] = true
			n, ok := d.Field(f.Key)
			if !ok {
				return bad("Removing published fields requires a future destructive-change workflow.")
			}
			if f.Type != n.Type || f.MaxLength != n.MaxLength || f.Precision != n.Precision || f.Scale != n.Scale || f.Target != n.Target || f.Unique != n.Unique || f.Filterable != n.Filterable || f.Sortable != n.Sortable {
				return bad("Changing a published field's storage/index definition is not supported yet.")
			}
			if count > 0 && entity.Hash(f.Calculation) != entity.Hash(n.Calculation) {
				return bad("Changing stored calculations on existing records requires a backfill.")
			}
		}
		for _, f := range d.Fields {
			if oldKeys[f.Key] {
				continue
			}
			if count > 0 && (f.Required || f.Calculation != nil) {
				return bad("Adding required/calculated fields to existing records requires a backfill.")
			}
			var n int
			if e = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?", table, "f_"+f.Key).Scan(&n); e != nil {
				return e
			}
			if n == 0 {
				clauses := []string{"ADD COLUMN " + data.FieldColumn(f.Key) + " " + data.Type(f) + data.Nullability(f)}
				for _, idx := range data.Indexes(f, d) {
					if !strings.HasPrefix(idx, "FOREIGN KEY") {
						clauses = append(clauses, "ADD "+idx)
					}
				}
				if _, e = conn.ExecContext(ctx, "ALTER TABLE "+data.Table(entityID)+" "+strings.Join(clauses, ",")+", ALGORITHM=INPLACE, LOCK=NONE"); e != nil {
					return e
				}
			}
		}
		if entity.NeedsRecordValidation(old, d) {
			after := ""
			processed := 0
			batchSize := entity.MaxReadResponseBytes / entity.EstimatedRecordBytes(d.Fields)
			if batchSize < 1 {
				batchSize = 1
			}
			if batchSize > 64 {
				batchSize = 64
			}
			for {
				records, e := rows(ctx, conn, "SELECT * FROM "+data.Table(entityID)+" r WHERE tenant_id=? AND space_id=? AND id>? ORDER BY id LIMIT ?", tenant, space, after, batchSize)
				if e != nil {
					return e
				}
				if len(records) == 0 {
					break
				}
				for _, raw := range records {
					processed++
					if processed > entity.MaxPublicationValidationRecords {
						return bad("Existing-data validation exceeds the current publication limit. Policy-only changes remain available.")
					}
					record := businessRecord(d, raw)
					for _, field := range d.Fields {
						if _, e := entity.Value(field, record[field.Key], true); e != nil {
							return bad("Existing records do not satisfy the proposed field constraints.")
						}
					}
					for _, rule := range d.Rules {
						if rule.Kind == "validation" {
							c := d.Bind(entity.Context{Record: record})
							if entity.Allows(rule.When, c) && !entity.Allows(&rule.Assert, c) {
								return bad("Existing records do not satisfy the proposed validation rules.")
							}
						}
					}
					after = str(raw["id"])
				}
				if len(records) < batchSize {
					break
				}
			}
		}

	} else if !exists {
		if alreadyActivated {
			return ownershipConflict()
		}
		if _, e = conn.ExecContext(ctx, plan.Create); e != nil {
			return e
		}
	}
	for _, f := range d.Fields {
		var nullability string
		if e = conn.QueryRowContext(ctx, "SELECT IS_NULLABLE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?", table, "f_"+f.Key).Scan(&nullability); e != nil {
			return e
		}
		if (nullability == "NO") != f.Required {
			if _, e = conn.ExecContext(ctx, "ALTER TABLE "+data.Table(entityID)+" MODIFY COLUMN "+data.FieldColumn(f.Key)+" "+data.Type(f)+data.Nullability(f)+", ALGORITHM=INPLACE, LOCK=NONE"); e != nil {
				return e
			}
		}
	}
	if e = ensureReferenceConstraints(ctx, conn, entityID, d); e != nil {
		return e
	}
	if e = verifyEntityColumns(ctx, conn, entityID, d); e != nil {
		return e
	}
	// Registry activation and its projections are one transaction.
	rt, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer rt.Rollback()
	var lockedTenant string
	if e = rt.QueryRowContext(ctx, "SELECT id FROM tenants WHERE id=? FOR UPDATE", tenant).Scan(&lockedTenant); e != nil {
		return e
	}
	if v["deployment_id"] != nil {
		job, err := row(ctx, rt, "SELECT * FROM release_deployments WHERE id=? AND state='publishing'", v["deployment_id"])
		if err != nil {
			return err
		}
		if _, err = s.deploymentPrincipal(ctx, rt, job); err != nil {
			return err
		}
		// Storage is ready, but live roles, ACLs and App policies must switch
		// together. Keep the old registry version and the local gate closed.
		result, err := rt.ExecContext(ctx, "UPDATE entity_versions SET state='prepared',error_message=NULL WHERE id=? AND attempt_id=? AND state='publishing'", version, attempt)
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return conflict("Publication attempt superseded.")
		}
		return rt.Commit()
	}
	if e = s.activateEntityVersion(ctx, rt, v, d); e != nil {
		return e
	}
	if e = rt.Commit(); e != nil {
		return e
	}
	if e = openPublicationGate(ctx, conn, v, plan.FieldsHash); e != nil {
		return e
	}
	done, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer done.Rollback()
	if _, e = done.ExecContext(ctx, "UPDATE entity_versions SET state='published',finished_at=UTC_TIMESTAMP(6),error_message=NULL WHERE id=? AND attempt_id=?", version, attempt); e != nil {
		return e
	}
	if _, e = done.ExecContext(ctx, "UPDATE entity_definitions SET pending_version_id=NULL,revision=revision+1 WHERE id=? AND active_version_id=? AND pending_version_id=?", entityID, version, version); e != nil {
		return e
	}
	return done.Commit()
}

func openPublicationGate(ctx context.Context, q Queryer, v Row, layoutHash string) error {
	result, err := q.ExecContext(ctx, "UPDATE _lastapp_entity_state SET version_id=?,layout_hash=?,gate_open=1 WHERE entity_id=? AND attempt_id=?", v["id"], layoutHash, v["entity_id"], v["attempt_id"])
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		// A deployment can restart after opening storage but before its registry
		// transaction commits. MariaDB reports zero changed rows on that retry.
		var matched int
		if err = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM _lastapp_entity_state WHERE entity_id=? AND attempt_id=? AND version_id=? AND layout_hash=? AND gate_open=1", v["entity_id"], v["attempt_id"], v["id"], layoutHash).Scan(&matched); err != nil {
			return err
		}
		if matched != 1 {
			return conflict("Storage publication fence changed.")
		}
	}
	return nil
}

func (s *Server) ensureSupport(ctx context.Context, q Queryer, marker string) error {
	names := []string{"_lastapp_entity_state", "_lastapp_record_acl", "_lastapp_data_audit"}
	for i, name := range names {
		var c string
		e := q.QueryRowContext(ctx, "SELECT TABLE_COMMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?", name).Scan(&c)
		if errors.Is(e, sql.ErrNoRows) {
			if _, e = q.ExecContext(ctx, data.SupportDDL[i]+" COMMENT='"+marker+"'"); e != nil {
				return e
			}
		} else if e != nil {
			return e
		} else if c != marker {
			return ownershipConflict()
		}
	}
	return nil
}

func (s *Server) projectPermissions(ctx context.Context, tx *sql.Tx, v Row, d entity.Definition) error {
	previous, e := rows(ctx, tx, "SELECT id,state,security_hash,security_revision FROM permissions WHERE entity_id=?", v["entity_id"])
	if e != nil {
		return e
	}
	byID := map[string]Row{}
	for _, p := range previous {
		byID[str(p["id"])] = p
	}
	_, e = tx.ExecContext(ctx, "DELETE rp FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id JOIN roles r ON r.id=rp.role_id AND r.scope_kind='space' WHERE p.entity_id=?", v["entity_id"])
	if e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE permissions SET state='disabled' WHERE entity_id=?", v["entity_id"]); e != nil {
		return e
	}
	for _, p := range d.Permissions {
		pid := entity.PermissionID(str(v["entity_id"]), p.Key)
		security := p
		security.Label = ""
		security.Roles = slices.Clone(security.Roles)
		slices.Sort(security.Roles)
		security.Fields = slices.Clone(security.Fields)
		slices.Sort(security.Fields)
		security.Delegable = slices.Clone(security.Delegable)
		slices.Sort(security.Delegable)
		securityHash := entity.Hash([]any{security, d.Owner, d.Team, d.Sharing})
		revision := int64(1)
		if old := byID[pid]; old != nil {
			revision = integer(old["security_revision"])
			if old["state"] != "active" || str(old["security_hash"]) != securityHash {
				revision++
			}
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO permissions(id,code,domain,description,owner_key,tenant_id,space_id,entity_id,version_id,security_hash,security_revision) VALUES(?,?,'data',?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE security_revision=VALUES(security_revision),security_hash=VALUES(security_hash),description=VALUES(description),version_id=VALUES(version_id),state='active'", pid, p.Key, p.Label, "e:"+str(v["entity_id"]), v["tenant_id"], v["space_id"], v["entity_id"], v["id"], securityHash, revision)
		if e != nil {
			return e
		}
		for _, role := range p.Roles {
			if _, e = tx.ExecContext(ctx, "INSERT INTO role_permissions(role_id,permission_id,domain) VALUES(?,?,'data')", role, pid); e != nil {
				return e
			}
		}
	}
	return nil
}

func verifyEntityColumns(ctx context.Context, q Queryer, id string, d entity.Definition) error {
	for _, f := range d.Fields {
		var actual, collation, nullable string
		if e := q.QueryRowContext(ctx, "SELECT COLUMN_TYPE,COALESCE(COLLATION_NAME,''),IS_NULLABLE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?", strings.Trim(data.Table(id), "`"), "f_"+f.Key).Scan(&actual, &collation, &nullable); e != nil {
			return e
		}
		want := strings.Split(data.Type(f), " ")[0]
		normalize := func(s string) string {
			return regexp.MustCompile(`(?i)^(bigint|tinyint)\([0-9]+\)$`).ReplaceAllString(strings.ToLower(s), "$1")
		}
		if want == "BOOLEAN" {
			want = "tinyint"
		}
		if f.Type == "json" {
			want = "longtext"
		}
		if normalize(actual) != normalize(want) || ((nullable == "NO") != f.Required) {
			return conflict("Generated storage differs from the proposed field type; publish a compatible definition.")
		}
		if f.Type == "text" && collation != "utf8mb4_nopad_bin" {
			return ownershipConflict()
		}
	}
	return nil
}

func (s *Server) restorePublicationBase(ctx context.Context, v Row) {
	if v["base_version_id"] == nil {
		return
	}
	rowDef, e := row(ctx, s.DB, "SELECT definition_json FROM entity_versions WHERE id=?", v["base_version_id"])
	if e != nil {
		return
	}
	d, e := defOf(rowDef["definition_json"])
	if e != nil {
		return
	}
	db, e := s.businessDB(ctx, str(v["database_resource_id"]), true)
	if e != nil {
		return
	}
	defer db.Close()
	conn, e := db.Conn(ctx)
	if e != nil {
		return
	}
	defer conn.Close()
	lock := "lastapp:entity:" + str(v["entity_id"])
	var got int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", lock).Scan(&got); e != nil || got != 1 {
		return
	}
	defer releasePinnedLock(conn, lock)
	if verifyEntityColumns(ctx, conn, str(v["entity_id"]), d) != nil {
		return
	}
	var current string
	if s.DB.QueryRowContext(ctx, "SELECT COALESCE(active_version_id,'') FROM entity_definitions WHERE id=?", v["entity_id"]).Scan(&current) != nil || current != str(v["base_version_id"]) {
		return
	}
	if _, e = conn.ExecContext(ctx, "UPDATE _lastapp_entity_state SET gate_open=1,version_id=? WHERE entity_id=? AND attempt_id=?", v["base_version_id"], v["entity_id"], v["attempt_id"]); e != nil {
		// Best effort: if this fails the gate stays closed, which fails closed.
		s.Log.Warn("restore publication base failed", append([]any{"entity_id", str(v["entity_id"])}, diagnostic(e)...)...)
	}
}

// activateEntityVersion changes only registry authority. Deployment workers call
// it for every prepared entity in the same transaction as roles and App policies.
func (s *Server) activateEntityVersion(ctx context.Context, tx *sql.Tx, v Row, d entity.Definition) error {
	entityID, version, attempt := str(v["entity_id"]), str(v["id"]), str(v["attempt_id"])
	tenant, space := str(v["tenant_id"]), str(v["space_id"])
	current, e := row(ctx, tx, "SELECT e.*,v.attempt_id FROM entity_definitions e JOIN entity_versions v ON v.id=? WHERE e.id=? FOR UPDATE", version, entityID)
	if e != nil {
		return e
	}
	if str(current["attempt_id"]) != attempt {
		return conflict("Publication attempt superseded.")
	}
	if str(current["active_version_id"]) != version {
		if str(current["active_version_id"]) != str(v["base_version_id"]) || str(current["pending_version_id"]) != version {
			return conflict("Publication base changed; review again.")
		}
		if e = s.authorize(ctx, tx, Principal{ID: str(v["created_by"])}, "entity.publish", Resource{Tenant: tenant, Space: space}, true); e != nil {
			return e
		}
		candidates, err := s.publicationCandidates(ctx, tx, v)
		if err != nil {
			return err
		}
		if e = s.validateEntityRefsCandidates(ctx, tx, d, Row{"tenant_id": tenant, "space_id": space, "id": entityID}, candidates); e != nil {
			return e
		}
		if e = s.projectPermissions(ctx, tx, v, d); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE entity_definitions SET active_version_id=?,revision=revision+1 WHERE id=?", version, entityID); e != nil {
			return e
		}
		if e = audit(ctx, tx, tenant, str(v["created_by"]), "entity.published", "entity", entityID, newID(), Row{"version": version, "hash": entity.Hash(d)}); e != nil {
			return e
		}
	}
	return nil
}
