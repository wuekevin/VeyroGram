-- Phone/SMS identity, devices, rotating refresh tokens and login risk events (Story 1.12; ARCHITECTURE AD-9, AD-16).
-- Phone numbers are stored only as a keyed blind index (HMAC-SHA256) plus the last four digits for display.
CREATE TABLE user (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 1,
  uid          CHAR(32)        NOT NULL,
  phone_hash   CHAR(64)        NOT NULL,
  phone_last4  CHAR(4)         NOT NULL,
  status       ENUM('active','disabled') NOT NULL DEFAULT 'active',
  created_at   DATETIME(3)     NOT NULL,
  UNIQUE KEY uq_user_uid (uid),
  UNIQUE KEY uq_user_phone (tenant_id, phone_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE device (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 1,
  user_id      BIGINT UNSIGNED NOT NULL,
  device_key   VARCHAR(64)     NOT NULL,
  class        ENUM('app','web','pc') NOT NULL,
  name         VARCHAR(100)    NOT NULL DEFAULT '',
  created_at   DATETIME(3)     NOT NULL,
  last_seen_at DATETIME(3)     NOT NULL,
  revoked_at   DATETIME(3)     NULL,
  UNIQUE KEY uq_device_user_key (user_id, device_key),
  KEY idx_device_user (user_id, revoked_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Refresh tokens are opaque; only SHA-256 of the token is stored. A rotated token that is presented again is a reuse signal and revokes the device.
CREATE TABLE refresh_token (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 1,
  user_id      BIGINT UNSIGNED NOT NULL,
  device_id    BIGINT UNSIGNED NOT NULL,
  token_hash   CHAR(64)        NOT NULL,
  status       ENUM('active','rotated','revoked') NOT NULL DEFAULT 'active',
  expires_at   DATETIME(3)     NOT NULL,
  created_at   DATETIME(3)     NOT NULL,
  UNIQUE KEY uq_refresh_hash (token_hash),
  KEY idx_refresh_device (device_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- One-time SMS codes: only an HMAC of the code is stored; a new request supersedes older codes.
CREATE TABLE sms_code (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 1,
  phone_hash   CHAR(64)        NOT NULL,
  code_hash    CHAR(64)        NOT NULL,
  attempts     TINYINT UNSIGNED NOT NULL DEFAULT 0,
  expires_at   DATETIME(3)     NOT NULL,
  consumed_at  DATETIME(3)     NULL,
  created_at   DATETIME(3)     NOT NULL,
  KEY idx_sms_phone (phone_hash, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Append-only risk/audit trail for authentication; also the counters for rate limits and lockouts.
CREATE TABLE login_event (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 1,
  kind         VARCHAR(16)     NOT NULL,
  result       VARCHAR(24)     NOT NULL,
  phone_hash   CHAR(64)        NOT NULL DEFAULT '',
  ip           VARCHAR(45)     NOT NULL DEFAULT '',
  device_key   VARCHAR(64)     NOT NULL DEFAULT '',
  user_id      BIGINT UNSIGNED NULL,
  trace_id     VARCHAR(64)     NOT NULL DEFAULT '',
  created_at   DATETIME(3)     NOT NULL,
  KEY idx_le_phone (kind, result, phone_hash, created_at),
  KEY idx_le_ip (kind, result, ip, created_at),
  KEY idx_le_device (kind, result, device_key, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
