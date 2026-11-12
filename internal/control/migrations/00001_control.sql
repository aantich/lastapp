-- +goose NO TRANSACTION
-- +goose Up
-- Lastapp provisioning schema: final phase-one design, MariaDB / InnoDB.
-- Design DDL for an empty database, not an upgrade migration from the earlier draft.
-- Apply to an EMPTY, explicitly selected control-plane database.
-- No databases/accounts are created and no tenant business data is modified.
-- All service connections and migration sessions use UTC.
SET time_zone = '+00:00';

CREATE TABLE users (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    oidc_issuer VARCHAR(255) COLLATE utf8mb4_bin NOT NULL,
    oidc_subject VARCHAR(255) COLLATE utf8mb4_bin NOT NULL,
    display_name VARCHAR(160) NOT NULL,
    email VARCHAR(320) NULL,
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_users_identity (oidc_issuer, oidc_subject),
    CONSTRAINT ck_users_state CHECK (state IN ('active', 'disabled')),
    CONSTRAINT ck_users_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE sessions (
    token_hash BINARY(32) NOT NULL,
    user_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    csrf_hash BINARY(32) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    expires_at DATETIME(6) NOT NULL,
    PRIMARY KEY (token_hash),
    KEY ix_sessions_user (user_id),
    KEY ix_sessions_expiry (expires_at),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT ck_sessions_expiry CHECK (expires_at > created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE plans (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    display_name VARCHAR(160) NOT NULL,
    allow_shared_database TINYINT UNSIGNED NOT NULL,
    allow_dedicated_database TINYINT UNSIGNED NOT NULL,
    allow_managed_database TINYINT UNSIGNED NOT NULL,
    allow_registered_database TINYINT UNSIGNED NOT NULL,
    allow_dedicated_compute TINYINT UNSIGNED NOT NULL DEFAULT 0,
    default_layout VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    max_spaces INT UNSIGNED NULL,
    max_apps INT UNSIGNED NULL,
    max_databases INT UNSIGNED NULL,
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_plans_code (code),
    CONSTRAINT ck_plan_revision CHECK (revision > 0),
    CONSTRAINT ck_plan_flags CHECK (
        allow_shared_database IN (0, 1)
        AND allow_dedicated_database IN (0, 1) AND allow_managed_database IN (0, 1)
        AND allow_registered_database IN (0, 1) AND allow_dedicated_compute IN (0, 1)
    ),
    CONSTRAINT ck_plan_layouts CHECK (allow_shared_database = 1 OR allow_dedicated_database = 1),
    CONSTRAINT ck_plan_methods CHECK (allow_managed_database = 1 OR allow_registered_database = 1),
    CONSTRAINT ck_plan_default CHECK (
        (default_layout = 'shared' AND allow_shared_database = 1)
        OR (default_layout = 'dedicated' AND allow_dedicated_database = 1)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE tenants (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    slug VARCHAR(63) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    display_name VARCHAR(160) NOT NULL,
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
    plan_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    default_layout VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NULL,
    default_shared_database_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    default_shared_scope VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'shared',
    created_by CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_tenants_slug (slug),
    KEY ix_tenants_plan (plan_id),
    CONSTRAINT fk_tenants_plan FOREIGN KEY (plan_id) REFERENCES plans (id),
    CONSTRAINT fk_tenants_creator FOREIGN KEY (created_by) REFERENCES users (id),
    CONSTRAINT ck_tenants_state CHECK (state IN ('active', 'suspended', 'archived')),
    CONSTRAINT ck_tenants_default CHECK (default_layout IS NULL OR default_layout IN ('shared', 'dedicated')),
    CONSTRAINT ck_tenants_shared_scope CHECK (default_shared_scope = 'shared'),
    CONSTRAINT ck_tenants_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE tenant_memberships (
    tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, user_id),
    KEY ix_memberships_user (user_id, state, tenant_id),
    CONSTRAINT fk_memberships_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
    CONSTRAINT fk_memberships_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT ck_memberships_state CHECK (state IN ('active', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE database_targets (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    display_name VARCHAR(160) NOT NULL,
    canonical_host VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    port SMALLINT UNSIGNED NOT NULL DEFAULT 3306,
    tls_mode VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'verify_identity',
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
    managed_enabled TINYINT UNSIGNED NOT NULL DEFAULT 0,
    provisioning_credentials_enc BLOB NULL,
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_targets_endpoint (canonical_host, port),
    CONSTRAINT ck_targets_port CHECK (port > 0),
    CONSTRAINT ck_targets_tls CHECK (tls_mode IN ('verify_identity', 'disabled')),
    CONSTRAINT ck_targets_state CHECK (state IN ('active', 'disabled')),
    CONSTRAINT ck_targets_managed CHECK (
        managed_enabled IN (0, 1) AND (managed_enabled = 0 OR provisioning_credentials_enc IS NOT NULL)
    ),
    CONSTRAINT ck_targets_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE database_resources (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    target_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    database_name VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
    database_name_key VARCHAR(64) COLLATE utf8mb4_bin
        GENERATED ALWAYS AS (LOWER(database_name)) PERSISTENT,
    management_mode VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    layout VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'pending',
    runtime_credentials_enc BLOB NOT NULL,
    ownership_token BINARY(16) NULL,
    attempts INT UNSIGNED NOT NULL DEFAULT 0,
    last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
    last_error_message VARCHAR(500) NULL,
    last_verified_at DATETIME(6) NULL,
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_databases_tenant_scope (tenant_id, id, layout),
    UNIQUE KEY uq_databases_tenant (tenant_id, id),
    UNIQUE KEY uq_databases_physical (target_id, database_name_key),
    KEY ix_databases_inventory (tenant_id, state, id),
    KEY ix_databases_work (state, created_at, id),
    CONSTRAINT fk_databases_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
    CONSTRAINT fk_databases_target FOREIGN KEY (target_id) REFERENCES database_targets (id),
    CONSTRAINT ck_databases_mode CHECK (management_mode IN ('managed', 'registered')),
    CONSTRAINT ck_databases_layout CHECK (layout IN ('shared', 'dedicated')),
    CONSTRAINT ck_databases_state CHECK (state IN ('pending', 'provisioning', 'ready', 'failed', 'retired')),
    CONSTRAINT ck_databases_name CHECK (CHAR_LENGTH(database_name) > 0),
    CONSTRAINT ck_databases_ownership CHECK (
        (management_mode = 'managed' AND ownership_token IS NOT NULL)
        OR (management_mode = 'registered' AND ownership_token IS NULL)
    ),
    CONSTRAINT ck_databases_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE tenants ADD CONSTRAINT fk_tenants_shared_database
    FOREIGN KEY (id, default_shared_database_id, default_shared_scope)
    REFERENCES database_resources (tenant_id, id, layout);

CREATE TABLE spaces (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    slug VARCHAR(63) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    display_name VARCHAR(160) NOT NULL,
    database_resource_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    database_layout VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    dedicated_database_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin
        GENERATED ALWAYS AS (
            CASE WHEN database_layout = 'dedicated' THEN RTRIM(database_resource_id) ELSE NULL END
        ) PERSISTENT,
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_spaces_tenant (tenant_id, id),
    UNIQUE KEY uq_spaces_slug (tenant_id, slug),
    UNIQUE KEY uq_spaces_dedicated_database (dedicated_database_id),
    KEY ix_spaces_database (tenant_id, database_resource_id, database_layout),
    CONSTRAINT fk_spaces_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
    CONSTRAINT fk_spaces_database FOREIGN KEY (tenant_id, database_resource_id, database_layout)
        REFERENCES database_resources (tenant_id, id, layout),
    CONSTRAINT ck_spaces_layout CHECK (database_layout IN ('shared', 'dedicated')),
    CONSTRAINT ck_spaces_state CHECK (state IN ('active', 'suspended', 'archived')),
    CONSTRAINT ck_spaces_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE apps (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    space_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    slug VARCHAR(63) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    display_name VARCHAR(160) NOT NULL,
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
    compute_mode VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'shared',
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_apps_tenant (tenant_id, id),
    UNIQUE KEY uq_apps_slug (space_id, slug),
    KEY ix_apps_inventory (tenant_id, state, id),
    CONSTRAINT fk_apps_space FOREIGN KEY (tenant_id, space_id) REFERENCES spaces (tenant_id, id),
    CONSTRAINT ck_apps_state CHECK (state IN ('active', 'suspended', 'archived')),
    CONSTRAINT ck_apps_compute CHECK (compute_mode IN ('shared', 'dedicated')),
    CONSTRAINT ck_apps_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE microapp_installations (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    app_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    slug VARCHAR(63) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    display_name VARCHAR(160) NOT NULL,
    artifact_ref VARCHAR(1024) COLLATE utf8mb4_bin NULL,
    navigation_order INT NOT NULL DEFAULT 0,
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_microapps_slug (app_id, slug),
    KEY ix_microapps_navigation (tenant_id, app_id, navigation_order, id),
    CONSTRAINT fk_microapps_app FOREIGN KEY (tenant_id, app_id) REFERENCES apps (tenant_id, id),
    CONSTRAINT ck_microapps_state CHECK (state IN ('active', 'suspended', 'archived')),
    CONSTRAINT ck_microapps_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE audit_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    actor_user_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    action VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    resource_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    resource_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    request_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    details_json JSON NOT NULL DEFAULT '{}',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY ix_audit_tenant (tenant_id, id),
    KEY ix_audit_resource (resource_type, resource_id, id),
    CONSTRAINT fk_audit_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
    CONSTRAINT fk_audit_actor FOREIGN KEY (actor_user_id) REFERENCES users (id),
    CONSTRAINT ck_audit_details CHECK (JSON_TYPE(details_json) = 'OBJECT')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Shared authorization catalog. New Tenants add rows, never new auth tables.
CREATE TABLE groups (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    slug VARCHAR(63) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    display_name VARCHAR(160) NOT NULL,
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_groups_tenant (tenant_id, id),
    UNIQUE KEY uq_groups_slug (tenant_id, slug),
    CONSTRAINT fk_groups_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
    CONSTRAINT ck_groups_state CHECK (state IN ('active', 'disabled')),
    CONSTRAINT ck_groups_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE group_memberships (
    tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    group_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    PRIMARY KEY (tenant_id, group_id, user_id),
    KEY ix_group_memberships_user (tenant_id, user_id, group_id),
    CONSTRAINT fk_group_memberships_group FOREIGN KEY (tenant_id, group_id)
        REFERENCES groups (tenant_id, id),
    CONSTRAINT fk_group_memberships_member FOREIGN KEY (tenant_id, user_id)
        REFERENCES tenant_memberships (tenant_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE permissions (
    code VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    kind VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    description VARCHAR(160) NOT NULL,
    PRIMARY KEY (code),
    UNIQUE KEY uq_permissions_kind (code, kind),
    CONSTRAINT ck_permissions_kind CHECK (kind IN ('platform', 'tenant'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE roles (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    -- 'platform' owns shared built-ins; 't:<tenant UUID>' owns custom roles.
    -- Explicit and checked so ordinary composite FKs can enforce grant ownership.
    owner_key VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    kind VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    display_name VARCHAR(160) NOT NULL,
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_roles_code (owner_key, code),
    UNIQUE KEY uq_roles_kind (id, kind),
    UNIQUE KEY uq_roles_owner_kind (owner_key, id, kind),
    CONSTRAINT fk_roles_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
    CONSTRAINT ck_roles_kind CHECK (kind IN ('platform', 'tenant')),
    CONSTRAINT ck_roles_owner CHECK (
        (tenant_id IS NULL AND owner_key = 'platform')
        OR (tenant_id IS NOT NULL AND owner_key = CONCAT('t:', RTRIM(tenant_id)) AND kind = 'tenant')
    ),
    CONSTRAINT ck_roles_state CHECK (state IN ('active', 'disabled')),
    CONSTRAINT ck_roles_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE role_permissions (
    role_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    kind VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    permission_code VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    PRIMARY KEY (role_id, permission_code),
    CONSTRAINT fk_role_permissions_role FOREIGN KEY (role_id, kind) REFERENCES roles (id, kind),
    CONSTRAINT fk_role_permissions_permission FOREIGN KEY (permission_code, kind)
        REFERENCES permissions (code, kind)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- A grant assigns either a role bundle OR a direct permission to one user/group.
-- NULL space/app means the whole Tenant; one child ID narrows the scope.
CREATE TABLE access_grants (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    tenant_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    kind VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    group_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    space_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    app_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    role_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
    role_owner_key VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NULL,
    permission_code VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NULL,
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY ix_grants_user (user_id, tenant_id),
    KEY ix_grants_group (tenant_id, group_id),
    CONSTRAINT fk_grants_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
    CONSTRAINT fk_grants_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT fk_grants_member FOREIGN KEY (tenant_id, user_id)
        REFERENCES tenant_memberships (tenant_id, user_id),
    CONSTRAINT fk_grants_group FOREIGN KEY (tenant_id, group_id) REFERENCES groups (tenant_id, id),
    CONSTRAINT fk_grants_space FOREIGN KEY (tenant_id, space_id) REFERENCES spaces (tenant_id, id),
    CONSTRAINT fk_grants_app FOREIGN KEY (tenant_id, app_id) REFERENCES apps (tenant_id, id),
    CONSTRAINT fk_grants_role FOREIGN KEY (role_owner_key, role_id, kind)
        REFERENCES roles (owner_key, id, kind),
    CONSTRAINT fk_grants_permission FOREIGN KEY (permission_code, kind)
        REFERENCES permissions (code, kind),
    CONSTRAINT ck_grants_subject CHECK (
        (user_id IS NOT NULL AND group_id IS NULL) OR (user_id IS NULL AND group_id IS NOT NULL)
    ),
    CONSTRAINT ck_grants_scope CHECK (
        (kind = 'platform' AND tenant_id IS NULL AND user_id IS NOT NULL
            AND group_id IS NULL AND space_id IS NULL AND app_id IS NULL)
        OR (kind = 'tenant' AND tenant_id IS NOT NULL AND (space_id IS NULL OR app_id IS NULL))
    ),
    CONSTRAINT ck_grants_target CHECK (
        (role_id IS NOT NULL AND permission_code IS NULL AND role_owner_key IS NOT NULL
            AND (role_owner_key = 'platform'
                OR (tenant_id IS NOT NULL AND role_owner_key = CONCAT('t:', RTRIM(tenant_id)))))
        OR (role_id IS NULL AND role_owner_key IS NULL AND permission_code IS NOT NULL)
    ),
    CONSTRAINT ck_grants_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Protected catalog and role definitions: runtime Tenant APIs cannot edit these.
INSERT INTO permissions (code, kind, description) VALUES
    ('platform.admin', 'platform', 'Administer the platform control plane'),
    ('tenant.read', 'tenant', 'Read Tenant metadata'),
    ('tenant.manage', 'tenant', 'Manage Tenant metadata'),
    ('membership.read', 'tenant', 'Read Tenant memberships'),
    ('membership.manage', 'tenant', 'Manage Tenant memberships'),
    ('authorization.read', 'tenant', 'Read groups, roles and grants'),
    ('authorization.manage', 'tenant', 'Manage groups, custom roles and grants'),
    ('database.read', 'tenant', 'Read database resource metadata'),
    ('database.manage', 'tenant', 'Request and manage database resources'),
    ('space.read', 'tenant', 'Read Space metadata'),
    ('space.manage', 'tenant', 'Create and manage Spaces'),
    ('app.read', 'tenant', 'Read App metadata'),
    ('app.manage', 'tenant', 'Create and manage Apps'),
    ('microapp.read', 'tenant', 'Read micro-app installations'),
    ('microapp.manage', 'tenant', 'Create and manage micro-app installations'),
    ('audit.read', 'tenant', 'Read Tenant audit history');

INSERT INTO roles (id, tenant_id, owner_key, kind, code, display_name) VALUES
    ('00000000-0000-4000-8000-000000000001', NULL, 'platform', 'platform', 'superadmin', 'Platform superadmin'),
    ('00000000-0000-4000-8000-000000000002', NULL, 'platform', 'tenant', 'tenant_admin', 'Tenant administrator');

INSERT INTO role_permissions (role_id, kind, permission_code)
    SELECT '00000000-0000-4000-8000-000000000001', kind, code FROM permissions WHERE kind = 'platform';
INSERT INTO role_permissions (role_id, kind, permission_code)
    SELECT '00000000-0000-4000-8000-000000000002', kind, code FROM permissions WHERE kind = 'tenant';

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Restore from backup; automatic destructive rollback is disabled';
