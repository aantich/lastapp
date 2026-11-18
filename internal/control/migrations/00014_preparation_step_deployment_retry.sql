-- +goose Up
-- The step a preparing workspace has reached (delivery.WorkspaceSteps); a
-- failed workspace keeps the step it failed at, a ready one has none.
ALTER TABLE development_workspaces ADD COLUMN preparation_step VARCHAR(16) NULL AFTER state,
 ADD CONSTRAINT ck_workspace_preparation_step CHECK(preparation_step IS NULL OR preparation_step IN ('storage','tables','publishing'));
UPDATE development_workspaces SET preparation_step='storage' WHERE state IN ('preparing','failed');
-- Retrying a failed deployment re-queues the same record: attempts counts its
-- runs, queued_at orders the Tenant's queue (a retry joins its end),
-- retried_by is the person whose live authority the latest attempt runs under,
-- and failed_base_hash is the production Space configuration left when it
-- failed, so a retry is refused once anything else has changed it.
ALTER TABLE release_deployments ADD COLUMN attempts INT UNSIGNED NOT NULL DEFAULT 1 AFTER failed_state,
 ADD COLUMN failed_base_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER attempts,
 ADD COLUMN retried_by CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER created_by,
 ADD COLUMN queued_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) AFTER created_at,
 ADD KEY ix_deployment_queue(state,queued_at),
 ADD CONSTRAINT fk_deployment_retried_by FOREIGN KEY(retried_by) REFERENCES users(id),
 ADD CONSTRAINT ck_deployment_attempts CHECK(attempts>0 AND (failed_base_hash IS NULL OR state='failed'));
UPDATE release_deployments SET queued_at=created_at;
-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
