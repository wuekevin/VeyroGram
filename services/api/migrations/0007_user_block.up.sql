-- Who has blocked whom (Story 1.15 follow-up, ARCHITECTURE AD-21). The send-policy gate reads this table, authoritative in MySQL, before every one-to-one
-- message. Keyed by uid (no cross-module foreign keys, AD-1). The blocker can still write to the blocked account; the blocked account cannot write to
-- the blocker, and is told only "recipient unavailable" (the same answer as for a missing account).
CREATE TABLE user_block (
  tenant_id   BIGINT UNSIGNED NOT NULL DEFAULT 1,
  user_uid    VARCHAR(40)     NOT NULL,
  blocked_uid VARCHAR(40)     NOT NULL,
  created_at  DATETIME(3)     NOT NULL,
  PRIMARY KEY (tenant_id, user_uid, blocked_uid),
  KEY idx_block_reverse (tenant_id, blocked_uid, user_uid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
