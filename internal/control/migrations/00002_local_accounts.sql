-- +goose Up
CREATE TABLE local_accounts (
    user_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    login_email VARCHAR(320) COLLATE utf8mb4_nopad_bin NOT NULL,
    password_hash BINARY(32) NOT NULL,
    password_salt BINARY(16) NOT NULL,
    password_iterations INT UNSIGNED NOT NULL,
    must_change_password TINYINT UNSIGNED NOT NULL DEFAULT 1,
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (user_id),
    UNIQUE KEY uq_local_login_email (login_email),
    CONSTRAINT fk_local_account_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT ck_local_password_iterations CHECK (password_iterations=600000),
    CONSTRAINT ck_local_password_change CHECK (must_change_password IN (0,1)),
    CONSTRAINT ck_local_revision CHECK (revision>0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE sessions ADD COLUMN auth_method VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'oidc',
    ADD CONSTRAINT ck_session_auth_method CHECK (auth_method IN ('oidc','password'));

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Automatic destructive rollback is disabled';
