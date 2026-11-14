-- +goose Up
-- Earlier development grants have no audience and must be re-approved. Never
-- infer an OAuth resource from a client-controlled request or backfill a host.
ALTER TABLE agent_grants ADD COLUMN audience_uri VARCHAR(500) COLLATE utf8mb4_bin NOT NULL DEFAULT '';
ALTER TABLE agent_oauth_requests ADD COLUMN resource_uri VARCHAR(500) COLLATE utf8mb4_bin NOT NULL DEFAULT '';
UPDATE agent_grants SET state='revoked',revision=revision+1 WHERE audience_uri='' AND state='active';

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
