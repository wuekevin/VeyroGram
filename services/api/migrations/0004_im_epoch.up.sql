-- IM connection tokens are derived (HMAC) from (user, device class, epoch); revoking a device bumps the epoch of its class, which invalidates the
-- old IM token without ever storing a token in clear (Story 1.13; ARCHITECTURE AD-9: WuKongIM keys tokens by user and device class).
CREATE TABLE im_epoch (
  tenant_id  BIGINT UNSIGNED NOT NULL DEFAULT 1,
  user_id    BIGINT UNSIGNED NOT NULL,
  class      ENUM('app','web','pc') NOT NULL,
  epoch      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  updated_at DATETIME(3)     NOT NULL,
  PRIMARY KEY (tenant_id, user_id, class)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
