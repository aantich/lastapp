-- +goose NO TRANSACTION
-- +goose Up
-- OIDC identifiers are opaque strings: even trailing spaces are significant.
-- ALTER preserves every existing user ID and its grants/sessions.
ALTER TABLE users
  MODIFY oidc_issuer VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_nopad_bin NOT NULL,
  MODIFY oidc_subject VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_nopad_bin NOT NULL;

-- Preserve evidence before coalescing equivalent historical grants. Only the
-- lexically smallest ID is retained; effective authorization does not change.
INSERT INTO audit_events (tenant_id,actor_user_id,action,resource_type,resource_id,request_id,details_json)
SELECT d.tenant_id,NULL,'grant.coalesced','access_grant',d.id,'migration-00002',
 JSON_OBJECT('before',JSON_OBJECT('id',d.id,'user_id',d.user_id,'group_id',d.group_id,'space_id',d.space_id,'app_id',d.app_id,'role_id',d.role_id,'permission_code',d.permission_code),'after',NULL)
FROM access_grants d WHERE EXISTS (
 SELECT 1 FROM access_grants g WHERE g.id<d.id AND g.kind=d.kind
 AND g.tenant_id <=> d.tenant_id AND g.user_id <=> d.user_id AND g.group_id <=> d.group_id
 AND g.space_id <=> d.space_id AND g.app_id <=> d.app_id AND g.role_id <=> d.role_id AND g.permission_code <=> d.permission_code);
DELETE d FROM access_grants d JOIN access_grants g ON g.id<d.id AND g.kind=d.kind
 AND g.tenant_id <=> d.tenant_id AND g.user_id <=> d.user_id AND g.group_id <=> d.group_id
 AND g.space_id <=> d.space_id AND g.app_id <=> d.app_id AND g.role_id <=> d.role_id AND g.permission_code <=> d.permission_code;
ALTER TABLE access_grants ADD COLUMN equivalence_key VARCHAR(400) CHARACTER SET ascii COLLATE ascii_bin
 GENERATED ALWAYS AS (CONCAT_WS('|',kind,COALESCE(RTRIM(tenant_id),''),COALESCE(RTRIM(user_id),''),COALESCE(RTRIM(group_id),''),COALESCE(RTRIM(space_id),''),COALESCE(RTRIM(app_id),''),COALESCE(RTRIM(role_id),''),COALESCE(permission_code,''))) PERSISTENT,
 ADD UNIQUE KEY uq_grant_equivalence (equivalence_key);

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Security migrations cannot be rolled back';
