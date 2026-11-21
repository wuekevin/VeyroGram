-- Per-user conversation state that must look the same on every device (Story 1.16): drafts and per-conversation settings. Both carry a per-user
-- monotonically increasing `version` (from sync_clock) so a device can ask "what changed since version N" and get exactly the changes, in order.
-- Rows are keyed by uid, not by a foreign key into the identity module's tables (AD-1: no cross-module table access).
-- A cleared draft stays as a tombstone (deleted=1, content empty) so other devices learn about the deletion; a retention job can purge old tombstones.
CREATE TABLE sync_clock (
  tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 1,
  user_uid  VARCHAR(40)     NOT NULL,
  version   BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (tenant_id, user_uid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE draft (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 1,
  user_uid     VARCHAR(40)     NOT NULL,
  channel_id   VARCHAR(100)    NOT NULL,
  channel_type TINYINT UNSIGNED NOT NULL,
  content      TEXT            NOT NULL,
  deleted      TINYINT(1)      NOT NULL DEFAULT 0,
  version      BIGINT UNSIGNED NOT NULL,
  updated_at   DATETIME(3)     NOT NULL,
  UNIQUE KEY uq_draft (tenant_id, user_uid, channel_id, channel_type),
  KEY idx_draft_version (tenant_id, user_uid, version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE conversation_setting (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 1,
  user_uid     VARCHAR(40)     NOT NULL,
  channel_id   VARCHAR(100)    NOT NULL,
  channel_type TINYINT UNSIGNED NOT NULL,
  pinned       TINYINT(1)      NOT NULL DEFAULT 0,
  muted        TINYINT(1)      NOT NULL DEFAULT 0,
  version      BIGINT UNSIGNED NOT NULL,
  updated_at   DATETIME(3)     NOT NULL,
  UNIQUE KEY uq_convsetting (tenant_id, user_uid, channel_id, channel_type),
  KEY idx_convsetting_version (tenant_id, user_uid, version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
