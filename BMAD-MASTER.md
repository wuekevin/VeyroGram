# VeyroGram — BMAD 商业级全栈二次开发总指令

> 版本：1.0｜2026-11-19｜项目类型：全新品牌的独立即时通讯产品｜执行者：BMAD 多 Agent 编排
>
> **立即执行方式**：将本文件完整输入已安装 BMAD 的 Claude Code / Codex 工作区；优先调用当前安装的 `/bmad` 工作流，先识别实际可用命令与 Agent 列表，不可假定固定子命令。执行前将附件 `logo.png` 放入项目 `brand/source/logo.png`。

## 0. 项目不可变约束

1. 产品及所有客户端、管理后台、开发者平台、官网、文档的品牌统一为 **VeyroGram**，本项目基于唐僧叨叨（TangSengDaoDao）和悟空 IM 二开，与此前基于 SereinGram/Telegram 的另一个项目**完全隔离**；不得复用后者的仓库、配置、API ID、Telegram 凭据、URL Scheme 或技术假设。品牌重名将引发包名、域名、应用商店和商标冲突，发布前必须完成核验；若既有产品与本项目品牌冲突，必须提交风险清单，不得擅自混用两套代码。
2. 目标为可运营、可维护、可审计、可扩展、可持续交付的**全端商业级 IM 平台**，不是改 Logo / 改色或基础 Demo。保留上游全部经验证的基础功能与兼容性；每一端每一功能认真重构、修复、完善或替换，并用测试证据证明。
3. 五大源代码：Server、Manager、Android、iOS、Web（含现有 Electron PC），另有 Docs 作为规范和证据基线。新建官网、开放平台及必要基础服务；全部界面和图标重新设计；不得把上游 UI 直接换主题后冒充重设计。
4. 所有关键业务能力须服务端授权与鉴权，不允许仅通过客户端 UI 控制权限。所有跨端能力必须有统一协议、状态机、权限、测试及优雅降级策略。
5. 用户偏好：Go 后端、MySQL 8.0、Redis 8、Nginx、宝塔管理 Linux、原生 systemd，**正式部署禁止 Docker**。保留并单独维护开发测试环境，绝不将演示环境直接暴露线上。
6. 官方收费模块是**功能需求基线**，不是授权使用其闭源代码。未取得源码和合法使用权时，只能基于公开功能描述独立实现，不拷贝、反编译、绕过授权。逐仓库建立许可证、NOTICE、字体/图标/SDK/素材来源及商标清单；保留法定许可与署名义务。
7. 不准用 Emoji 做任何导航菜单、工具栏、底部 Tab 或后台菜单图标；设计原创一致的 SVG 图标系统。聊天内容中的正常 Emoji、表情商店属于通讯功能，允许。
8. 未经审计不得宣称 E2EE、绝对安全、无限群成员、全端功能一致或性能指标已经达标。
9. 开发每个阶段严格遵守：**发现证据 → 设计 → 实现 → 本地测试 → 静态/安全审计 → 多端集成 → 验收**；减少在公网及线上进行重复试错，生产不可用于开发验证。

## 1. 资料输入及调研强制门禁

### 上游源代码（全部克隆、锁定 commit SHA 并审计）
- https://github.com/TangSengDaoDao/TangSengDaoDaoServer
- https://github.com/TangSengDaoDao/TangSengDaoDaoManager
- https://github.com/TangSengDaoDao/TangSengDaoDaoAndroid
- https://github.com/TangSengDaoDao/TangSengDaoDaoiOS
- https://github.com/TangSengDaoDao/TangSengDaoDaoWeb
- https://github.com/TangSengDaoDao/TangSengDaoDaoDocs
- 通讯层：https://github.com/WuKongIM/WuKongIM（版本/兼容性另行锁定）

### 官方文档入口（递归读取导航栏/站内链接与 Docs 源文件）
- https://tsdaodao.com/dev/backend/deploy-compose.html
- https://tsdaodao.com/dev/manager/build.html
- https://tsdaodao.com/dev/android/intro.html
- https://tsdaodao.com/dev/ios/intro.html
- https://tsdaodao.com/dev/web/intro.html
- https://tsdaodao.com/price/detail.html
- https://tsdaodao.com/price/index.html
- 后端 API 文档入口：https://apidocs.botgate.cn/

