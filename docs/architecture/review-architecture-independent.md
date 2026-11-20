# 架构独立对抗式评审 — VeyroGram Architecture Spine（Phase 1 草案）

- 评审日期：2026-11-19
- 评审对象：`docs/architecture/ARCHITECTURE.md`、`DATA-MODEL.md`、`API-CONTRACT.md`、`SECURITY-THREAT-MODEL.md`、`adr/0002…0008`
- 对照基线：`docs/product/PRD.md`（FR-1…FR-72、NFR-1…NFR-18、SM、OQ）、`BMAD-MASTER.md` §0/§3/§8/§9、`docs/audit/risk-register.md`
- 源码核对：`upstream/WuKongIM@7b810e08`（main 分支 HEAD，本地克隆无 tag，CHANGELOG 仍处 `[Unreleased]`）、`upstream/TangSengDaoDaoServer`
- 评审者未参与上述文档的编写，也没有修改任何被评审文件。

---

## 0. 结论（Verdict）

**结论：不通过，需修订后再评审（REVISE）。**

文档结构合格。分层、模块化单体、outbox、契约优先、原生 systemd 部署、IM API 隔离这些方向都正确，也遵守了大部分不可变约束。问题集中在 Spine 对**WuKongIM 实际行为的假设**上，有几处与源码不符。这部分恰好是"各端各模块分头实现时必须一致"的核心，照现稿交给 Phase 2 实施，几乎一定会出现无法实现或互相矛盾的设计：

1. **发送路径不经过业务层**。客户端经 WuKongIM 网关直接 SEND，但 AD-3/AD-4 默认所有授权、去重都在业务 HTTP handler 内完成。禁言、拉黑、历史可见性、权益配额、`(sender, client_msg_id)` 去重在发送路径上都没有落点。（C-1、C-2）
2. **IM 令牌模型与 WuKongIM 不符**。WuKongIM 每个 `(uid, device_flag)` 只存一个明文、不过期的 token，device_flag 只有 App/Web/PC 三类。AD-9 写的"按设备派生、随设备吊销、≤60s"在同一类设备多台在线时无法成立。（C-3）
3. **IM Webhook HMAC 不可执行**。固定版本的 WuKongIM webhook 发送端不带任何签名头，AD-2/API-CONTRACT 要求的"HMAC + 时间戳"在不改上游的前提下无法实现。（H-1）
4. **消息本体在 WuKongIM 存储里**，但撤回/删除、保留期、注销、搜索、备份/PITR 都按"MySQL 权威"设计，与 AD-5 自己的规则冲突。（C-4、H-6、H-7）

修完 Critical 和 High 项、并补齐未覆盖 FR/NFR 的显式决策（AD 或 Deferred）后，可以重新评审。

---

## 1. 发现清单（按严重度排序）

> 引用格式：`ARCH L41-44` 表示 `ARCHITECTURE.md` 第 41–44 行；WuKongIM 源码路径相对于 `upstream/WuKongIM/`。

### Critical

#### C-1 消息发送路径绕过业务策略引擎：AD-3 只覆盖 HTTP，IM 网关发送没有授权落点
- **引用**：ARCH L46-49（AD-3："every handler declares a policy … route without a policy fails startup"）；ARCH L172（客户端 `WSS/TLS-TCP --> IMGATE`）；PRD FR-12（拉黑后统一失败）、FR-17（禁言服务端拒绝）、FR-18（历史可见性）、FR-29/30（隐私）、FR-54（权益/群人数上限）、FR-64（@所有人受角色与频率限制）。
- **源码事实**：
  - 客户端通过网关 SEND 帧发消息，由 WuKongIM 内部 `internal/usecase/message/permission.go:12-108` 校验。它只认 WuKongIM 自己的频道元数据，包括 `SendBan`、频道解散、频道黑白名单（`internal/access/api/channel_management.go:65-69`，`/channel/blacklist_*`、`/channel/whitelist_*`）以及订阅者关系，不会调用业务服务。
  - 唯一的同步业务钩子是 `[webhook.before_send]`（`wukongim.toml.example:281-293`）。它默认关闭（`enabled = false`），超时只有 500ms，超过 `max_in_flight` 时直接拒绝。
- **问题**：Spine 没有决定 IM 发送授权用哪种机制：
  - (a) 把业务状态（群成员、禁言、黑名单）**镜像**到 WuKongIM 频道元数据。这是双写，有一致性窗口，还依赖 outbox 顺序（见 H-4）。
  - (b) 启用 `before_send` 同步回调。业务服务进入每条消息的关键路径，可用性和延迟要单独设计，`on_error=deny` 时业务服务宕机会导致全站无法发消息。
  - (c) 两者组合。

  不做这个决定，T-05/T-06 的缓解只是纸面结论，FR-12/17/18 的"服务端拒绝"也无法验收。
- **建议**：新增 **AD-21"IM 发送授权"**，明确以下几点：
  1. 哪些规则镜像到 WuKongIM（订阅者、频道黑名单、send_ban），哪些在 `before_send` 中判定（拉黑统一失败、权益、@所有人频控、内容风控）。
  2. 镜像的一致性目标，例如移除成员后 ≤ N 秒不再收发，并给出测试方法。
  3. `before_send` 的超时、熔断、降级策略和 SLO。
  4. 把"IM 网关路径"加入 AD-3 生成式负面测试（被移除成员、被拉黑者、禁言者直接构造 SEND 帧）。
  5. T-05/T-06 改为引用该 AD。

