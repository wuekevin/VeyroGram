---
title: VeyroGram Open Platform & Mini Apps
status: draft
created: 2026-11-19
updated: 2026-11-19
parent: docs/product/PRD.md
---

# PRD: VeyroGram 开放平台与小程序生态
*父文档：PRD.md（FR-48…53、63、71）。本文把它们展开为可验收的需求；ID 为 `OP-FR-n`，并回填映射。词汇沿用 PRD §3 Glossary，新增词在 §3 登记。*

## 0. Document Purpose
面向产品、架构、安全、开放平台工程与审核运营。范围：开发者注册→应用创建→授权→API/Webhook→机器人→小程序运行时→审核发布→市场→运营与投诉。支付能力、开发者结算依赖 PRD OQ-3/OQ-12，本文只定边界。技术方案见 `docs/architecture/ARCHITECTURE.md`（AD-11、AD-12）与 ADR-0005；**AD-11 在 iOS/Android WebView 上的可行性尚未验证，本文所有“独立 origin / 分区存储”要求以 Phase 5 spike 通过为前提**。

## 1. Vision
让第三方开发者在 VeyroGram 内可注册、可开发、可提交、可审核、可发布、可运营，而不牺牲用户的隐私与平台的安全。小程序不是 iframe 跳转页：它运行在受限容器中，只能通过带白名单与用户授权的 Bridge 使用宿主能力；开放 API 与 Webhook 与 IM 主站互通，但在服务端坚持隔离与授权边界。

## 2. Target User
### 2.1 Jobs To Be Done
- 开发者：快速接入登录授权与消息能力；清楚知道审核标准与进度；出问题能灰度回滚；看到调用量与错误率。
- 企业主体：用团队与角色管理多人协作，密钥可轮换、可审计。
- 终端用户：用得放心——清楚小程序要什么权限，能随时撤销，被举报的应用能被快速下架。
- 审核/运营：高效审核、风险扫描辅助、处理投诉，所有处置可追溯。
### 2.2 Non-Users (v1)
匿名未实名的商业支付开发者；需要原生代码插件（非 Web 技术栈）接入宿主的开发者（v1 只提供受限 WebView 运行时）。
### 2.3 Key User Journeys
- **OUJ-1. 艾琳上线小程序**（同 PRD UJ-5）：创建应用→绑定域名→提交→自动扫描→人工审核→10% 灰度→全量→用户撤权验证。
- **OUJ-2. 老魏的机器人只能进被邀请的群**：开发者创建机器人，群主邀请进群，机器人仅在该群收发；被移出后立即失效。
- **OUJ-3. 小米用户撤销授权**：用户在“我→已授权应用”撤销，应用下一次调用 401，已缓存数据按声明的保留期清理。
- **OUJ-4. 审核员发现恶意 Bridge 调用**：自动扫描标记→人工复核→下架→已授权用户收到通知→开发者收到整改说明与申诉入口。
- **OUJ-5. 开发者排查 Webhook 失败**：控制台看到签名失败/超时/死信，手动重放。

## 3. Glossary（新增）
- **主体 (Subject)** — 开发者的认证实体，个人或企业。
- **AppID / App Secret** — 应用公开标识 / 机密凭据；Secret 仅创建时展示一次。
- **Scope** — 最小授权单元，如 `profile.basic`。
- **授权 (Grant)** — 用户对某应用某组 scope 的同意记录，可撤销。
- **包 (Package)** — 小程序的签名发布物，含 manifest。
- **Manifest** — 声明域名、能力、版本、权限用途的清单。
- **能力 (Capability)** — Bridge 暴露的宿主功能，分风险级别。
- **灰度 (Rollout)** — 按比例向用户发布某版本。
- **服务账号 (Service Account)** — 以应用身份发送服务消息的非人类账号。

## 4. Features

### 4.1 开发者账号与主体认证
**Description:** 个人或企业开发者注册、完成主体认证（流程按法域与 OQ-12 定）、创建团队并分配角色。
#### OP-FR-1: 注册与主体认证（→ FR-48）
开发者可注册并提交主体认证材料；未认证主体只能创建沙箱应用。
**Consequences (testable):**
- 未认证主体无法提交上架审核、无法申请需实名的 scope。
- 认证材料加密存储、访问需理由并留审计、可按保留策略删除。
#### OP-FR-2: 团队与角色（→ FR-71）
主体可邀请成员，角色为所有者/管理员/开发者/只读；所有者转让需二次验证。
**Consequences (testable):**
- 角色权限由服务端强制；只读成员调用写接口返回 403；变更写审计。

