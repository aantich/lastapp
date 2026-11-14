-- +goose Up
-- Bounded cleanup must preserve clients with an in-progress consent/code flow.
CREATE INDEX ix_agent_request_client ON agent_oauth_requests(client_id,consumed,expires_at);

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