**必须执行的工作**：递归梳理后端安装/配置/架构/API/模块开发/部署，后台的全部用户手册，Android/iOS/Web 的编译/配置/项目结构/模块引入/常见问题，以及收费模块每一细项。使用 `TangSengDaoDaoDocs` 仓库交叉核对；生成 `docs/audit/source-index.md`，记录每页 URL、标题、更新时间（若可得）、对应源码路径、阅读状态。无法访问的页标 `UNVERIFIED` 并记录原因；不可声称已阅读。检查 release、Issue、PR、依赖树、社区问题，但不得把未知风险写成已验证缺陷。

### 已确认上游架构事实（需要在实际 commit 再确认）
- WuKongIM 承担连接、消息投递与存储，TangSengDaoDaoServer 承担好友、群组等业务；以 API、Webhook/gRPC 衔接。
- 官方后台文档为 Vue 3 + TypeScript + Vite + Pinia + Element Plus；Web 仓库 README 涉及 React/TypeScript、Electron 桌面构建，须实地确认版本。
- Android 采用 `wkbase`、`wkuikit`、`wklogin`、`wkpush`、`wkscan` 等模块；iOS 使用 CocoaPods 模块化结构；Web 存在 modular npm packages。
- 官方的“宝塔部署”仍为 Compose 流程；新项目**另行建立原生无 Docker 部署方案**。

### 产物
`source-index.md`、`upstream-baseline.md`、`license-and-ip.md`、`dependency-sbom.md`、`architecture-as-is.md`、`endpoints-inventory.yaml`、`feature-matrix.csv`、`risk-register.md`、`adr/`，每条功能标识 `[DOCUMENTED|FOUND_IN_SOURCE|TESTED|MISSING|BROKEN|LICENSE_BLOCKED]`，必须引用证据路径和 commit。

## 2. BMAD 多 Agent 编排与自动模型选择

**执行引擎必须首先探测当前 CLI 实际支持的模型、模型别名、每个 Agent 的模型覆写机制及可用工具。不要杜撰模型 ID，不要声称所有 BMAD Agent 原生具备动态切换模型。** 如果系统只支持会话级模型切换，则通过编排器启动不同模型的独立子任务/会话并汇总；若不支持，保留当前模型，明确写入调度日志并继续。

### 模型选择策略（能力分层，不绑定假想型号）
- `reasoning-high`：复杂架构、威胁建模、跨端协议、数据库迁移、并发一致性、关键安全评审。
- `coding-high`：高风险业务代码、SDK、IM 协议、Electron 安全、原生客户端、复杂重构。
- `design-vision`：视觉设计、设计系统、图标、截图对照、UI 审核；必须真的能看图。
- `fast-efficient`：资料抓取、索引、单测、文档、常规样板代码、机械修改。
- `independent-review`：关键阶段用与实现 Agent 不同的可用模型/独立上下文交叉审核。

### 可执行模型路由规则
1. 每个 Agent 在任务开始写出 `task_kind`、复杂度、风险级别、输入规模、需不需要视觉/代码执行及推荐模型能力档；由调度器解析**当前可用**模型名单并选型。
2. 高风险任务不得为了省 Token 降级到不能胜任的模型；简单任务不必始终选最高配。
3. 当工具允许，允许任务中途升级：初步探索用快速模型，遇到跨端协议冲突或安全问题升级推理模型，执行结束交叉复核。
4. 记账：`docs/agent/model-routing-log.jsonl` 记录 Agent、任务 ID、可用模型、选中模型、原因、成本、重试、结果及 fallback；不得记录密钥与隐私数据。
5. 每个 Agent 必须输出证据、决策、改动文件、测试命令/结果、风险和下一步；不得仅返回“完成”。