#### C-2 去重与 `message_ref` 的数据来源不成立
- **引用**：ARCH L54（AD-4："the business layer dedups by `(sender, client_msg_id)`"）；DATA-MODEL L20（`message_ref` 中 `(sender, client_msg_id)` 唯一）；PRD FR-6。
- **事实**：业务层不在发送路径上（见 C-1），只能经异步 webhook `msg.notify`（`wukongim.toml.example:266-279`：批量发送、`queue_size=1024`、`retry_max_attempts=3`）事后得知消息。`message_ref` 的唯一约束只能事后发现重复，无法阻止重复投递。WuKongIM 内部 `client_msg_no` 是否做发送端去重、窗口多长，本次未能在源码中确认（`submit_batch.go` 只在 trace 中携带 `ClientMsgNo`）。
- **建议**：
  - AD-4 改为"去重由 IM 内核基于 `client_msg_no` 完成（需对固定版本做源码核实与契约测试）；业务层 `message_ref` 只用于索引/撤回/编辑，允许最终一致"。
  - 如果核实发现 WuKongIM 不去重，就必须启用 `before_send`，在其中做去重（Redis SETNX + TTL），并写明窗口。
  - 同时规定 `client_msg_id`（ULID）与 WuKongIM `client_msg_no` 的映射关系。

#### C-3 IM 令牌模型与 WuKongIM 实际实现不符：无法按设备吊销，也没有生命周期
- **引用**：ARCH L79（AD-9："IM tokens are derived per device and revoked with it (target ≤ 60 s)"）；DATA-MODEL L17（`im_token`，撤销 device 即撤销全部令牌）；PRD FR-3（被下线设备 ≤60s 不能再用于 API 与 IM 连接）、NFR-15（令牌有生命周期、绑定设备、可吊销）。
- **源码事实**：
  - `internal/usecase/user/app.go:14-46` 的 `UpdateToken` 以 `(UID, DeviceFlag)` 为键 upsert 一个 token。`internal/contracts/protocolmeta/types.go:8-13` 中 DeviceFlag 只有 `App / Web / PC` 三个值。
  - `app.go:49-70` 的 `VerifyToken` 只做常量时间比较，**没有过期时间**，token 在 WuKongIM 中以原文持久化。
  - `app.go:42-44` 只有 `DeviceLevelMaster` 会触发 `kickLocalDevice`。`app.go:72` 的 `DeviceQuit` 注释写明只关闭 **owner-local** 会话，集群模式下其他节点上的连接是否被踢掉，需要另行核实。
  - `pkg/gateway/auth.go:63-76` 只在 CONNECT 时校验 token。已建立的连接在 token 被更换后仍然存活，除非被踢。
- **问题**：
  1. 同一用户的两台手机（或手机 + iPad）共用 App 槽位，后登录的设备会覆盖或踢掉前一台。产品层"设备列表/逐台下线"与 IM 层"按设备类别"不一致。
  2. "生命周期"只能靠业务方定期轮换 token 来模拟。
  3. ≤60s 吊销依赖 `device_quit` 能跨节点生效。
  4. 访问令牌（JWT）的吊销延迟也没有设计：AD-9 只写了"short-lived"，没有给出 TTL 上限，也没有 denylist 检查，FR-3 的"≤60s 不能用于 API"因此无法保证。
- **建议**：重写 AD-9 的 IM 部分：
  - 明确"IM 会话 = `(uid, device_flag)`"。如果要支持同类多设备，需要评估 uid 后缀方案（如 `uid@deviceid`）对会话、已读、推送的连锁影响，并以 ADR 决策。
  - IM token 用 CSPRNG 生成 ≥256 bit 随机值，不要"从 JWT 派生"。业务库只存哈希。
  - 吊销流程：`/user/device_quit` 加主动 kick，并对集群模式做契约测试。
  - 访问令牌 TTL ≤ 5 分钟，并加 Redis `jti`/设备 denylist 检查，使 FR-3 的 60s 可证明。
  - 在 ADR 中把"WuKongIM 连接后不复验 token"登记为已接受的残余风险。

#### C-4 "MySQL 权威"与"消息本体在 WuKongIM"冲突：撤回、删除、保留、注销没有可执行路径
- **引用**：ARCH L56-59（AD-5："MySQL … is the system of record … WuKongIM keeps its own message store"）；ARCH L54（撤回/编辑为 tombstone，"read path (history, search, downloads) checks the tombstone"）；DATA-MODEL L60-61（保留/删除由 worker 清理）；PRD FR-5、FR-10（撤回后接口/搜索/预览/缓存 URL 均失效）、FR-66、FR-72。
- **事实**：
  - 消息正文存在 WuKongIM 自己的存储中。WuKongIM 的 Manager（5301）、legacy `/channel/messagesync`、消息查询接口都能直接返回原文。
  - 在线客户端已经通过网关收到正文，墓碑只能在业务读路径上生效。
  - 至于 WuKongIM 是否支持对单条消息物理删除或覆写正文，Spine 写的是"worker 清理"，但没有给出对应的 WuKongIM 能力依据。CHANGELOG 中保留只按频道推进。
- **建议**：
  - 修订 AD-5：**消息正文的权威存储是 WuKongIM，业务状态的权威存储是 MySQL**，两者都要有备份和保留策略。
  - 新增 AD"消息读路径"：所有历史/会话同步必须经业务 API 代理到 WuKongIM（客户端不得直连 5001，这与 AD-2 一致），以便统一执行墓碑、历史可见性和隐私过滤。撤回通过 WuKongIM 的消息更新/事件机制（`internal/access/api/message_update.go`、`message_event.go`）下发给全部设备，并对固定版本核实"撤回后服务端是否仍返回原文"。
  - 单条删除、注销和保留期清理在 WuKongIM 侧的实现方式，在核实前标 `[UNVERIFIED]` 列入 Deferred，不能写成既定能力。

