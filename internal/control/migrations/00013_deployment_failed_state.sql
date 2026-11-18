-- +goose Up
-- The step a failed deployment had reached, so its status can name it.
ALTER TABLE release_deployments ADD COLUMN failed_state VARCHAR(16) NULL AFTER state,
 ADD CONSTRAINT ck_deployment_failed_state CHECK(failed_state IS NULL OR (state='failed' AND failed_state IN ('queued','preparing','publishing')));
-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
