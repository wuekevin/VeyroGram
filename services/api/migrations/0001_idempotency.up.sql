-- Idempotency keys (Story 1.3, ARCHITECTURE AD-5/AD-8). Rows expire after the configured TTL (default 24h).
CREATE TABLE idempotency_key (
  tenant_id        BIGINT UNSIGNED  NOT NULL DEFAULT 1,
  scope            VARCHAR(190)     NOT NULL,
  key_hash         BINARY(32)       NOT NULL,
  request_hash     BINARY(32)       NOT NULL,
  state            ENUM('in_progress','completed') NOT NULL,
  response_status  SMALLINT UNSIGNED NULL,
  response_ctype   VARCHAR(128)     NULL,
  response_body    MEDIUMBLOB       NULL,
  created_at       DATETIME(3)      NOT NULL,
  expires_at       DATETIME(3)      NOT NULL,
  PRIMARY KEY (tenant_id, scope, key_hash),
  KEY idx_idempotency_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