### 4.2 应用与密钥生命周期
#### OP-FR-3: 应用创建与类型（→ FR-48）
开发者可创建应用，类型为机器人、H5 应用、小程序、服务账号、企业内部应用；分配 AppID。
**Consequences (testable):** 应用状态机 `draft → sandbox → in_review → approved → published → suspended → retired`，非法迁移被拒绝。
#### OP-FR-4: 域名与包名绑定
开发者须验证域名所有权（DNS/文件校验）后才能在 manifest 声明该域名；移动应用包名/签名指纹绑定到应用。
**Consequences (testable):** 未验证域名的请求在容器内被拦截；验证失效（域名转手）自动暂停相关版本。
#### OP-FR-5: 密钥生命周期（→ FR-48）
Secret 创建时一次性展示；支持轮换（新旧并存宽限期）与立即吊销；仅存哈希。
**Consequences (testable):** 数据库与日志中不出现明文 Secret；吊销后 ≤ 60 秒内相关令牌失效 [ASSUMPTION]。

### 4.3 授权（OAuth 2.1 风格）
#### OP-FR-6: 授权码 + PKCE（→ FR-49）
仅支持 Authorization Code + PKCE（公开客户端必须），不支持隐式/密码授权；重定向 URI 精确匹配。
**Consequences (testable):** 授权码一次性、≤ 5 分钟有效 [ASSUMPTION]；重放与 `state` 缺失被拒绝；开放重定向用例被拦截。
#### OP-FR-7: 同意页与最小 scope
授权界面由宿主绘制，列出 scope 的用户可读说明与数据用途；用户可部分同意。
**Consequences (testable):** 小程序无法伪造或遮挡同意页；未获同意的 scope 调用返回 403。
#### OP-FR-8: 撤销与令牌管理（→ FR-49）
用户可在“已授权应用”撤销；开发者可吊销令牌；访问令牌短期，刷新令牌轮换并检测复用。
**Consequences (testable):** 撤销后 ≤ 60 秒内访问失败 [ASSUMPTION]；刷新令牌复用触发整条链吊销并告警。

#### Scope 目录（草案，Phase 5 前评审）
| Scope | 说明 | 风险 |
|---|---|---|
| `openid` | 稳定、应用内唯一的用户标识（不泄露内部 ID） | 低 |
| `profile.basic` | 昵称、头像 | 低 |
| `profile.phone` | 手机号（需额外同意与资质） | 高 |
| `contacts.match` | 通讯录匹配结果（哈希，不含原文） | 高 |
| `chat.share` | 向用户选择的会话分享内容 | 中 |
| `chat.bot` | 机器人在被邀请会话收发消息 | 中 |
| `notify.subscribe` | 用户同意的订阅消息 | 中 |
| `storage.files` | 读取用户选择的文件 | 中 |
| `location.once` | 单次位置 | 高 |

### 4.4 开放 API、SDK 与 Webhook
#### OP-FR-9: 开放 API（→ FR-52、FR-53）
`/open/v1`（独立监听、独立限流与审计，AD-12）；OpenAPI 3.1；速率限制、幂等键、调用统计。
**Consequences (testable):** 超限 429 + `Retry-After`；错误响应含 `trace_id`；个人用户 API 令牌不可调用开放 API。
#### OP-FR-10: SDK（→ FR-71）
提供 TypeScript、Go、Android、iOS SDK，由 OpenAPI 与 Bridge 契约生成并版本化。
**Consequences (testable):** SDK 随契约版本发布并带兼容窗口；契约测试在 CI 中覆盖 SDK 示例。
#### OP-FR-11: Webhook（→ FR-52）
签名（`HMAC-SHA256(timestamp.body)`）、时间戳窗口、至少一次投递、指数退避、死信与手动重放。
**Consequences (testable):** 过期或签名错误请求被拒；同一事件重复投递携带相同事件 ID；死信可在控制台重放。
#### OP-FR-12: 沙箱与调试（→ FR-71）
沙箱环境与测试账号与生产严格隔离；调试日志可查看最近请求（脱敏）。
**Consequences (testable):** 沙箱令牌无法访问生产数据；调试日志不含令牌与完整手机号。

