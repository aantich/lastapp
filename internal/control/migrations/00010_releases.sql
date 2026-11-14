-- +goose Up
CREATE TABLE app_builds (
 id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 app_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 content_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 files_json JSON NOT NULL,
 entrypoint VARCHAR(240) NOT NULL,
 size_bytes BIGINT UNSIGNED NOT NULL,
 created_by CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_build_app(app_id,id),
 FOREIGN KEY(app_id) REFERENCES apps(id), FOREIGN KEY(created_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE artifact_uploads (
 token_hash BINARY(32) PRIMARY KEY,
 tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 app_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 user_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 agent_grant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 agent_grant_revision BIGINT UNSIGNED NULL,
 build_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 expires_at DATETIME(6) NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 FOREIGN KEY(app_id) REFERENCES apps(id), FOREIGN KEY(user_id) REFERENCES users(id),
 FOREIGN KEY(agent_grant_id) REFERENCES agent_grants(id), FOREIGN KEY(build_id) REFERENCES app_builds(id),
 KEY ix_upload_expiry(expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE app_releases (
 id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 app_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 build_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 workspace_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 display_name VARCHAR(160) NOT NULL,
 manifest_json JSON NOT NULL,
 snapshot_json JSON NOT NULL,
 snapshot_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 created_by CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 UNIQUE KEY uq_release_app(app_id,id),
 FOREIGN KEY(app_id,build_id) REFERENCES app_builds(app_id,id),
 FOREIGN KEY(workspace_id) REFERENCES development_workspaces(id), FOREIGN KEY(created_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
ALTER TABLE apps ADD COLUMN active_release_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD CONSTRAINT fk_app_release FOREIGN KEY(active_release_id) REFERENCES app_releases(id);
CREATE TABLE release_deployments (
 id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
 tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 space_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 release_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 target_app_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 created_by CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 agent_grant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 agent_grant_revision BIGINT UNSIGNED NULL,
 state VARCHAR(16) NOT NULL DEFAULT 'queued',
 error_message VARCHAR(500) NULL,
 plan_json JSON NOT NULL,
 revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
 created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 finished_at DATETIME(6) NULL,
 KEY ix_deployment_jobs(state,created_at),
 FOREIGN KEY(tenant_id,space_id) REFERENCES spaces(tenant_id,id),
 FOREIGN KEY(release_id) REFERENCES app_releases(id), FOREIGN KEY(target_app_id) REFERENCES apps(id),
 FOREIGN KEY(created_by) REFERENCES users(id), FOREIGN KEY(agent_grant_id) REFERENCES agent_grants(id),
 CHECK(state IN ('queued','preparing','publishing','active','failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
ALTER TABLE entity_versions ADD COLUMN deployment_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD CONSTRAINT fk_publication_deployment FOREIGN KEY(deployment_id) REFERENCES release_deployments(id);
-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