### High

#### H-1 IM → 业务 Webhook 的 HMAC 签名在固定版本上不可实现
- **引用**：ARCH L44（AD-2："Inbound IM webhooks/events are authenticated (shared HMAC + timestamp, source IP allowlist)"）；API-CONTRACT L42（HMAC + 时间戳 + 来源白名单；gRPC 与 HTTP 两种写法的矛盾标为 `[UNVERIFIED]`）；STM 信任边界 3。
- **源码事实**：
  - `internal/runtime/webhook/sender.go:41-75` 只设置 `Content-Type`，以 `POST <http_addr>?event=<name>` 发送，**没有签名、时间戳或认证头**。`before_send` 用的也是同一种 HTTP 回调（`wukongim.toml.example:281-293`）。
  - 上游 TSDD Server 自己也承认这一缺口：`modules/webhook/api.go:111` 写着 `// TODO: 此接口需要与IM做安全认证`。
  - gRPC 6979 只是 TSDD Server 旧版的配置（`configs/tsdd.yaml:5`，已注释）。固定版本的 WuKongIM 只有 HTTP，API-CONTRACT 的 `[UNVERIFIED]` 现在可以关闭。
- **建议**：AD-2 改为可执行的组合：
  1. 业务服务为 IM 回调开**独立内部监听端口**，只绑回环或私网，不经 Nginx。
  2. `webhook.http_addr` 使用带高熵密钥的路径（例如 `http://10.0.0.2:7080/im/<secret>/events`，`sender.go` 会保留原 URL 只追加 query）。业务服务以常量时间比较校验，并按 ADR-0008 轮换该密钥。
  3. 来源 IP 白名单。
  4. 事件幂等。

  如果坚持要 HMAC，就要提交 ADR 说明维护上游补丁的成本。同时删除 API-CONTRACT L43 中"内部 mTLS"的说法：WuKongIM API 不支持 TLS，见 H-2。

#### H-2 IM 网关没有原生 TLS，"TLS-TCP"依赖未核实的 Nginx stream 与 PROXY 协议，真实 IP 可被伪造
- **引用**：ARCH L44、L172（客户端 `WSS/TLS-TCP` 直连网关）；AD-16 L111-114 只覆盖 HTTP 转发头；AD-14 L104；BMAD-MASTER §9（"WuKongIM TCP 直连场景经 TLS 与安全评估才可开放"）；NFR-4、NFR-15。
- **源码事实**：
  - `wukongim.toml.example:181-195` 写明网关"TLS for wss:// terminates upstream"，默认监听器为 `0.0.0.0:5100`（tcp-wkproto）和 `0.0.0.0:5200`（ws），都是明文。
  - 同一处注释："Empty/omitted CIDRs accept [PROXY] headers from any TCP peer, so clients can assert arbitrary addresses"。
  - 网关代码中没有 `crypto/tls`。
- **问题**：
  - 部署图（ARCH L185-208）中没有 Nginx 到 WuKongIM 网关这条边。
  - 宝塔管理的 Nginx 是否带 `stream` 模块做 TLS-TCP 终止，没有核实。
  - 没有规定配置 `proxy_protocol_trusted_cidrs`，风控和限速拿到的 IP 可能被伪造。
  - 默认 `0.0.0.0` 与 AD-14 的"只经 Nginx 暴露"矛盾。
- **建议**：
  - AD-14 / AD-16 补充：网关监听改为私网或回环；Nginx 负责 WSS（http upgrade）和 TLS-TCP（stream，需核实宝塔构建）终止；启用 PROXY 协议并配置 `proxy_protocol_trusted_cidrs`。
  - TLS-TCP 直连在完成 §9 要求的安全评估前不开放，首发只开 WSS。
  - 补充网关 CONNECT 的限速和暴力破解防护（STM 目前缺少这一项）。

#### H-3 IM 内核的暴露面清单不完整：集群端口、插件、MQTT、Prometheus 都未纳入
- **引用**：ARCH L44（负面测试只列 5001/5301/metrics）；ADR-0006；T-01。
- **源码事实**（`wukongim.toml.example`）：
  - L14、L22：集群 `127.0.0.1:7001`，`join_token = "change-me"`。
  - L95-97：Manager 默认 `jwt_secret = "wukongim-dev-manager-secret"`。
  - L108：`[bench] api_token = ""`。
  - L127：Prometheus `9099`。
  - L295-306：`[plugin] enable = true`、`hot_reload = true`，插件目录和 socket 可加载代码。
  - L314：MQTT `0.0.0.0:1883`（默认关闭）。
  - `internal/access/api/server.go:600` 有 `/plugins/:plugin_no/*path`；`:670-673` 有 `/debug/pprof`；`:592-593` 有 `/route`。
- **建议**：
  - ADR-0006 / AD-2 列出完整的端口和功能清单，逐项给出处置（关闭、仅回环、带认证）。
  - 生产环境 `plugin.enable=false`，或只允许签名插件、关闭 `hot_reload`，并在 STM 中新增插件供应链威胁。
  - `join_token`、Manager `jwt_secret`、`bench.api_token` 纳入 AD-20 启动校验。注意它们属于 WuKongIM 进程，需要在 `infrastructure/scripts` 中加预检。
  - 负面可达性测试覆盖 7001/9099/1883/5301/5001/pprof/plugin 路由。
  - legacy 客户端依赖 WuKongIM 的 `/route` 发现网关地址，需在业务 API 中提供替代接口（API-CONTRACT 目前未列出）。

