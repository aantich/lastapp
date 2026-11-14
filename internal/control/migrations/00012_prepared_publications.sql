-- +goose Up
ALTER TABLE entity_versions DROP CONSTRAINT ck_version_state,
 ADD CONSTRAINT ck_version_state CHECK(state IN ('queued','publishing','prepared','published','failed'));
-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
