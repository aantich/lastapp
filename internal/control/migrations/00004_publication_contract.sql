-- +goose Up
ALTER TABLE entity_versions ADD COLUMN compiler_version INT UNSIGNED NOT NULL DEFAULT 1,
 ADD COLUMN storage_plan_json JSON NULL,
 ADD COLUMN storage_plan_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL;
ALTER TABLE apps ADD COLUMN approved_origin VARCHAR(500) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN origin_host VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD UNIQUE KEY uq_app_origin_host(origin_host),
 ADD KEY ix_app_origin(approved_origin);
-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