#### H-4 Outbox 没有顺序保证，IM 状态镜像和 webhook 会乱序
- **引用**：ARCH L66-69（AD-7：`SKIP LOCKED`、至少一次、消费者幂等）；ADR-0004。
- **问题**：
  - 多个 worker 用 `SKIP LOCKED` 并行领取，天然不保证同一聚合内的顺序。例如"移除成员"和"再加入"、"禁言"和"解禁"乱序后，C-1 的镜像状态会出错：被移除成员可能仍能收发。
  - 消费者用"event ID 去重"对外部端（APNs/FCM、开发者 webhook）不成立，外部端无法替我们去重。
  - outbox 是轮询的，轮询间隔与 FR-3 的 60s、FR-19 的 5s 扩散目标之间的预算没有给出。
  - 缺少 outbox 表的清理和归档策略，也没有处理毒消息的上限。
- **建议**：AD-7 增加：
  1. `partition_key`（聚合 ID）并按分区串行领取，或在事件中携带聚合版本号，由消费方按"版本单调"做幂等 upsert。
  2. 外部投递语义写明"至少一次，接收方需按 `event_id` 去重"（开放平台 webhook 契约要求开发者去重）。
  3. 轮询或 `LISTEN` 替代方案的延迟 SLO。
  4. 已投递行的保留与清理。

#### H-5 哈希链审计与 DB 权限规则自相矛盾，链锚定缺失
- **引用**：ARCH L109（AD-15："append-only `audit_log` (hash-chained per day); the app DB user has no UPDATE/DELETE"）；ARCH L59（每表都有 `updated_at`、软删除列）；DATA-MODEL L28、L61（审计保留期由 worker 清理）；PRD FR-46。
- **问题**：
  1. worker 清理过期审计需要 DELETE，与"无 DELETE"冲突。如果 worker 和 api 共用一个 DB 账号，这条约束就失效了。
  2. 迁移账号、DBA 账号能改表并重算整条链。没有外部锚定（例如每日链头写入只读存储或 WORM 对象存储、对外公告或签名），篡改检测只在应用层有效。
  3. "按日"单链要求串行写入，高并发下链头锁会成为热点。
  4. AD-5 要求"每表有 `updated_at`/软删除"，这不适用于追加写表。
- **建议**：
  - AD-15 写明使用三个独立 DB 账号：`vg_app` 只有 INSERT/SELECT；`vg_audit_retention` 只能 DELETE 超期分区，并且要审批；`vg_migrate` 只在发布窗口使用。
  - 按日分区，用 `DROP PARTITION` 做保留，避免行级 DELETE。
  - 每日链头用 Ed25519 签名（标准库，不自造构造），并写入独立的 WORM 或对象锁存储。
  - 并发设计采用"按分片多链 + 每日合并根"。
  - AD-5 对追加写表开例外。
  - AD-20 用 `SHOW GRANTS` 在启动时校验这些账号的权限。

#### H-6 备份/灾备（NFR-6）没有任何 AD，WuKongIM 存储的备份与一致性恢复完全缺失
- **引用**：ARCH L56-59（AD-5 binds NFR-6，但规则中没有备份内容）；部署图 L190-198（`wukongim.service` 被放在"app tier"，实际是有状态组件）；BMAD-MASTER §3（"IM 数据存储按 WuKongIM 实际内核要求单独设计"）、§8/§9（PITR、恢复演练、备份加密）；PRD NFR-6、SM-7、UJ-7。
- **问题**：
  - 没有 MySQL binlog/PITR 方案，没有 WuKongIM 数据目录的快照方案（是否需要停机，集群一致性快照如何做），也没有说明两个存储如何对齐到同一恢复点：MySQL 中的 `message_ref` 与 WuKongIM 中的序号可能错位。
  - 对象存储版本化、备份加密和密钥管理同样没有设计。
  - R-09（WuKongIM channel promise 激活后不可回滚）没有纳入升级和回滚设计。
- **建议**：
  - 新增 **AD"备份、恢复与升级回滚"**，覆盖 MySQL PITR、WuKongIM 快照方式（核实官方支持方式）、跨存储恢复后的对账脚本（以 WuKongIM 序号为准重建 `message_ref`）、备份加密、季度恢复演练，以及 WuKongIM 升级前的不可回滚检查清单（R-09）。
  - 部署图把 WuKongIM 标为有状态组件，并单列其磁盘与副本规划。

#### H-7 搜索方案与数据归属矛盾：MySQL 中没有消息正文，WuKongIM 内核也没有搜索路由
- **引用**：ARCH L126-129（AD-19："start with MySQL/IM-native search"）；ARCH L59（业务库只存引用）；PRD FR-33、FR-41。
- **源码事实**：
  - 在 `internal/access/api/server.go` 的路由注册（L574-673）中没有消息搜索路由。
  - 上游 TSDD 的 `modules/search/api.go:75` 调用的是 `ctx.IMSearchUserMessages`，实现在未克隆的 `TangSengDaoDaoServerLib` 中。历史上它依赖 WuKongIM 的搜索插件，走 `/plugins/...` 路由（本次未核实）。
  - 这与 H-3"生产关闭插件"的建议直接冲突。