### 4.5 机器人与服务账号
#### OP-FR-13: 机器人（→ FR-53）
机器人需被用户/群主邀请进会话才能收发；命令菜单可声明；速率受限。
**Consequences (testable):** 未被邀请的会话调用返回 403；被移出后立即失效；机器人不得直接调用 IM 内核（AD-2）。
#### OP-FR-14: 服务账号与订阅消息
服务账号可向已订阅用户发送模板化服务消息；用户可随时退订，频率受限。
**Consequences (testable):** 未订阅用户收不到；退订立即生效；模板需审核。

### 4.6 小程序运行时与 Bridge（→ FR-50）
**Description:** 第一阶段采用受限 WebView + Native JS Bridge（ADR-0005）。宿主绘制顶栏与所有授权弹窗；每个小程序独立 origin 与分区存储（待 spike 验证，见 §0）。
#### OP-FR-15: 容器安全基线
签名包校验；CSP；网络与导航域名白名单；禁止外部页面获取特权；资源与内存上限；每应用独立存储/Cookie。
**Consequences (testable):** 访问未声明域名被拦截并上报；一个小程序读不到另一个小程序的存储；包签名不符拒绝加载。
#### OP-FR-16: Bridge 协议
所有 Bridge 调用携带 nonce 与签名（标准库 HMAC/JWS，不自研密码学），经 allowlist 与用户授权校验；提供 `bridge.version` 与 `has(capability)` 能力探测；未知调用 fail-closed。
**Consequences (testable):** 重放同一 nonce 被拒；未声明能力调用被拒；旧容器遇到新能力返回“不支持”而非崩溃。

#### Bridge 能力目录（草案）
| 能力 | 授权方式 | 风险 | 备注 |
|---|---|---|---|
| 登录授权 `auth.login` | 同意页 | 低 | 返回授权码，不返回凭据 |
| 基础资料 `user.profile` | 同意页 | 低 | 仅 `profile.basic` |
| 分享到会话 `chat.share` | 用户选择会话弹窗 | 中 | 内容预览后发送 |
| 扫一扫 `scan` | 系统相机权限+说明 | 中 | 结果经用户确认 |
| 文件选取 `file.pick` | 系统选择器 | 中 | 只返回所选文件 |
| 位置 `location.once` | 弹窗+系统权限 | 高 | 单次，不后台 |
| 剪贴板 `clipboard.read` | 每次显式确认 | 高 | 默认禁用 |
| 订阅消息 `notify.subscribe` | 弹窗 | 中 | 可退订 |
| 主题 `ui.theme` | 无需 | 低 | 明/暗 |
| 支付 `pay` | 合规支付渠道落地后 | 高 | 本期不开放（OQ-3） |

#### OP-FR-17: 运行时兼容与降级
iOS、Android、Web、桌面的能力差异通过能力探测暴露；不支持的能力降级或隐藏。
**Consequences (testable):** 同一示例小程序在四端运行；不支持的能力不会静默失败。

### 4.7 审核、发布与回滚（→ FR-51）
#### OP-FR-18: 自动风险扫描
提交的包经静态扫描（域名、危险 API、敏感能力使用、依赖许可证）与恶意样本检测。
**Consequences (testable):** 恶意测试集（XSS、越权 Bridge、SSRF、剪贴板窃取）全部被标记；扫描报告随审核可见。
#### OP-FR-19: 人工审核与申诉
审核员依据清单审核（权限用途说明、隐私政策、内容合规）；开发者可申诉，申诉由不同审核员处理。
**Consequences (testable):** 每次审核结论与理由写审计；驳回给出可操作的整改项。
#### OP-FR-20: 灰度、发布与回滚
按比例灰度；监控错误率自动暂停；一键回滚到上一版本；紧急下架。
**Consequences (testable):** 回滚后新开启的小程序使用上一版本 [ASSUMPTION：≤ 5 分钟生效]；下架后已授权用户收到通知，运行中的实例在下次启动不可用。

### 4.8 市场与发现（→ FR-63）
#### OP-FR-21: 浏览、搜索与详情
分类、搜索、详情页（权限用途、版本、评价）、使用历史与收藏。
**Consequences (testable):** 下架/暂停的应用不出现在任何列表与搜索；详情页显示其申请的权限与用途。
#### OP-FR-22: 评价与举报
用户可评价与举报；评价限频、防刷；举报进入审核队列。
**Consequences (testable):** 同一用户对同一应用只保留一条有效评价；举报量达阈值触发复审 [ASSUMPTION]。

