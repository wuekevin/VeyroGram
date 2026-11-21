-- Business-side reference of messages sent through POST /v1/messages (Story 1.15; ARCHITECTURE AD-4). Bodies live only in WuKongIM; this table keeps the
-- identifiers business state (tombstones, edits, receipts, retention) will hang off. The unique key mirrors the IM's own idempotency key
-- (sender, client_msg_no, channel), so a retried API call records exactly one row.
CREATE TABLE message_ref (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id     BIGINT UNSIGNED NOT NULL DEFAULT 1,
  message_id    BIGINT UNSIGNED NOT NULL,
  message_seq   BIGINT UNSIGNED NOT NULL,
  from_uid      VARCHAR(40)     NOT NULL,
  channel_id    VARCHAR(100)    NOT NULL,
  channel_type  TINYINT UNSIGNED NOT NULL,
  client_msg_no VARCHAR(64)     NOT NULL,
  created_at    DATETIME(3)     NOT NULL,
  UNIQUE KEY uq_msgref_idem (tenant_id, from_uid, channel_id, channel_type, client_msg_no),
  KEY idx_msgref_message (tenant_id, message_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
