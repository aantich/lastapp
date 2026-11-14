-- +goose Up
CREATE TABLE agent_grants (
 id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 user_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 client_id VARCHAR(500) COLLATE utf8mb4_bin NOT NULL,
 client_name VARCHAR(160) NOT NULL,
 tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 scope_kind VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 scope_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 permissions_json JSON NOT NULL,
 limits_json JSON NOT NULL,
 state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
 revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 expires_at DATETIME(6) NOT NULL,
 last_used_at DATETIME(6) NULL,
 KEY ix_agent_owner(user_id,state), KEY ix_agent_tenant(tenant_id,state),
 FOREIGN KEY(user_id) REFERENCES users(id), FOREIGN KEY(tenant_id) REFERENCES tenants(id),
 CHECK(scope_kind IN ('platform','tenant','team','space','app')),
 CHECK((scope_kind='platform' AND tenant_id IS NULL AND scope_id IS NULL) OR
       (scope_kind='tenant' AND tenant_id IS NOT NULL AND scope_id IS NULL) OR
       (scope_kind IN ('team','space','app') AND tenant_id IS NOT NULL AND scope_id IS NOT NULL)),
 CHECK(state IN ('active','revoked') AND revision>0 AND expires_at>created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE agent_oauth_requests (
 id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 client_id VARCHAR(500) COLLATE utf8mb4_bin NOT NULL,
 client_name VARCHAR(160) NOT NULL,
 redirect_uri VARCHAR(1500) COLLATE utf8mb4_bin NOT NULL,
 challenge VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 state_value VARCHAR(1024) COLLATE utf8mb4_bin NOT NULL,
 scopes VARCHAR(500) NOT NULL,
 grant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 code_hash BINARY(32) NULL UNIQUE,
 consumed TINYINT UNSIGNED NOT NULL DEFAULT 0,
 expires_at DATETIME(6) NOT NULL,
 FOREIGN KEY(grant_id) REFERENCES agent_grants(id),
 KEY ix_agent_request_expiry(expires_at), CHECK(consumed IN (0,1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE agent_token_sessions (
 id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 grant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 grant_revision BIGINT UNSIGNED NOT NULL,
 family_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 access_hash BINARY(32) NOT NULL UNIQUE,
 refresh_hash BINARY(32) NOT NULL UNIQUE,
 state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
 access_expires_at DATETIME(6) NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 FOREIGN KEY(grant_id) REFERENCES agent_grants(id),
 KEY ix_agent_tokens_grant(grant_id,state), KEY ix_agent_tokens_family(family_id),
 CHECK(state IN ('active','rotated','revoked'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE agent_operation_receipts (
 grant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 request_key CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 input_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 operation VARCHAR(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 result_json JSON NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 PRIMARY KEY(grant_id,request_key), FOREIGN KEY(grant_id) REFERENCES agent_grants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE entity_versions ADD COLUMN agent_grant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD CONSTRAINT fk_entity_agent FOREIGN KEY(agent_grant_id) REFERENCES agent_grants(id);
ALTER TABLE database_resources ADD COLUMN agent_grant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN agent_user_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD CONSTRAINT fk_database_agent FOREIGN KEY(agent_grant_id) REFERENCES agent_grants(id);

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
