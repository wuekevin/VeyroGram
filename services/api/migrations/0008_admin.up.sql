-- Admin plane (Story 1.19; ARCHITECTURE AD-3, AD-9, AD-15, AD-24). Admin identities are separate from end users: own table, own sessions, mandatory TOTP.
CREATE TABLE role (
  id   TINYINT UNSIGNED NOT NULL PRIMARY KEY,
  name VARCHAR(32)      NOT NULL,
  UNIQUE KEY uq_role_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE permission (
  role_id    TINYINT UNSIGNED NOT NULL,
  permission VARCHAR(48)      NOT NULL,
  PRIMARY KEY (role_id, permission)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO role (id, name) VALUES (1, 'owner'), (2, 'auditor');
INSERT INTO permission (role_id, permission) VALUES
  (1, 'self'), (1, 'audit:read'), (1, 'audit:verify'), (1, 'admin:read'), (1, 'admin:write'),
  (2, 'self'), (2, 'audit:read'), (2, 'audit:verify');

CREATE TABLE admin_user (
  id               BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id        BIGINT UNSIGNED  NOT NULL DEFAULT 1,
  username         VARCHAR(64)      NOT NULL,
  password_hash    VARCHAR(255)     NOT NULL,
  role_id          TINYINT UNSIGNED NOT NULL,
  status           ENUM('active','disabled') NOT NULL DEFAULT 'active',
  totp_secret_enc  VARBINARY(128)   NOT NULL,
  totp_enrolled_at DATETIME(3)      NULL,
  totp_last_step   BIGINT UNSIGNED  NOT NULL DEFAULT 0,
  failed_attempts  INT UNSIGNED     NOT NULL DEFAULT 0,
  locked_until     DATETIME(3)      NULL,
  created_by       BIGINT UNSIGNED  NULL,
  created_at       DATETIME(3)      NOT NULL,
  UNIQUE KEY uq_admin_username (tenant_id, username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- One row, written in the same transaction as the first admin. This (not a lock file) is what refuses a second initialisation.
CREATE TABLE system_state (
  k          VARCHAR(32)  NOT NULL PRIMARY KEY,
  created_at DATETIME(3)  NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Short-lived single-use tokens between password and second factor (kind: mfa | enroll).
CREATE TABLE admin_challenge (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  admin_id   BIGINT UNSIGNED NOT NULL,
  kind       VARCHAR(8)      NOT NULL,
  token_hash CHAR(64)        NOT NULL,
  expires_at DATETIME(3)     NOT NULL,
  used_at    DATETIME(3)     NULL,
  UNIQUE KEY uq_challenge_hash (token_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Opaque, revocable sessions (SHA-256 of the token is stored). Only issued after the second factor succeeded.
CREATE TABLE admin_session (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  admin_id     BIGINT UNSIGNED NOT NULL,
  token_hash   CHAR(64)        NOT NULL,
  ip           VARCHAR(45)     NOT NULL DEFAULT '',
  created_at   DATETIME(3)     NOT NULL,
  last_seen_at DATETIME(3)     NOT NULL,
  expires_at   DATETIME(3)     NOT NULL,
  revoked_at   DATETIME(3)     NULL,
  UNIQUE KEY uq_session_hash (token_hash),
  KEY idx_session_admin (admin_id, revoked_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