- **建议**：
  - AD-19 标为 `[UNVERIFIED]`，并列出三个候选：WuKongIM 搜索插件（需审计并签名）、业务侧建立受控的正文索引（违反 AD-5，需 ADR）、仅客户端本地搜索加服务端元数据搜索。
  - 无论选哪种，撤回、删除、历史可见性都必须同步到索引（FR-10/33）。
  - 在完成核实前，不要写成"先用 MySQL"。

#### H-8 签名 URL 无法"立即失效"，与 FR-10/FR-22/FR-62 冲突
- **引用**：ARCH L64（AD-6：短时签名 URL）；PRD FR-22（"成员资格被移除后 URL 立即失效"）、FR-10（"缓存下载 URL 均失效"）、FR-62（"期间不可通过旧 URL 访问"）。
- **问题**：S3 预签名 URL 在过期前无法吊销。"立即"只能靠两种方式实现：一是业务网关代理下载（每次请求鉴权），二是 CDN 鉴权加回源校验。只缩短 TTL 意味着接受一个时间窗口。
- **建议**：AD-6 写明二选一：
  - 敏感会话走 `media` 代理下载（Nginx `X-Accel-Redirect` 或 `auth_request`，鉴权后再签发 ≤60s URL）。
  - 或者接受 TTL 窗口（例如 ≤120s），并把 PRD 中的"立即"改为"≤TTL"，回推 PM 修订 PRD。

  同时补充：扫描引擎选型（ClamAV 等）列入 Deferred；转码和文档预览在沙箱中执行（BMAD-MASTER §8 中"转码沙箱""恶意文档预览风险"目前没有 AD 覆盖）。

#### H-9 小程序"独立 origin + 分区存储 + Bridge 签名"的端上可行性未经核实，却写成了既定决策
- **引用**：ARCH L86-89（AD-11）；ADR-0005（状态 Accepted）；PRD FR-50；BMAD-MASTER §6。
- **问题**（以下都是未核实的技术主张，被写成了事实）：
  1. **iOS WKWebView**：默认数据存储为全局共享。按小程序隔离需要 `WKWebsiteDataStore(forIdentifier:)`（较新系统版本才有）或非持久化存储。最低部署目标尚未确定（ARCH L159），可行性取决于这个目标。
  2. **Android WebView**：同一进程默认只有一个 profile。按应用分区要么用 `setDataDirectorySuffix`（每个进程一个，需要多进程），要么用 androidx.webkit 的 Profile/MultiProfile API（依赖 WebView 版本）。Spine 都没有说明。
  3. **Bridge "nonce + signature"**：签名密钥如果在小程序 JS 中，同源的任意脚本都能签名，签名不提供额外安全性。真正的边界是原生侧对**调用帧 origin**的校验：iOS 用 `WKScriptMessage.frameInfo.securityOrigin`，Android 用 `WebViewCompat.addWebMessageListener` 的 `allowedOriginRules`，并禁止 `addJavascriptInterface`（它会暴露给所有 frame）。这套"签名"属于自造协议，与"不自研密码学"的约束有张力。
  4. 包加载方式没有定：远程站点 `<app_id>.apps.<domain>`（需要通配证书和 DNS，CSP 的 `connect-src` 由清单生成），还是本地签名包经 `WKURLSchemeHandler` / `WebViewAssetLoader` 映射为虚拟 origin。两者的 origin 模型和"签名包"校验点完全不同。
  5. AD-11 中"revoke/rollback propagate within the SLA in PRD SM-6"引用错误：SM-6 的 60s 是授权撤销时间，回滚在 FR-51 中是"下一次打开生效"。
- **建议**：
  - ADR-0005 降为 Proposed，补一个 Phase 3 的端上 spike（iOS/Android/Electron 各一），验证分区存储、origin 校验和包加载方式。
  - AD-11 把"Bridge 签名"改为"原生侧校验调用帧 origin + 一次性 nonce 防重放 + 清单授权"，删除含义不清的"signature"。
  - Electron 使用 `session.fromPartition('persist:<app_id>')`，这一点可行，可以写明。
  - 修正 SM-6 的引用。

#### H-10 租户感知规则不可强制执行，也没有覆盖 IM、Redis 和对象存储
- **引用**：ARCH L59、L81-84（AD-10："forbidden by repository helpers and lint"；每表都有 `tenant_id`）；DATA-MODEL L6；PRD FR-60、FR-39（客服跨租户不泄露）。
- **问题**：
  1. 通用 lint 无法可靠识别"未带 tenant 的 SQL"。
  2. 唯一索引（手机号盲索引、`(sender, client_msg_id)`、`public_id`）是否包含 `tenant_id` 没有定义。
  3. WuKongIM 没有租户概念，uid/channel_id 的命名空间没有规定。
  4. Redis key、对象存储前缀、审计链是否按租户划分没有规定。
  5. "每表都有 `tenant_id`"把 `schema_migration`、`outbox` 这类平台表也包含进去，规则过宽。
- **建议**：
  - 采用生成式查询层（如 sqlc）。repo 方法签名强制传入 `TenantID` 类型，再配合静态检查（AST 规则：`database/sql` 只能在 adapters 包内使用）和集成测试（双租户数据下查询结果不串）。
  - 列出平台表豁免清单。
  - 规定 IM uid/channel_id、Redis key、S3 prefix 的租户编码（v1 可固定为 `t1`，但格式先定下来）。