### Agent 分工
- Orchestrator / Scrum Master：依赖、里程碑、质量门禁、冲突管理。
- Analyst / Product Manager：产品范围、上游功能盘点、商业功能 PRD、验收标准。
- Architect / Protocol Engineer：WuKongIM 集成、API/OpenAPI、事件模型、可扩展架构。
- Security / Privacy Engineer：鉴权、加密、滥用防护、威胁建模、审计。
- DB / Backend Engineer：Go/MySQL/Redis/文件/任务队列/支付/运营平台。
- Android Engineer、iOS Engineer、Web/Desktop Engineer：端侧完整开发与测试。
- UX Researcher / UI Designer / Brand Designer：每端全新界面、体验、图标与 Logo 规范。
- Open Platform Engineer：小程序运行时、沙箱、OAuth、SDK、审核发布。
- QA / SDET / Performance Engineer：自动化测试、压测、可用性和兼容验收。
- DevOps / SRE：原生部署、监控、恢复、CI/CD、灰度。
- Tech Writer / Release Manager：完整文档、版本发布、支持手册。

**BMAD 命令规则**：`/bmad` 为工作流入口。Agent 先读取当前安装的 BMAD 方法论/工作流定义，查明实际可用的 PRD、architecture、epic/story、implementation、review 等命令再调用；若某命令不存在，调用等效已安装流程，不伪造执行结果。先产出设计与 Backlog 再并行实施。

## 3. 产品与架构目标

- 形态：移动端 Android/iOS、浏览器 Web、Windows/macOS/Linux 桌面（以 Web 现有 Electron 项目评估为基础）、商业运营后台、开发者开放平台、审核后台、产品官网及开发者文档。
- 服务：Go API/BFF、身份与授权、社交关系、群组、消息业务适配、社交动态、搜索、对象存储、文件处理、客服、通知、支付/订阅、开放平台、内容审核与风控、观测平台。
- 通讯内核：优先保留可维护的 WuKongIM 协议/SDK 并锁版本，不能随意更换导致跨端行为退化；所有扩展消息通过版本化 schema/统一内容类型注册表实现。
- 数据：MySQL 8.0（业务主库）、Redis 8（会话/限流/缓存/临时状态），兼容 S3 的对象存储（可接 OSS/COS/S3/自建实现），IM 数据存储按 WuKongIM 实际内核要求单独设计。先验证兼容，再考虑引入额外组件。
- 系统分层：API Gateway/Nginx → Auth/API → Domain Services → MySQL/Redis/ObjectStore/WuKongIM；异步任务可优先 DB outbox + worker，性能与规模证明有必要后再引入消息中间件。
- 所有外部接口：OpenAPI 3.1、SDK 版本化、错误码、速率限制、幂等、追踪 ID、分页、游标、API/事件兼容策略。
- 数据库：表名/字段规范、迁移版本、索引、审计字段、幂等键、软删除/过期策略、备份恢复、敏感数据加密与保留策略。
- 搜索：起步 MySQL/IM 内建能力，经性能与权限验证再引入搜索引擎；搜索权限必须与消息读取权限一致。
- 多组织/多租户：作为**可配置商业扩展**，先完成单部署业务边界与 tenant-aware 设计，再根据隔离与性能测试决定共享库或物理隔离，禁止半成品“假多租户”。

## 4. 官方全部付费模块：逐条实现与跨端矩阵

以下列为全部必做 Story，不允许只做 UI、只做 Android 或仅在服务端有接口。为每一项建立 Server、Android、iOS、Web、Desktop、Manager、API、DB、通知、权限、E2E 十列矩阵，未适用要有正式说明和替代交互。

