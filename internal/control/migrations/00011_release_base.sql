-- +goose Up
ALTER TABLE app_releases ADD COLUMN base_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '';
UPDATE app_releases r JOIN development_workspaces w ON w.id=r.workspace_id SET r.base_hash=w.base_hash;
-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