#### H-11 管理后台 API 与终端用户 API 共用公网监听
- **引用**：API-CONTRACT L6（`/v1/manager/*` 与终端用户共用 `/v1`）；AD-12 只把开放平台拆成独立监听；NFR-16、T-09。
- **建议**：
  - 管理后台 API 使用独立监听和域名，在 Nginx 层加 IP 白名单或零信任接入，强制 MFA，并采用 cookie + CSRF 或 bearer 二选一（写明）。
  - 与 AD-12 对称，新增 AD"Manager 边缘"。
  - STM 补充宝塔面板（8888 等）的暴露威胁：它本身就是高权限管理面。

### Medium

- **M-1 MySQL 版本与不可变约束冲突**：ARCH L152 的 Stack 表写"target 8.4 LTS"，而 BMAD-MASTER §0.5、PRD NFR-4 都要求 MySQL 8.0，ADR-0002 也还是 Proposed（待用户确认）。Stack 表应写"8.0（约束）；8.4 兼容矩阵待 ADR-0002 决策"，不能提前把 8.4 写成目标。"8.0 已于 2026-04 EOL、最后版本 8.0.46"这一说法引用了 Oracle 公告，本次评审无法在线核实，应附 URL 和核实日期。如果最终选 8.4，需要同步修订 PRD NFR-4。
- **M-2 legacy 路径与 `/v1` 冲突**：上游 TSDD 的旧路径本身就是 `/v1/*`（例如 `modules/webhook/api.go:105`），compat 模块和新 `/v1` 会撞路由。AD-8 应规定 compat 使用独立前缀（如 `/legacy/v1`）或独立监听，并写明下线日期的决策人。
- **M-3 WuKongIM 不是"tagged version"**：ARCH L124、L151 写"pinned to a tagged version"，但本地克隆是 main 分支 HEAD `7b810e08`（grafted，无 tag，CHANGELOG 为 `[Unreleased]`）。应写明"当前审计基线为未发布 main SHA，Phase 2 必须改钉到正式 tag 并重新做源码核对"。go1.26.10 已在 `go.mod`（`toolchain go1.26.10`）中核实。
- **M-4 客服访客（未登录）接入 IM 未设计**：PRD FR-38 要求"访客（登录或未登录）可发起咨询"。WuKongIM 有 visitors 频道类型（`internal/usecase/message/send.go:22`，`channelTypeVisitors = 10`），网关鉴权中 `isVisitor` 会跳过 token 校验（`pkg/gateway/auth.go:63`），但当前 wiring 没有注入 `IsVisitor`（`internal/app/wiring.go:1433-1445`）。匿名访客的身份签发、限速、滥用防护都没有 AD；`IsBanned` 也没有注入，封禁用户能否在 CONNECT 时被拒，需要核实。
- **M-5 推送触发链路与 webhook 丢失**：离线推送依赖 WuKongIM webhook（`retry_max_attempts=3`，队列满时的行为未知）。业务服务宕机超过重试窗口，事件就会丢失，造成漏推和 `message_ref` 缺口。需要对账任务（按 WuKongIM 序号补齐）以及 webhook 积压和丢弃的监控指标（AD-15）。
- **M-6 链接预览 SSRF 出口没有落到架构**：T-08 要求"独立出口"，但拓扑和模块（AD-1 模块列表）中都没有 egress 代理或 `linkpreview` 组件，FR-65 也没有映射。
- **M-7 敏感字段加密方案不具体**："加密或哈希存储，密钥分级、可轮换"（DATA-MODEL L8）没有指定 KMS 或密钥层级，也没有指定 AEAD 实现。应写明使用标准 AEAD（AES-256-GCM / XChaCha20-Poly1305）、经审计的库（如 Tink 或 Go 标准库），信封加密，数据密钥按 `kid` 轮换，避免自造构造。通讯录手机号空间很小（约 10^10），普通哈希可被穷举，FR-13 匹配必须使用服务端密钥 HMAC 加限频，STM 的 abuse case 也应写明这一点。
- **M-8 wkproto 应用层加密**：网关默认协商 `wkprotoenc` 会话加密（`pkg/gateway/auth.go:117-138`）。这是上游自定义方案，Spine 应声明安全性只依赖 TLS，不得把 wkproto 加密作为对外的安全承诺（呼应"诚实声明"和"不自研密码学"），并评估在 TLS 下是否关闭。
- **M-9 NFR-10 灰度/回滚脚本、日志轮转、安装向导首次初始化竞态**：AD-14 没有覆盖。BMAD-MASTER §9 明确要求灰度发布、回滚脚本、日志轮转和备份恢复。安装向导在完成前如果暴露在公网，攻击者可能抢先创建首个管理员，需要一次性安装令牌（从控制台或文件读取）。
- **M-10 DATA-MODEL 与约定不一致**：约定写"tables `snake_case` plural"（ARCH L140），但 DATA-MODEL 全部是单数（`user`、`device`、`order`…），且 `order` 是 MySQL 保留字。应统一约定（建议单数加模块前缀，并避开保留字）。
- **M-11 依赖漏洞阻断与 SAST/DAST/fuzz 没有 AD**：AD-18 和约定只写了 SBOM/license，NFR-17 的 SAST/DAST/secret scan/fuzz 以及"高危漏洞阻断发布"（R-13、R-18）没有规则承载。

### Low