1. **基础功能完整保留**：注册登录、个人资料、联系人/好友申请/通讯录匹配、群聊、文本/图片/语音/名片、输入状态、系统消息、扫码/二维码、会话/置顶/免打扰、群公告、历史记录、搜索、多语言、主题、设备管理等。逐项测试，不以文档代替代码证据。
2. **收藏**：文字/图片收藏，列表、删除、复制、转发、权限、同步；可扩展文件/视频/链接收藏。
3. **位置**：地图选点、当前位置、地点搜索、坐标卡片/缩略图/跳转，地图 Provider 可配置、位置权限最小化；不默认后台定位。
4. **文件**：上传/下载/断点续传（必要时）、类型/大小/扩展名验证、病毒扫描/危险文件隔离、签名 URL、预览、分享/转发、保留和配额策略。
5. **联系人标签**：标签 CRUD、联系人批量整理、隐私隔离、跨设备同步、批量行为限制。
6. **高级群管理**：群主/管理员/成员角色、邀请审核、群头像、全员/个人禁言、群黑名单、成员互加限制、历史消息可见性、撤回与审计、超大群权限/性能验证。
7. **小视频**：录制/相册、15 秒上游基线（可配置限额）、大小显示、转码/封面、上传进度、播放、保存和转发；音视频编解码及版权合规。
8. **安全与隐私**：手机号/短号可搜索性、密码重设、设备锁与设备列表、会话撤销、账号注销、黑名单、敏感配置/密钥保护、数据导出/删除机制；生命周期清晰可测试。
9. **表情商店**：系统 Emoji、个人收藏/制作、动图表情、自定义表情包上传、审核、排序/删除/分享、跨端兼容、版权与内容处理。**菜单图标不能使用 Emoji，不妨碍聊天表情。**
10. **朋友圈/动态**：文本/图/视频发布、位置、提醒好友、9 图排序、评论/点赞、分组与排除列表的可见性、动态列表、消息提醒、删除、违规举报和后台审核；关注越权与删除后缓存。
11. **客服**：官方详细页标注“开发中”，必须从零完成完整商业闭环：访客入口、技能组/队列、自动/手动分配、转接、会话 SLA、工单、快捷回复、知识库、客服绩效、用户标签、质检、审计、工作时间。不能认为上游已交付。
12. **旗舰消息能力**：已读/未读及成员详情、消息编辑及版本策略、消息点赞、在线状态/最近在线、图片编辑、历史按日期/成员/类型/关键词查找、截屏通知（仅操作系统实际支持；不可保证全面检测）、撤回提示；明确隐私配置与可用性限制。
13. **音视频**：一对一音视频、群聊/会议、呼叫/接听/挂断/忙线/超时、系统电话集成（按系统权限）、前后台切换、断网重连、弱网自适应、TURN/STUN、SFU/媒体服务选型、设备权限、录制需明确同意。以可测试的 WebRTC/RTP 方案落地；不能仅实现拨号页。

**强制对照**：https://tsdaodao.com/price/detail.html 的全部三级功能逐行导入需求矩阵；页面列表是最低基线，遗漏任何一行即不能关闭对应 Epic。供应商功能以页面与实际演示为参考，不允许复制闭源实现。

## 5. 超越付费模块：新增商业级产品功能

### P0 必须建设
- 多设备登录与可信设备、异地登录提醒、设备撤销、登录通知、可选 MFA/Passkeys、会话过期策略。
- 消息幂等/去重、顺序/游标、ACK/回执、失败重试、离线同步、草稿跨设备、编辑/撤回一致性、引用回复、转发/合并转发、删除范围、消息搜索与媒体中心。
- 用户/群组/消息举报、恶意链接/文件检测、反垃圾与限速、账户/设备/IP 风险控制、合规的用户申诉流程。
- 推送通道：APNs、FCM 和适用 Android 厂商通道；前后台/静默/免打扰/多设备冲突消解；通知中不泄露敏感消息正文。
- 对象存储生命周期、文件清理、缩略图/预览、头像/图片变换、资源水印（按需）、附件访问控制。
- 稳健的管理后台：全面 RBAC/最小权限、操作审批、审计日志、运营配置、用户/设备/群/消息索引、风控举报、内容审核、客服工单、统计与监控。
- 官网、开发者文档、隐私政策/条款、后台可配置应用下载链接和版本更新。

### P1 商业增长
- VIP/订阅及 Feature Entitlement（权益服务），配额/功能开关、优惠券、订单、退款、发票/税务对接、套餐变更与对账；支付采用合规第三方，服务端签名验证与 webhook 幂等，不自行保管银行卡敏感信息。
- 企业/组织通讯录、部门、角色、公告、企业空间、组织安全策略（经租户设计验收）。
- 频道/广播/公告号、可配置机器人、开放 API、Webhook、消息自动化、工作流。
- 运营消息模板、活动、推送任务、通知中心、分群触达与退订、数据统计。
- AI 辅助翻译/总结/客服建议（明确用户同意、敏感数据最小化、成本与速率限制、模型无关网关），先设计隐私策略，不默认发送私聊数据给第三方模型。

### P2 扩展（架构预留，PRD 审核后排期）
- 实时协作/群任务、会议纪要、云盘空间、内容频道、企业 SSO/SAML/OIDC、合规数据驻留/导出、可选 E2EE 协议评估（独立协议审查，避免自研密码学）。

