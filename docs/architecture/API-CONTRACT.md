# API-CONTRACT — VeyroGram（Phase 1 草案，Draft）

权威源：`packages/contracts/openapi/*.yaml`（OpenAPI 3.1）；本文只定约定与资源清单。

## 通用约定
- 前缀 `/v1`（终端用户与管理后台，按受众分 `/v1/manager/*`）；开放平台 `/open/v1/*`（独立监听，AD-12）。
- 认证：`Authorization: Bearer <access_token>`；开放平台另支持 OAuth2.1 令牌与签名头；**不使用**上游的明文 `token` 头 + 固定验证码模式。
- 幂等：不安全的 POST 接受 `Idempotency-Key`（24h 去重）。
- 追踪：`X-Request-Id` 回显并写日志；兼容 W3C `traceparent`。
- 错误：`{"code":"string","message":"…","trace_id":"…","details":[…]}`；HTTP 语义（400/401/403/404/409/422/429/5xx）；`code` 稳定、可文档化。
- 分页：游标 `?limit=&cursor=`，响应 `{items, next_cursor}`；禁止 offset 深分页。
- 限流：响应头 `RateLimit-Limit/Remaining/Reset`；超限 429 + `Retry-After`。
- 版本与兼容：破坏性变更必须升版本或 ADR；弃用公告 ≥ 一个 minor 版本窗口 [ASSUMPTION：窗口长度待定]。
- 时间 RFC 3339 UTC；ID 为不透明字符串；JSON `snake_case`。

## 资源分组（与官方 Swagger 24 份规范对照，仅作需求参考）
| 分组 | 对应 FR | 官方 Swagger 参考 |
|---|---|---|
| auth / device | FR-1..5, 67 | user |
| users / privacy | FR-29, 61 | user |
| contacts / tags / blocklist | FR-12..14 | friend, label |
| groups | FR-15..19, 64 | group |
| conversations / messages | FR-6..11, 30..34 | message, conversation, channel |
| files / media | FR-22..24, 62 | file |
| favorites | FR-20 | favorite |
| stickers | FR-25 | sticker |
| moments | FR-26..28 | moments |
| location | FR-21 | common |
| rtc | FR-35..37 | rtc, tgocall |
| support | FR-38..40 | hotline |
| search | FR-41 | search |
| reports / moderation | FR-43, 44 | report |
| push / notifications | FR-42 | — |
| manager（RBAC、审计、配置、报表） | FR-45..47, 72 | common, workplace |
| open platform | FR-48..53, 63, 71 | openapi, robot |
| billing | FR-54, 55 | — |
| config / versions | FR-47, 57 | common |

## IM 相关契约
- 客户端 ↔ WuKongIM：沿用其连接协议，令牌由业务服务签发（AD-2/AD-9）。
- 消息类型：`packages/contracts/content-types/`（AD-4）；每个类型含 schema、示例、各端渲染与降级。
- IM → 业务 Webhook：内部监听 + 来源白名单 + 路径密钥；事件幂等（上游发送端不带签名，已源码核实）。上游文档对 Webhook 传输写法有矛盾（gRPC 6979 vs HTTP `?event=`），以对 WuKongIM 固定版本的源码核对结果为准 [UNVERIFIED]。
- 业务服务 → WuKongIM API：仅 `imgw` 适配器，回环/私网（WuKongIM API 不支持 mTLS，不能依赖）；IM→业务 webhook 无签名，靠内部独立监听+网络隔离+路径密钥（AD-2），并规划在 fork 中加 HMAC。
- 发送路径：`msg.before_send` 同步 webhook（AD-21）为业务规则执行点，拒绝码 128–255 透传 SENDACK。

## 兼容层
`compat` 模块提供上游 TangSengDaoDao 旧路径（如需保留旧客户端/数据迁移期），带下线日期；新功能不得加到 compat。

## 契约测试
OpenAPI diff 门禁；schema 验证响应；消息类型 schema 的跨端 golden 样本；Webhook 签名与重放负面用例；Mini App SDK 契约用例。