- **L-1** ARCH L143 写"non-Docker service containers"，用语自相矛盾。CI 中的服务容器本质就是容器，而约束只限制生产环境。建议改为"CI 可用容器化服务实例或原生 runner；生产禁止 Docker"。
- **L-2** AD-2 的 Binds 中列入 FR-35..37（音视频）没有依据，建议删除或说明理由。
- **L-3** Stack 表中的 Android `compileSdk 35+`：到 2026-11，Google Play 的 targetSdk 要求很可能已经上调（未核实），建议标 `[UNVERIFIED]`，在 Phase 3 前按 Play 政策核实。Node 版本状态（20 EOL / 22 maintenance / 24、26 LTS）同样应附核实来源。
- **L-4** API-CONTRACT 中 `Idempotency-Key` 的作用域（用户 + 路由 + 请求体哈希）和冲突返回码（409/422）没有定义。
- **L-5** Capability Map 的最后一行把"NFR-1..18 cross-cutting"笼统映射到 5 个 AD，掩盖了 NFR-6/9/11/17 没有规则的事实（见 §2）。
- **L-6** Redis 8 的许可证（RSALv2/SSPLv1/AGPLv3 三选一）应登记到 `license-and-ip.md`。自托管使用没有障碍，但需要留存记录。

---

## 2. FR/NFR 覆盖缺口

`ARCHITECTURE.md` L11 声明 `binds: FR-1..FR-72, NFR-1..NFR-18`，逐条核对结果如下。

### 2.1 完全没有 AD 映射，也没有显式 Deferred
| ID | 说明 | 建议 |
|---|---|---|
| FR-58 频道/广播与运营消息 | Capability Map 和 Deferred 中都没有（PRD §6.2 标为 P1/P2） | 在 Deferred 中写明"架构预留：IM 频道类型 + 退订表 + outbox"，或新增 AD |
| FR-59 AI 辅助 | 只有 STM T-22，没有 AD（模型网关、同意记录、数据最小化） | 加入 Deferred 并注明 T-22 约束 |
| FR-61 个人资料与通用设置 | API-CONTRACT 有分组，Capability Map 中没有 | 映射到 `identity` + AD-3/AD-6（头像校验） |
| FR-65 语音、名片、链接预览 | 只有 T-08，没有组件和 AD | 见 M-6 |
| FR-66 清空记录与服务端删除范围 | 没有映射，且依赖 WuKongIM 删除能力 | 见 C-4 |

### 2.2 有映射但没有可执行规则（名义覆盖）
| ID | 缺口 |
|---|---|
| FR-3 | API 和 IM 令牌 ≤60s 吊销都没有机制（C-3） |
| FR-5 | 注销和导出涉及 WuKongIM 中的正文，导出产物的签名下载与过期没有落到 AD-6 |
| FR-6 / FR-8 | 去重落点（C-2）；内容类型注册表的 CI 门禁已有（AD-4），但"缺任一项 CI 失败"没有给出执行工具 |
| FR-10 / FR-33 / FR-41 | 撤回在 WuKongIM 存储、搜索索引和签名 URL 上的一致性（C-4、H-7、H-8） |
| FR-11 / FR-67 | 二维码签发、过期、撤销，扫码登录"未批准会话不能读历史"（需规定 IM token 只在批准后签发），没有 AD |
| FR-12 / FR-17 / FR-18 / FR-64 | IM 发送路径授权（C-1） |
| FR-13 | 通讯录哈希可被穷举（M-7） |
| FR-24 | 转码沙箱、编解码许可（OQ-6）没有 AD |
| FR-30 / FR-31 | 大群已读扇出上限、在线状态隐私（WuKongIM `/user/onlinestatus` 的暴露路径）没有规则 |
| FR-38..40 | 匿名访客接入 IM（M-4）；客服跨技能组隔离只写了 AD-10 |
| FR-42 | 推送触发链路与丢失补偿（M-5）；VoIP/CallKit 依赖 OQ-5，没有列入 Deferred |
| FR-54 | 权益（群人数、文件配额）在 IM 路径和上传路径上的强制点没有规定 |
| FR-57 | 客户端更新签名只在 AD-17 中对 Electron 提到，移动端强制更新的契约没有写 |

### 2.3 NFR 缺口
| ID | 状态 |
|---|---|
| **NFR-6 可靠性与灾备** | **未覆盖**（H-6） |
| **NFR-9 规模与性能** | **未覆盖**：没有 AD 规定压测阶梯、扇出上限、WuKongIM beta 的容量验证门槛 |
| **NFR-11 许可证合规** | **未覆盖**：没有 AD，@fancyapps/ui 阻断项（R-28）没有落点 |
| **NFR-17 安全质量门禁** | 部分覆盖：只有 SBOM/license/race，缺 SAST/DAST/secret scan/fuzz 和漏洞阻断（M-11） |
| NFR-4 传输与数据 | 部分覆盖：IM 网关 TLS 终止（H-2）、静态加密（M-7）、MySQL 版本（M-1） |
| NFR-8 可访问性与国际化 | 名义覆盖：AD-17 binds 了它，但规则里没有 a11y/i18n 内容 |
| NFR-10 部署 | 部分覆盖：缺灰度、回滚脚本、日志轮转（M-9） |
| NFR-14 移动端安全 | 名义覆盖：AD-17 中没有安全存储、证书校验、备份排除的规则（只出现在 STM T-14） |
| NFR-15 令牌与连接 | 与 WuKongIM 实现不符（C-3） |
| NFR-16 端口暴露 | 清单不完整（H-3、H-11） |

---

## 3. 风险映射缺口（risk-register High/Critical → AD/T-xx）