## 6. 微信/支付宝式开放平台与小程序生态（重点独立产品）

**不是简单 iframe 跳转页。** 提供真正开发者可注册、开发、提交、审核、发布和运营的 App/Mini App 生态；完整设计桌面 Web 与移动端安全承载能力。

### 开放平台产品模块
- 开发者账号/实名认证流程（按法律及业务需要）、个人/企业主体、团队与角色、开发者控制台、应用创建、AppID、应用密钥生命周期、包名/域名绑定。
- OAuth 2.1 风格授权（Authorization Code + PKCE、OIDC 如需要）、用户同意、最小 scope、撤销授权、回调白名单、反重放/CSRF/SSRF。
- 开放 REST API / OpenAPI、TypeScript/Go/Android/iOS SDK、Webhook（签名+时间戳+重放防护+重试死信）、速率限制和调用统计。
- 应用类型：机器人、H5 应用、轻应用/小程序、服务账号、企业内部应用。开发平台支持沙箱/测试账号、调试日志、版本管理、审核、灰度、上/下架、投诉处理。
- Mini App 运行时：优先选择**经过安全设计的受限 WebView 容器 + Native JS Bridge** 作为第一阶段；Bridge 明确 API allowlist、版本/能力探测、权限弹窗、origin 校验、消息签名/Nonce、导航白名单、CSP、资源限制、网络与存储隔离、文件/摄像头/定位显式授权。不可把外部 HTML 直接提升为系统特权代码；iOS App Review / Android 平台规则需审查。
- Mini App 能力：登录授权、用户基础资料授权、分享聊天/群、扫一扫、文件选取、支付能力（仅在合规支付渠道落地后）、订阅消息、位置、剪贴板（显式授权）、UI 主题、SDK 生命周期。
- Mini App 市场：发现、分类、搜索、详情、评价、推荐、使用历史/收藏、风险扫描、举报、运营审核、开发者结算（若商业模型批准）。
- 开放平台的身份、订单、消息、权限和审计与 IM 主站互通，但必须坚持服务端隔离和授权边界。
- **小程序审核、签名包、发布回滚、安全隔离、跨端兼容、权限撤销、恶意小程序测试**属于上线硬门槛。

## 7. 完整重设计：各端 UI/UX + 品牌图标

唯一视觉源：用户上传的 `logo.png`，在本地存为 `brand/source/logo.png`；蓝/青/紫渐变 + 深色底的 VG 星际聊天气泡图形为品牌识别基础。所有图标和衍生资产需保持主形状和识别度，避免裁切/模糊/透明背景处理不当。先输出 `brand-guidelines.md`、矢量重绘/授权来源、调色板、字体、间距、形状、响应式规则，再导出多平台图标。没有源矢量时允许手工重绘，不可把 PNG 自动放大后冒充高质量 SVG。

- Android：Adaptive Icon 前景/背景/单色、通知小图标、启动页、应用内品牌图、Play Store 512×512。严格适配 Android 版本/状态栏/深色模式/动态字体/手势与触控。
- iOS：AppIcon 多尺寸、Launch Screen、深浅色、Apple HIG、Dynamic Type、VoiceOver、隐私权限说明，审核应用图标可读性。
- Web/PWA：favicon SVG/ICO、192/512 图标、maskable、OG/Twitter 图片、浏览器不同 DPR。
- Windows/macOS/Linux Electron：ICO 多分辨率、ICNS、PNG、多系统托盘图标、通知图标、安装程序、签名与更新渠道。
- Manager/Developer/Official Website：SVG logo 横版/竖版/单图标、品牌背景资产、落地页 OG、文档标记。
- 提供原创功能图标库（24px 基准，统一网格、stroke/fill、状态集、无障碍描述），**导航和后台侧边栏严禁 Emoji 代替图标**。

