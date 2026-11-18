-- +goose Up
-- Each entity numbers its versions 1, 2, ... in creation order. Every created
-- version gets one, whatever its state; a failed version keeps its number, so the
-- published sequence can have gaps. Versions are never deleted, so a number is
-- never reused. Existing versions are numbered by created_at (then id).
ALTER TABLE entity_versions ADD COLUMN version_number INT UNSIGNED NULL AFTER entity_id;
UPDATE entity_versions v JOIN (SELECT id,ROW_NUMBER() OVER (PARTITION BY entity_id ORDER BY created_at,id) AS n FROM entity_versions) o ON o.id=v.id SET v.version_number=o.n;
ALTER TABLE entity_versions MODIFY version_number INT UNSIGNED NOT NULL,
 ADD UNIQUE KEY uq_version_number(entity_id,version_number);
-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
