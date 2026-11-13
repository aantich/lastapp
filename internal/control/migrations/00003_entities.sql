-- +goose Up
CREATE TABLE entity_definitions (
 id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 space_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 entity_key VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 label VARCHAR(160) NOT NULL,
 state VARCHAR(16) NOT NULL DEFAULT 'active',
 draft_json JSON NOT NULL,
 revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
 active_version_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 pending_version_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_entity_key(space_id,entity_key),
 UNIQUE KEY uq_entity_owner(tenant_id,space_id,id),
 CONSTRAINT fk_entity_space FOREIGN KEY(tenant_id,space_id) REFERENCES spaces(tenant_id,id),
 CONSTRAINT ck_entity_state CHECK(state IN ('active','archived')),
 CONSTRAINT ck_entity_revision CHECK(revision>0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE entity_versions (
 id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 space_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 entity_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 base_version_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 definition_json JSON NOT NULL,
 definition_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 state VARCHAR(16) NOT NULL DEFAULT 'queued',
 attempt_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 error_message VARCHAR(500) NULL,
 created_by CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 finished_at DATETIME(6) NULL,
 KEY ix_publication_work(state,created_at),
 UNIQUE KEY uq_version_owner(entity_id,id),
 CONSTRAINT fk_version_entity FOREIGN KEY(tenant_id,space_id,entity_id) REFERENCES entity_definitions(tenant_id,space_id,id),
 CONSTRAINT fk_version_author FOREIGN KEY(created_by) REFERENCES users(id),
 CONSTRAINT ck_version_state CHECK(state IN ('queued','publishing','published','failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
ALTER TABLE entity_definitions
 ADD CONSTRAINT fk_entity_active FOREIGN KEY(id,active_version_id) REFERENCES entity_versions(entity_id,id),
 ADD CONSTRAINT fk_entity_pending FOREIGN KEY(id,pending_version_id) REFERENCES entity_versions(entity_id,id);
ALTER TABLE permissions DROP CONSTRAINT ck_permissions_domain,
 DROP INDEX uq_permissions_code,
 ADD COLUMN owner_key VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'builtin',
 ADD COLUMN tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN space_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN entity_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN version_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN security_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN security_revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
 ADD COLUMN state VARCHAR(16) NOT NULL DEFAULT 'active',
 ADD UNIQUE KEY uq_permission_owner(owner_key,code),
 ADD CONSTRAINT fk_permission_entity FOREIGN KEY(tenant_id,space_id,entity_id) REFERENCES entity_definitions(tenant_id,space_id,id),
 ADD CONSTRAINT fk_permission_version FOREIGN KEY(entity_id,version_id) REFERENCES entity_versions(entity_id,id),
 ADD CONSTRAINT ck_permissions_domain CHECK((domain='control' AND owner_key='builtin' AND entity_id IS NULL AND tenant_id IS NULL AND space_id IS NULL AND version_id IS NULL) OR (domain='data' AND entity_id IS NOT NULL AND tenant_id IS NOT NULL AND space_id IS NOT NULL AND version_id IS NOT NULL AND owner_key=CONCAT('e:',RTRIM(entity_id)))),
 ADD CONSTRAINT ck_permission_state CHECK(state IN ('active','disabled'));
ALTER TABLE apps ADD COLUMN data_access_json JSON NOT NULL DEFAULT '{"origin":"","inherit":false,"ceiling":[],"restricted":false,"restrictions":[]}',
 ADD COLUMN origin_generation BIGINT UNSIGNED NOT NULL DEFAULT 1;
ALTER TABLE database_resources ADD COLUMN schema_credentials_enc BLOB NULL,
 ADD COLUMN credential_generation BIGINT UNSIGNED NOT NULL DEFAULT 1;
ALTER TABLE sessions DROP CONSTRAINT ck_sessions_purpose,
 ADD COLUMN parent_hash BINARY(32) NULL,
 ADD COLUMN tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN space_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN app_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN origin VARCHAR(500) NULL,
 ADD COLUMN origin_generation BIGINT UNSIGNED NULL,
 ADD COLUMN challenge VARCHAR(128) NULL,
 ADD COLUMN nonce VARCHAR(128) NULL,
 ADD KEY ix_session_parent(parent_hash),
 ADD CONSTRAINT fk_session_app FOREIGN KEY(tenant_id,space_id,app_id) REFERENCES apps(tenant_id,space_id,id),
 ADD CONSTRAINT ck_sessions_purpose CHECK((purpose='console' AND state='active' AND parent_hash IS NULL AND app_id IS NULL) OR (purpose IN ('launch','app') AND state IN ('active','consumed') AND parent_hash IS NOT NULL AND app_id IS NOT NULL AND origin IS NOT NULL AND origin_generation IS NOT NULL));
-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