### UI 全面交付范围
- 统一设计系统：色彩 tokens、语义色、明/暗主题、响应式、布局、排版、图标库、阴影、动画、表单/弹层/列表/表格/空状态/错误状态/骨架屏/Toast/权限弹窗。
- Android/iOS：登录注册、聊天列表/对话/消息菜单/文件/音视频/朋友圈/收藏/表情/个人中心/设备/隐私/小程序/支付与订阅、完整设置，全部状态与异常流程。
- Web/Desktop：三栏响应式 IM、大屏快捷键、窗口拖拽/托盘、文件拖拽、通知、离线/恢复/更新、权限告知；不是手机页放大版。
- Manager：全新企业 SaaS 风格，概览、风控、成员/群/消息、媒体、活动、支付/订阅、客服、权限、审计、开发者/小程序审核、数据报表、系统配置等，每页必须有真实数据动作与 Empty/Loading/Error 状态。
- 开放平台：开发者落地页、应用清单、应用配置、App Secret 安全展示、授权设置、API 文档/调试、版本审查/发布、统计、组织管理。
- 官网：产品价值、跨端下载、功能详情、安全隐私、版本公告、FAQ、定价/权益（如启用）、开发者入口、服务状态/帮助、品牌/法务页面、SEO/性能/无障碍/国际化。
- UI 审核：先高保真组件/流程原型，设计系统审批后编码；截图基线 + 视觉回归；禁止保留上游页面样式、拼凑第三方模版或仅替换 Logo。

## 8. 安全、隐私与可靠性阻断项

- 建立 STRIDE 威胁模型、资产/信任边界、数据流和 abuse-case；重点：登录/短信、设备绑定、WebSocket token、WuKongIM API 与 webhook/gRPC 信任、消息越权、文件访问、管理后台、机器人 API、Mini App Bridge 与支付。
- 不可使用固定 SMS 验证码、通用管理员密码、演示账号、开放数据库 root、无鉴权监控页面/管理端口。官方部署示例中的 `TS_SMSCODE=123456` 与 `TS_ADMINPWD=admin1234567` **必须作为风险审计项**，生产禁止默认值及便捷后门。
- 密码：成熟 KDF（Argon2id 或合规等价）、速率限制、设备风控、密码重设；所有密钥来自受保护环境/凭据管理，自动轮换，日志脱敏。
- 权限：所有消息/媒体/群管理/好友/动态/开发者资源的服务端对象级权限检查；管理后台 RBAC、双人审批高风险动作（按需）、审计不可任意删除。
- IM 消息传输必须使用 TLS，验证客户端证书/服务器配置、token 生命周期；E2EE 如实施须使用成熟且可审计的协议，不自造加密方案。
- 文件媒体：MIME/魔数、AV、大小/速率、签名下载、内容隔离、删除回收、转码沙箱、恶意文档预览风险。
- Web/Electron：CSP、XSS/CSRF/SSRF、依赖完整性、禁用 nodeIntegration、contextIsolation、安全 preload、IPC allowlist、外部链接校验、签名更新。
- 移动：安全存储 Token、证书验证、屏幕截取与隐私提示按系统能力、备份排除敏感数据、越狱/root 风险提示按策略。
- 小程序：origin/URL 白名单、Bridge 最小权限、隔离、SDK 风险扫描、审核与可撤销授权。
- 备份/灾备：RPO/RTO 可配置且压测验证，数据库 PITR、文件存储版本与生命周期、恢复演练、备份加密与权限。
- QA：SAST/DAST/SCA、SBOM、license scan、secret scan、fuzz/race、安全回归；高危/严重漏洞、未审计核心权限漏洞阻止上线。
- 隐私：明确数据保留、合法请求、删除/导出、地区要求、未成年人保护、管理员访问边界；不能默认永久保存所有私聊内容并对外宣称合规。

## 9. 原生 Linux + 宝塔部署与持续交付

- 不使用 Docker 或 Compose 作为交付运行方式。Go 交叉编译成 Linux 二进制，配置 `systemd` units，非 root 服务账号、目录权限、Resource Limits、Restart、Health；Nginx 站点由宝塔管理。
- WuKongIM 与业务 Go 服务独立运行；MySQL 8/Redis 8 独立安装并限制本地/内网访问；附件走 S3 兼容服务或云对象存储，避免被 Docker 依赖锁死。
- 官方 Compose 配置只能供理解依赖拓扑，不能照搬默认账号密码或开放端口。
- 输出完整安装向导（install.lock/首次管理员初始化/环境预检/数据库迁移）、配置管理、日志轮转、备份恢复、健康/就绪探针、滚动/灰度发布与回滚脚本；`install.lock` 不得成为唯一鉴权或完整性依据。
- 对外仅暴露必要 HTTPS/WSS，WuKongIM TCP 直连场景经 TLS 与安全评估才可开放；内网管理端口隐藏；CDN/反代正确传递真实 IP 需可信代理列表，不信任任意客户端头。
- 可观测性：Prometheus/Grafana 或兼容替代、OpenTelemetry、集中日志、错误追踪、消息投递延迟、连接数、投递成功率、数据库连接池、推送成功率、文件失败率、支付对账、风控、开放 API/小程序错误率与告警。
- CI/CD：锁定依赖、可重复构建、签名/SBOM、测试门禁、制品校验和可回滚。移动端和桌面端提供各平台签名、版本升级、灰度说明。

