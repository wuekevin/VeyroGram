# ADR-0008 配置与密钥处理
状态：Accepted｜日期：2026-11-19
决策：`VG_` 前缀类型化配置；生产启动校验拒绝弱/默认值（固定验证码、默认管理员、短 JWT 密钥、`*` CORS）；密钥来自 systemd credentials 或 0640 的 EnvironmentFile；轮换通过 `kid`；日志脱敏（AD-20、AD-15）。官方示例 `.env`（Aa1234567、wk_secret_123 等）仅作反例。