| 风险 | 级别 | 映射 | 评审意见 |
|---|---|---|---|
| R-02 固定验证码 | 高 | AD-9/AD-20、T-02 | 已覆盖 |
| R-03 默认管理员口令 | 高 | AD-9、T-03 | 已覆盖；补充安装向导抢占防护（M-9） |
| R-05 Electron 26 / React 17 | 高 | AD-17、T-13 | 已覆盖 |
| R-12 第三方镜像 lockfile | 高 | AD-18、T-15 | 已覆盖 |
| R-13 npm 高危/严重漏洞 | 高 | T-15 | **部分覆盖**：AD-18 没有"高危漏洞阻断发布"规则（M-11） |
| R-17 iOS Pods 无法编译 | 高 | AD-17（"stale Pods replaced"） | 已覆盖（构建风险，不需要 T-xx） |
| R-20 .env 硬编码口令 | 高 | ADR-0008、T-19 | **部分覆盖**：WuKongIM 自身的 `join_token`、Manager `jwt_secret`、`bench.api_token` 没有纳入启动校验（H-3） |
| R-21 WuKongIM API 无鉴权 | 严重 | AD-2、ADR-0006、T-01 | 已覆盖 5001；**但 webhook 反向信任（H-1）、集群/插件/MQTT 端口（H-3）缺失** |
| R-24 TURN/推送密钥写死 | 高 | AD-13（TURN 短时）、T-14 | **部分覆盖**：推送厂商密钥如何分发（客户端只放 AppID，服务端保存密钥）没有 AD |
| **R-28 @fancyapps/ui 商业授权** | 高 | **无** | **未映射**：需要 AD/ADR 或 Release 门禁条目（NFR-11） |

中风险中值得补映射的：R-09（WuKongIM 不可回滚，应纳入 H-6 的升级 AD）、R-18（govulncheck，应纳入 M-11）、R-22（adminer/MinIO 控制台暴露，应在 NFR-16 清单中显式写出）、R-29（WuKongIM beta，应作为 NFR-9 容量门槛的前提）。

---

## 4. 不可变约束核对（BMAD-MASTER §0/§3/§8/§9）

| 约束 | 结论 |
|---|---|
| 生产禁止 Docker | 符合（AD-14）；L143 用语需澄清（L-1） |
| Go 后端 | 符合 |
| MySQL 8.0 | **存在偏离倾向**：Stack 表把 8.4 写成目标（M-1），需等用户决策 |
| Redis 8 | 符合；版本标为未核实，处理诚实 |
| Nginx（宝塔）/ systemd | 符合；TLS-TCP stream 终止能力未核实（H-2） |
| 不自研密码学 | **有张力**：Bridge "nonce + signature"（H-9）、哈希链构造（H-5）、wkproto 加密（M-8）需改为标准构造并写清楚 |
| 诚实声明 | 大体符合（E2EE、多租户、性能都没有承诺）。以下几处把未核实内容写成了事实：webhook HMAC（H-1）、到 WuKongIM 的 mTLS（API-CONTRACT L43）、IM token 按设备（C-3）、MySQL 搜索（H-7）、小程序分区可行性（H-9）、"tagged version"（M-3） |
| §3 IM 存储单独设计 | **未满足**（C-4、H-6） |
| §8 转码沙箱 / 文档预览 / 备份加密 | **未满足**（H-8、H-6） |
| §9 灰度、回滚、日志轮转、安装向导、真实 IP | **部分满足**（M-9、H-2） |

---

## 5. 已经正确或可以保留的部分
- AD-2 的方向（5001 仅回环，由 `imgw` 独占调用）与源码一致：ADR-0006 引用的 `server.go:372`（`openCORSMiddleware`）、`:421-423`（只有 debug/维护/epoch 中间件）、`wukongim.toml.example:80`（`127.0.0.1:5001`）都已逐行核实无误。
- `gateway.token_auth_on` 默认为 true（`internal/config/build.go:46`、`wiring.go:120-121`），AD-2 中"客户端持业务签发的 token 接入"的基础前提成立。
- AD-3（无 policy 则启动失败）、AD-7（outbox）、AD-8（OpenAPI diff 门禁）、AD-12（开放平台独立边缘、PKCE、精确回调）、AD-13（端口化加短时 TURN）、AD-16、AD-20 都是可执行的好规则。
- ADR-0007 对未核实版本和许可的处理诚实；ADR-0002 没有擅自更改用户偏好。

---

## 6. 修订优先级建议
1. **先改 IM 集成相关的决策**：C-1、C-2、C-3、C-4、H-1、H-2、H-3。建议新增 AD-21（IM 发送授权）和 AD-22（消息读路径与正文归属），重写 AD-2、AD-4、AD-9 的相关段落，然后对 WuKongIM 固定版本做一次专项源码核对，形成 `docs/audit/wukongim-integration-facts.md`。
2. **补齐缺失的 AD**：备份/恢复/升级回滚（H-6、NFR-6）、规模与压测门槛（NFR-9）、许可证门禁（NFR-11、R-28）、安全质量门禁（NFR-17）、Manager 边缘（H-11）。
3. **修正有内部矛盾的规则**：AD-7 顺序（H-4）、AD-15 权限与锚定（H-5）、AD-6 URL 失效（H-8，需回推 PRD）、AD-10 执行手段（H-10）、AD-19 搜索（H-7）。
4. **ADR-0005 降为 Proposed**，补端上 spike（H-9）。
5. 在 Deferred 中显式列出 FR-58/59/66 等，消除 §2.1 的空白；Stack 表回到 MySQL 8.0 约束（M-1）。