## 10. 测试策略：减少线上重复测试

1. 首先建立隔离的 dev、integration、staging、production。线上只做健康检查、经审批的灰度与必要观测。
2. 基础功能建立可重复的 golden test，覆盖旧版与新版消息兼容、历史数据迁移、移动/桌面互通、断网重试、离线消息、群权限、消息编辑/撤回/收藏、注册/注销。
3. 合约测试：OpenAPI、WebSocket/WuKongIM 消息 schema、Webhook、Mini App SDK/API、支付回调。新增服务不得静默破坏现有端。
4. 单元/集成测试：`go test -race`、MySQL/Redis 真实测试实例、Android instrumentation、iOS XCTests、Web component/E2E、Electron Playwright 等；CI 必须给出实际执行命令和结果。
5. 性能：明确目标 SLO（消息交付、连接恢复、关键 API 延迟、崩溃率等），用测试环境模拟 1k→10k→更高连接及群发；指标为验收目标而非宣称达成，记录硬件/配置/流量模型。
6. UI：所有关键屏幕必须提供 360/390 手机、平板、桌面常见断点截图；暗黑模式、英文/中文、字体放大、慢网、空状态、异常状态、无障碍、端间设计一致性回归。
7. 安全：OWASP ASVS/MASVS 基线，越权、Token 续期、客服跨租户、下载 URL、消息删除后权限、小程序 XSS/Bridge、Webhook 重放、支付重放等负面测试。
8. 失败门禁：任何高危安全问题、核心 E2E 失败、迁移不可回滚、关键设备崩溃、许可证冲突、无备份恢复证明，**不得标记 Release Ready**。
9. 每 Story 交付必须包含：PRD/设计链接、代码 PR、测试报告、视觉对照、性能影响、回滚与配置说明、必要的操作手册。

## 11. 分阶段执行（必须按依赖，不得跳阶段）

- **Phase 0：证据与供应链审计**：完整抓取文档与源码，锁定提交，许可证/依赖/漏洞/功能矩阵；可运行基线，形成风险优先级。Exit：每端至少有可复现的构建结果或明确阻断项。
- **Phase 1：PRD 与品牌设计**：完整需求、跨端 IA、用户旅程、12 模块功能矩阵、开放平台 PRD、设计系统、Logo 多平台输出、技术 ADR；Exit：评审通过。
- **Phase 2：基础架构/安全/测试基座**：Go 服务结构、DB migrations、鉴权、审计、配置、接口契约、测试/CI、原生部署；Exit：登录、基础消息、基本监控贯通。
- **Phase 3：全端基础功能商业重构**：Android、iOS、Web/Electron、Manager 全面 UI 重设计 + 基础能力修复；Exit：基础功能端到端、视觉回归和安全测试通过。
- **Phase 4：12 项官方收费模块独立实现**：按依赖推进：收藏/文件/位置/标签/群管理/安全 → 小视频/表情/朋友圈/旗舰 → 客服/音视频；Exit：逐条明细交叉端验收。
- **Phase 5：开放平台与小程序生态**：开发者中心、OAuth、SDK、H5/Mini App Runtime、应用市场、审查/发布/回滚与机器人；Exit：真实示例小程序从提交到审核上线、跨端运行及撤权通过。
- **Phase 6：商业增长与官网**：订阅/订单/支付/运营/客服报表、官网、下载中心、文档、国际化；Exit：端到端交易核对与官网下载闭环。
- **Phase 7：硬化与发布**：高并发、灾备演练、渗透测试、合规材料、签名/商店审核准备、灰度；Exit：Release Readiness 逐项签字。

