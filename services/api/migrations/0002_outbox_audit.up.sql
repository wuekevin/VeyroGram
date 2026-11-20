-- Transactional outbox, consumer dedupe, dead letters and the tamper-evident audit log (Story 1.4; ARCHITECTURE AD-7, AD-15).
CREATE TABLE outbox (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  event_id      CHAR(32)        NOT NULL,
  tenant_id     BIGINT UNSIGNED NOT NULL DEFAULT 1,
  topic         VARCHAR(100)    NOT NULL,
  ordering_key  VARCHAR(190)    NOT NULL,
  payload       MEDIUMBLOB      NOT NULL,
  trace_id      VARCHAR(64)     NOT NULL DEFAULT '',
  status        ENUM('pending','processing','done','dead') NOT NULL DEFAULT 'pending',
  attempts      INT UNSIGNED    NOT NULL DEFAULT 0,
  available_at  DATETIME(3)     NOT NULL,
  locked_until  DATETIME(3)     NULL,
  locked_by     VARCHAR(64)     NOT NULL DEFAULT '',
  last_error    VARCHAR(500)    NOT NULL DEFAULT '',
  created_at    DATETIME(3)     NOT NULL,
  UNIQUE KEY uq_outbox_event (event_id),
  KEY idx_outbox_claim (status, available_at),
  KEY idx_outbox_order (ordering_key, id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE outbox_consumed (
  consumer     VARCHAR(100) NOT NULL,
  event_id     CHAR(32)     NOT NULL,
  consumed_at  DATETIME(3)  NOT NULL,
  PRIMARY KEY (consumer, event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE dead_letter (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  outbox_id    BIGINT UNSIGNED NOT NULL,
  event_id     CHAR(32)        NOT NULL,
  topic        VARCHAR(100)    NOT NULL,
  attempts     INT UNSIGNED    NOT NULL,
  last_error   VARCHAR(500)    NOT NULL,
  failed_at    DATETIME(3)     NOT NULL,
  replayed_at  DATETIME(3)     NULL,
  KEY idx_dead_pending (replayed_at, id),
  KEY idx_dead_event (event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE audit_log (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id   BIGINT UNSIGNED NOT NULL,
  actor       VARCHAR(190)    NOT NULL,
  action      VARCHAR(100)    NOT NULL,
  target      VARCHAR(190)    NOT NULL,
  detail      TEXT            NULL,
  trace_id    VARCHAR(64)     NOT NULL,
  created_at  DATETIME(3)     NOT NULL,
  prev_hash   BINARY(32)      NOT NULL,
  hash        BINARY(32)      NOT NULL,
  KEY idx_audit_actor (actor, id),
  KEY idx_audit_action (action, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- serialises appends to the hash chain; verification never trusts this row (it recomputes from audit_log)
CREATE TABLE audit_head (
  id       TINYINT UNSIGNED NOT NULL PRIMARY KEY,
  last_id  BIGINT UNSIGNED  NOT NULL,
  hash     BINARY(32)       NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- the single head row exists from the start, so concurrent appends only ever lock it (no racy INSERT IGNORE)
INSERT INTO audit_head (id, last_id, hash) VALUES (1, 0, UNHEX(REPEAT('00', 32)));
