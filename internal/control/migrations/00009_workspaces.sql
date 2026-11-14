-- +goose Up
CREATE TABLE development_workspaces (
 id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 production_space_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 backing_space_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL UNIQUE,
 owner_user_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 owner_agent_grant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 display_name VARCHAR(160) NOT NULL,
 base_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 base_json JSON NOT NULL,
 state VARCHAR(16) NOT NULL DEFAULT 'preparing',
 error_message VARCHAR(500) NULL,
 revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 KEY ix_workspace_owner(owner_user_id,state),
 FOREIGN KEY(tenant_id,production_space_id) REFERENCES spaces(tenant_id,id),
 FOREIGN KEY(tenant_id,backing_space_id) REFERENCES spaces(tenant_id,id),
 FOREIGN KEY(owner_user_id) REFERENCES users(id),
 FOREIGN KEY(owner_agent_grant_id) REFERENCES agent_grants(id),
 CHECK(state IN ('preparing','ready','failed','archived') AND revision>0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
ALTER TABLE spaces ADD COLUMN workspace_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD CONSTRAINT fk_space_workspace FOREIGN KEY(workspace_id) REFERENCES development_workspaces(id);
CREATE TABLE workspace_items (
 workspace_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 kind VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 development_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 production_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 PRIMARY KEY(workspace_id,kind,development_id),
 UNIQUE KEY uq_workspace_production(workspace_id,kind,production_id),
 FOREIGN KEY(workspace_id) REFERENCES development_workspaces(id),
 CHECK(kind IN ('entity','role','app','microapp'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
ALTER TABLE sessions DROP CONSTRAINT ck_sessions_purpose,
 ADD COLUMN agent_grant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN agent_grant_revision BIGINT UNSIGNED NULL,
 ADD CONSTRAINT fk_session_agent FOREIGN KEY(agent_grant_id) REFERENCES agent_grants(id),
 ADD CONSTRAINT ck_sessions_purpose CHECK(
 (purpose='console' AND state='active' AND parent_hash IS NULL AND app_id IS NULL AND agent_grant_id IS NULL) OR
 (purpose IN ('launch','app') AND state IN ('active','consumed') AND app_id IS NOT NULL AND origin IS NOT NULL AND origin_generation IS NOT NULL AND
 ((parent_hash IS NOT NULL AND agent_grant_id IS NULL) OR (parent_hash IS NULL AND agent_grant_id IS NOT NULL AND agent_grant_revision IS NOT NULL))));
-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