## 12. 建议仓库组织（按实际技术审计可修订）

```text
veyrogram/
  brand/source/                 # logo.png 原图（未经授权不得替换）
  brand/icons/                  # 多平台 AppIcon + 自制 UI SVG
  docs/audit/                   # 上游证据、风险、许可证、功能矩阵
  docs/product/                 # PRD、功能验收、用户流程
  docs/design/                  # Token、组件、原型、视觉回归
  docs/architecture/            # ADR、API、数据、威胁模型
  docs/agent/                   # 模型路由与任务结果日志
  docs/operations/              # 安装、升级、监控、备份、回滚
  services/api/                 # Go 业务/API
  services/worker/              # 任务与 Outbox
  services/open-platform/       # Go 开放平台服务（可先模块化单体）
  services/wukongim/            # 配置/适配/版本依赖，不抹除许可证
  apps/android/
  apps/ios/
  apps/web/
  apps/desktop/
  apps/manager/
  apps/developer/
  apps/website/
  packages/contracts/           # OpenAPI、事件、消息 Schema
  packages/design-tokens/       # 多端 tokens / icon sources
  packages/miniapp-sdk/
  packages/client-sdk/
  infrastructure/systemd/
  infrastructure/nginx/
  infrastructure/scripts/
  tests/contract/
  tests/e2e/
  tests/performance/
  tests/security/
  .github/workflows/
```

保留 upstream remote 与 fork commit pin，不直接覆盖原有仓库；如多仓拆分更适合各端 CI，提交 ADR 说明依赖锁与 release train。优先模块化单体，避免为“微服务”而微服务。统一 VeyroGram 命名、配置前缀、可升级的数据迁移；旧 API 的兼容窗口、数据迁移与回滚必须书面化。

## 13. 首轮必须直接完成的任务（无需再次问确认）

1. 检测仓库/运行系统/实际 BMAD 版本/模型可用列表，自动选取适当 Agent；列出无法访问的资料。
2. 克隆六仓、记录 commit，建立目录和 `source-index.md`；遍历 Docs 和官网文档目录，逐页阅读（按最小必要请求，限速）。
3. 盘点每端全功能、API、消息格式、依赖、测试覆盖与安全缺陷，生成现状矩阵；标注已证实/疑似/未验证。
4. 运行现有测试与构建，保存失败日志及最小复现；仅在本地隔离环境验证，禁止影响上游或生产。
5. 读取 logo.png，制作品牌规范草稿与完整图标尺寸清单，先做高保真关键页面与全端信息架构，不把未经审核设计直接铺满所有页面。
6. 生成 `PRD.md`、`ARCHITECTURE.md`、`DATA-MODEL.md`、`API-CONTRACT.md`、`SECURITY-THREAT-MODEL.md`、`DESIGN-SYSTEM.md`、`OPEN-PLATFORM-PRD.md`、`EPICS.md` 与按依赖排序的 Stories；每 Story 必须包含 DoD、测试和跨端任务。
7. 建立 P0 可运行基线和 CI，按 Phase 2 开始最小纵切（注册→登录→聊天→离线同步→管理审计），无证据不声称可商用。
8. 每一阶段提供中文汇报：完成内容、真实运行结果、问题/风险、下阶段排序、模型路由日志。遇到必要的商业决策（支付服务商、域名、法规辖区、苹果账号、商标冲突）提交具体备选与影响，不可凭空指定。

## 14. 完成定义（Definition of Done）

**全功能**：上游基础功能无可证明回归，12 个高级模块逐行匹配官方需求，新增商业/开放平台功能完成 PRD 验收。**全端**：Android、iOS、Web、桌面、后台、开发者平台、官网有一致的完整品牌 UI，不使用 Emoji 做菜单图标。**安全**：高危安全阻断项清零，权限/合规/依赖证据齐备。**可靠**：自动测试、弱网、备份恢复、压测、监控与回滚实际演练。**交付**：无 Docker 的原生 Linux/宝塔部署文档、可重复编译的签名应用、数据库脚本、配置说明、用户/管理员/开发者手册、许可证清单及审计报告。

**现在执行 Phase 0，不要只给路线图或空白文件；先把可复现的审计和构建事实交出来，然后再用 BMAD 工作流分解实施。**