### 4.9 开发者控制台与运营
#### OP-FR-23: 控制台
应用清单、配置、密钥（一次性展示）、授权设置、API 调试、版本与审核状态、统计、团队管理。
**Consequences (testable):** 每页具备加载/空/错误状态；密钥展示块支持复制且离开页面后不可再看。
#### OP-FR-24: 统计与计量
调用量、错误率、延迟、活跃用户（聚合，不含个人数据）、Webhook 成功率。
**Consequences (testable):** 统计口径文档化；个人级数据不出现在聚合报表。
#### OP-FR-25: 投诉与处置
用户/第三方投诉流程；处置包含警告、限权、下架、封禁主体，均可申诉并留痕。
**Consequences (testable):** 处置动作写审计；封禁主体使其所有应用暂停。

### 4.10 开发者结算（条件项）
#### OP-FR-26: 结算与分成
**仅在 OQ-3/OQ-12 决策并获法务批准后实现**：计费、对账、发票/税务对接。
**Consequences (testable):** 未决策前不暴露任何收费入口；决策后按 PRD FR-55 的幂等与对账要求实施。

## 5. Non-Goals
- 不开放原生插件或原生代码注入宿主。
- 不允许小程序读取其他应用数据、会话列表或通讯录原文。
- 不在支付渠道与法务未落地前开放支付能力。
- 不承诺“绝对安全”；安全性以审核、扫描、隔离与测试证据为准。
- 不复制微信/支付宝或官方闭源实现，只参考公开产品形态。

## 6. MVP / Scope
- 首发（Phase 5）：OP-FR-1…25（含 OP-FR-15 的 spike 前置）；机器人 + H5 + 小程序三类；市场基础版。
- 延后：OP-FR-26（结算）、支付 Bridge、企业内部应用高级管理、原生运行时。

## 7. Success Metrics
- **OSM-1**：示例小程序完成“提交→扫描→审核→灰度→全量→撤权”闭环，在 iOS/Android/Web/桌面四端通过（验收，非占比指标）。
- **OSM-2**：恶意小程序测试集拦截率 100%（上线硬门槛）。
- **OSM-3**：撤权/吊销生效时间 ≤ 60 秒（P95）[ASSUMPTION]。
- **OSM-4**：Webhook 重放/篡改用例 100% 拒绝。
- **Counter-metrics**：**OSM-C1** 不为提高上架通过率而放宽扫描规则；**OSM-C2** 不为提升活跃而增加默认授权范围。

## 8. Cross-Cutting Requirements
- 安全：威胁 T-10/T-11/T-12（SECURITY-THREAT-MODEL）；审计留痕；密钥与令牌脱敏；SSRF 防护。
- 合规：开发者协议、隐私政策、未成年人与内容规则（OQ-4）；iOS App Review 与 Android 平台规则审查（OQ-5）。
- 性能：开放 API 与 Web 站点预算在 Phase 5 压测中确定。
- 可观测：开放 API/小程序错误率与告警（NFR-7）。

## 9. Mapping 到主 PRD
| 本文 | 主 PRD |
|---|---|
| OP-FR-1,3,5 | FR-48 |
| OP-FR-6,7,8 | FR-49 |
| OP-FR-15,16,17 | FR-50 |
| OP-FR-18,19,20 | FR-51 |
| OP-FR-9,11 | FR-52 |
| OP-FR-13,14 | FR-53 |
| OP-FR-21,22 | FR-63 |
| OP-FR-2,10,12 | FR-71 |
| OP-FR-26 | FR-54/55（条件） |

## 10. Open Questions
1. **OQ-OP-1**：iOS WKWebView 与 Android WebView 上“每应用独立 origin + 分区存储”的可行实现（Phase 5 spike）。
2. **OQ-OP-2**：开发者实名认证的法域要求与个人开发者的权限边界（PRD OQ-4/12）。
3. **OQ-OP-3**：支付与结算（PRD OQ-3）。
4. **OQ-OP-4**：应用商店规则对小程序的约束（尤其 iOS 的动态代码下载条款，须审查 App Review 指南）。
5. **OQ-OP-5**：Scope 与 Bridge 目录最终清单与风险分级评审。
6. **OQ-OP-6**：小程序包托管与分发（对象存储 + CDN）与签名密钥管理方案。

## 11. Assumptions Index
- OP-FR-5/8/20：60 秒、5 分钟等时延目标；OP-FR-6：授权码 5 分钟有效；OP-FR-22：举报阈值；均为待测目标。
- Scope/Bridge 目录为草案，非最终。
