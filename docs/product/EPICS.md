---
stepsCompleted: [1, 2, 3, 4]
inputDocuments: [docs/product/PRD.md, docs/product/OPEN-PLATFORM-PRD.md, docs/product/addendum.md, docs/architecture/ARCHITECTURE.md, docs/design/DESIGN-SYSTEM.md, docs/audit/feature-matrix.csv, docs/audit/risk-register.md]
status: draft
---

# VeyroGram - Epic Breakdown

## Overview

本文把 PRD（FR-1…72、NFR-1…18）、开放平台 PRD（OP-FR-1…26）、架构（AD-1…24）与设计系统拆解为可实现的 Epic 与 Story。按 BMAD-MASTER §11 的 Phase 2–7 依赖排序；Story 只依赖更早的 Story。**阶段按 Story 标注**（Epic 可跨阶段）：标签、群高级管理、安全与隐私深化（设备锁/改密/注销/MFA/隐私）属 Phase 4，与 PRD §6.1 和 BMAD-MASTER §11 一致；基础在线状态、会话内查找、截屏/撤回通知属 Phase 3。执行顺序为“先阶段、再编号”，Story 只依赖顺序更靠前的 Story。每个 Story 的完整细节（测试、11 列跨端任务、配置与回滚）见 `docs/stories/`；逐行追溯见 `docs/product/traceability.csv`（功能矩阵 260 行 → Story）。

**约束提醒：** MySQL 8.0（已过 EOL，风险已接受）、Redis 7.x、Go 模块化单体、原生 systemd（禁 Docker）、WuKongIM 仅经 imgw 可达、客户端关键词搜索在本地（ADR-0010）。设计门禁（`docs/design/APPROVALS.md`）未全部签字前，Phase 3 批量编码不启动。

## Requirements Inventory

### Functional Requirements

- FR-1: 手机号注册与登录
- FR-2: 密码与重设
- FR-3: 设备列表、设备锁与会话撤销
- FR-4: 可选 MFA / Passkey
- FR-5: 账号注销与数据导出
- FR-6: 可靠投递与幂等
- FR-7: 离线同步与多端一致
- FR-8: 消息类型与内容类型注册表
- FR-9: 引用、转发、合并转发、删除范围
- FR-10: 撤回与编辑一致性
- FR-11: 输入状态、系统消息与扫码
- FR-12: 好友申请与黑名单
- FR-13: 通讯录匹配
- FR-14: 联系人标签（官方付费模块基线）
- FR-15: 群角色与权限
- FR-16: 邀请确认与群黑名单
- FR-17: 禁言（全员/个人/时长）
- FR-18: 历史可见性与互加好友限制
- FR-19: 撤回成员消息与群审计
- FR-20: 消息收藏与管理
- FR-21: 位置消息
- FR-22: 文件上传、下载与预览
- FR-23: 病毒扫描与隔离
- FR-24: 录制/选择、转码、播放与转发
- FR-25: 表情面板与表情包管理
- FR-26: 发布与可见性
- FR-27: 评论、点赞与提醒
- FR-28: 动态审核与举报
- FR-29: 隐私设置
- FR-30: 已读/未读与成员详情
- FR-31: 消息点赞与在线状态
- FR-32: 图片编辑
- FR-33: 历史消息查找
- FR-34: 截屏通知与撤回通知
- FR-35: 一对一音视频通话
- FR-36: 群通话与会议
- FR-37: 系统集成与权限
- FR-38: 访客入口与自动分配
- FR-39: 转接、SLA 与工单
- FR-40: 知识库、快捷回复、质检与绩效
- FR-41: 全局搜索与媒体中心
- FR-42: 多通道推送
- FR-43: 举报与申诉
- FR-44: 反垃圾与账户/设备/IP 风控
- FR-45: RBAC 与最小权限
- FR-46: 高危操作审批与审计
- FR-47: 运营配置与版本更新
- FR-48: 开发者账号与应用生命周期
- FR-49: OAuth 授权与最小 scope
- FR-50: 小程序运行时与 Bridge
- FR-51: 审核、灰度、发布与回滚
- FR-52: Webhook 与开放 API
- FR-53: 机器人与服务账号
- FR-54: 权益与配额
- FR-55: 订单、支付回调与对账
- FR-56: 官网与下载中心
- FR-57: 客户端更新与灰度
- FR-58: 频道/广播与运营消息
- FR-59: AI 辅助（翻译/总结/客服建议）
- FR-60: 企业组织与租户（tenant-aware 设计）
- FR-61: 个人资料与通用设置
- FR-62: 对象存储生命周期与清理
- FR-63: 小程序市场与发现
- FR-64: 群创建与成员管理基础能力
- FR-65: 语音、名片与链接预览
- FR-66: 清空聊天记录与服务端删除范围
- FR-67: Web/桌面登录与设备批准
- FR-68: 设计系统与原创图标库
- FR-69: 多平台品牌资产与视觉回归
- FR-70: 桌面端专属体验
- FR-71: SDK、沙箱与团队协作
- FR-72: 索引查询、审核队列与统计报表

- OP-FR-1: 注册与主体认证（→ FR-48）
- OP-FR-2: 团队与角色（→ FR-71）
- OP-FR-3: 应用创建与类型（→ FR-48）
- OP-FR-4: 域名与包名绑定
- OP-FR-5: 密钥生命周期（→ FR-48）
- OP-FR-6: 授权码 + PKCE（→ FR-49）
- OP-FR-7: 同意页与最小 scope
- OP-FR-8: 撤销与令牌管理（→ FR-49）
- OP-FR-9: 开放 API（→ FR-52、FR-53）
- OP-FR-10: SDK（→ FR-71）
- OP-FR-11: Webhook（→ FR-52）
- OP-FR-12: 沙箱与调试（→ FR-71）
- OP-FR-13: 机器人（→ FR-53）
- OP-FR-14: 服务账号与订阅消息
- OP-FR-15: 容器安全基线
- OP-FR-16: Bridge 协议
- OP-FR-17: 运行时兼容与降级
- OP-FR-18: 自动风险扫描
- OP-FR-19: 人工审核与申诉
- OP-FR-20: 灰度、发布与回滚
- OP-FR-21: 浏览、搜索与详情
- OP-FR-22: 评价与举报
- OP-FR-23: 控制台
- OP-FR-24: 统计与计量
- OP-FR-25: 投诉与处置
- OP-FR-26: 结算与分成

### NonFunctional Requirements

- NFR-1: 授权
- NFR-2: IM 内核隔离
- NFR-3: 凭据与默认值
- NFR-4: 传输与数据
- NFR-5: 供应链
- NFR-6: 可靠性与灾备
- NFR-7: 可观测性
- NFR-8: 可访问性与国际化
- NFR-9: 规模与性能
- NFR-10: 部署
- NFR-11: 许可证合规
- NFR-12: API 契约
- NFR-13: Web/Electron 安全
- NFR-14: 移动端安全
- NFR-15: 令牌与连接
- NFR-16: 端口与管理面暴露
- NFR-17: 安全质量门禁
- NFR-18: 威胁建模

### Additional Requirements（架构决策）

- AD-1: Dependency direction
- AD-2: IM core is unreachable except through the IM Gateway adapter  [R-21, NFR-2, NFR-16]
- AD-3: Server-side object-level authorization, deny by default
- AD-4: Message ownership, pipeline, idempotency and Content Type Registry
- AD-5: Persistence: MySQL authoritative, Redis disposable
- AD-6: Object storage via S3-compatible port with scan gate
- AD-7: Transactional outbox + worker
- AD-8: API contract first (OpenAPI 3.1)
- AD-9: Authentication, tokens and devices
- AD-10: Tenant-aware, single-tenant-deployed
- AD-11: Mini-app runtime and Bridge  [PROPOSED — feasibility spike required]
- AD-12: Open platform is a module with its own edge
- AD-13: Real-time media is a separate plane behind a port
- AD-14: Native Linux deployment, no containers
- AD-15: Observability and audit
- AD-16: Real client IP only from trusted proxies
- AD-17: Client and SDK topology
- AD-18: Upstream is pinned and isolated
- AD-19: Search honors read permissions
- AD-20: Typed configuration with safe-by-default startup
- AD-21: Send-path enforcement via synchronous IM hook
- AD-22: Backup and disaster recovery
- AD-23: Quality, supply-chain and performance gates
- AD-24: Admin plane separation

### UX Design Requirements

- UX-DR1: 设计令牌单一来源并校验对比度（Story 6.1）
- UX-DR2: 原创图标库且导航/菜单禁用 Emoji（Story 6.2）
- UX-DR3: 多平台品牌资产（启动页、应用/通知图标、PWA/桌面图标）（Story 6.3）
- UX-DR4: 桌面端三栏、快捷键、托盘、拖拽与加固壳（Story 6.4）
- UX-DR5: 每个界面具备加载/空/错误/离线/无权限/降级状态（Story 7.1, 9.6, 16.8）
- UX-DR6: WCAG 2.1 AA 验收（自动化 + 人工 + 第三方审计）（Story 6.5, 20.4）
- UX-DR7: 200% 字号与 320px 回流无内容丢失（Story 5.8, 6.5）
- UX-DR8: 授权同意页与小程序容器顶栏由宿主绘制，不可伪造（Story 16.4, 17.2）
- UX-DR9: 拖拽操作提供非拖拽替代控件（Story 11.4, 12.1）
- UX-DR10: 简体中文与英文 i18n，主题明/暗（Story 5.8）

### FR Coverage Map

FR-1: Story 1.12, 1.14
FR-2: Story 5.4
FR-3: Story 1.13, 1.14, 5.3, 5.7
FR-4: Story 5.5
FR-5: Story 5.6
FR-6: Story 1.15, 1.17, 1.18, 2.2
FR-7: Story 1.16, 1.17, 1.18, 7.1
FR-8: Story 2.1
FR-9: Story 2.5
FR-10: Story 2.4, 13.2
FR-11: Story 2.6, 3.3
FR-12: Story 3.1, 3.2, 3.4
FR-13: Story 3.3
FR-14: Story 3.5
FR-15: Story 4.3
FR-16: Story 4.4
FR-17: Story 4.5
FR-18: Story 4.6
FR-19: Story 4.7
FR-20: Story 10.1, 10.1
FR-21: Story 10.2
FR-22: Story 10.3
FR-23: Story 8.3, 10.4
FR-24: Story 11.1, 11.2
FR-25: Story 11.3, 11.4, 11.5
FR-26: Story 12.1
FR-27: Story 12.2, 12.3
FR-28: Story 12.4
FR-29: Story 5.2
FR-30: Story 13.1
FR-31: Story 13.3
FR-32: Story 13.4
FR-33: Story 13.5
FR-34: Story 13.6, 13.6
FR-35: Story 15.0, 15.1, 15.2
FR-36: Story 15.0, 15.3
FR-37: Story 15.4
FR-38: Story 14.1, 14.2, 19.3
FR-39: Story 14.2, 14.3
FR-40: Story 14.3
FR-41: Story 7.2, 7.3
FR-42: Story 7.4, 7.5, 7.6
FR-43: Story 8.1, 9.3, 12.4
FR-44: Story 8.2, 8.3, 8.4
FR-45: Story 1.19, 1.20, 9.1, 9.7
FR-46: Story 1.3, 1.4, 1.19, 1.20, 4.7, 9.1, 9.7
FR-47: Story 9.4, 9.6
FR-48: Story 16.3
FR-49: Story 16.4
FR-50: Story 17.1, 17.2
FR-51: Story 17.3, 17.4, 17.5
FR-52: Story 16.5
FR-53: Story 16.7
FR-54: Story 18.1
FR-55: Story 18.2
FR-56: Story 19.1
FR-57: Story 9.4, 19.2, 20.6
FR-58: Story 21.1
FR-59: Story 21.2
FR-60: Story 21.3
FR-61: Story 5.1, 5.8, 7.1
FR-62: Story 10.3, 10.4
FR-63: Story 17.6
FR-64: Story 4.1, 4.2, 4.8
FR-65: Story 2.2, 2.3
FR-66: Story 2.7
FR-67: Story 5.7
FR-68: Story 6.1, 6.2, 9.6
FR-69: Story 6.3, 6.5
FR-70: Story 6.4
FR-71: Story 16.8, 16.9
FR-72: Story 9.2, 9.3, 9.5
OP-FR-1: Story 16.1
OP-FR-2: Story 16.2
OP-FR-3: Story 16.3
OP-FR-4: Story 16.3
OP-FR-5: Story 16.3
OP-FR-6: Story 16.4
OP-FR-7: Story 16.4
OP-FR-8: Story 16.4
OP-FR-9: Story 16.5
OP-FR-10: Story 16.8
OP-FR-11: Story 16.6
OP-FR-12: Story 16.6
OP-FR-13: Story 16.7
OP-FR-14: Story 16.7
OP-FR-15: Story 17.2
OP-FR-16: Story 17.2
OP-FR-17: Story 17.2
OP-FR-18: Story 17.3
OP-FR-19: Story 17.4
OP-FR-20: Story 17.5
OP-FR-21: Story 17.6
OP-FR-22: Story 17.6
OP-FR-23: Story 16.9
OP-FR-24: Story 16.5
OP-FR-25: Story 17.7
OP-FR-26: Story 18.3

### 功能矩阵行追溯

`docs/product/traceability.csv` 将 `docs/audit/feature-matrix.csv` 的 260 行逐行映射到 Story（基础版 139 行 + 付费模块 121 行）。

## Epic List

### Epic 1: 账号与最小聊天纵切（可信基座）（Phase 2）
用户可以注册、登录并在两台设备间收发单聊消息且离线后不丢；运营可以初始化管理员并查看审计。同时建立后续一切故事依赖的安全、测试与原生部署基座（IM 内核隔离、迁移、outbox、CI 门禁）。
**FRs covered:** FR-1, FR-3, FR-45, FR-46, FR-6, FR-7  ·  **Stories:** 24

### Epic 2: 完整的消息体验（Phase 3）
用户可以发送并管理各种类型的消息——图片、语音、名片、链接，并撤回、引用、转发、删除；旧客户端遇到新类型也能优雅降级。
**FRs covered:** FR-10, FR-11, FR-6, FR-65, FR-66, FR-8, FR-9  ·  **Stories:** 7

### Epic 3: 联系人与社交关系（Phase 3–4）
用户可以添加好友、管理黑名单、通过通讯录或二维码找到人，并用标签整理联系人。
**FRs covered:** FR-11, FR-12, FR-13, FR-14  ·  **Stories:** 5

### Epic 4: 群聊与高级群管理（Phase 3–4）
用户可以创建和管理群：邀请成员、设置公告、@成员，群主与管理员可以审核、禁言、控制历史可见性并撤回违规消息，所有管理动作可审计。
**FRs covered:** FR-15, FR-16, FR-17, FR-18, FR-19, FR-46, FR-64  ·  **Stories:** 8

### Epic 5: 个人资料、隐私与账号安全（Phase 3–4）
用户可以管理个人资料与隐私，查看并撤销设备，设置密码与二次验证，通过扫码登录桌面端，并能注销账号或导出数据。
**FRs covered:** FR-2, FR-29, FR-3, FR-4, FR-5, FR-61, FR-67  ·  **Stories:** 8

### Epic 6: 统一品牌与设计系统落地（Phase 3）
所有客户端使用同一套品牌、令牌与原创图标库，桌面端有专属体验，视觉与无障碍有可回归的基线。
**FRs covered:** FR-68, FR-69, FR-70  ·  **Stories:** 5

### Epic 7: 会话列表、搜索与通知（Phase 3）
用户可以高效管理会话（置顶、免打扰、红点、状态），在本地搜索聊天记录，并收到不泄露敏感内容、多设备不重复的推送。
**FRs covered:** FR-41, FR-42, FR-61, FR-7  ·  **Stories:** 6

### Epic 8: 举报、内容安全与风控（Phase 3）
用户可以举报并申诉，平台能检测垃圾、危险链接与文件并限速，真实 IP 只信任可信代理。
**FRs covered:** FR-23, FR-43, FR-44  ·  **Stories:** 4

### Epic 9: 管理后台商业运营能力（Phase 3）
运营、风控、审核可以在最小权限下处理用户、群、消息、举报、配置与报表，高危动作需审批，所有操作可追溯。
**FRs covered:** FR-43, FR-45, FR-46, FR-47, FR-57, FR-68, FR-72  ·  **Stories:** 7

### Epic 10: 收藏、位置与文件（Phase 4）
用户可以收藏消息、发送位置、发送并预览文件，文件有类型校验、病毒扫描与签名下载。
**FRs covered:** FR-20, FR-21, FR-22, FR-23, FR-62  ·  **Stories:** 4

### Epic 11: 小视频与表情商店（Phase 4）
用户可以录制或选择小视频发送，使用表情面板与自定义表情，并从表情商店添加官方表情包。
**FRs covered:** FR-24, FR-25  ·  **Stories:** 5

### Epic 12: 朋友圈（动态）（Phase 4）
用户可以发布图文/视频动态，用公开、私有、部分可见、不给谁看控制可见性，好友可评论点赞，违规内容可举报并审核。
**FRs covered:** FR-26, FR-27, FR-28, FR-43  ·  **Stories:** 4

### Epic 13: 旗舰消息能力（Phase 3–4）
用户可以看到已读/未读详情、编辑消息、给消息点赞、看到在线状态、编辑图片、按日期/成员/类型查找历史，并在系统支持时收到截屏与撤回通知。
**FRs covered:** FR-10, FR-30, FR-31, FR-32, FR-33, FR-34  ·  **Stories:** 6

### Epic 14: 客服系统（Phase 4）
访客与用户可以联系客服，系统按技能组自动分配，客服可转接、使用快捷回复与知识库并创建工单，主管有 SLA、质检与绩效视图。
**FRs covered:** FR-38, FR-39, FR-40  ·  **Stories:** 3

### Epic 15: 音视频通话与会议（Phase 4）
用户可以发起一对一与多人音视频通话和会议，处理忙线/超时/重连，弱网自适应，录制需明确同意。
**FRs covered:** FR-35, FR-36, FR-37  ·  **Stories:** 5

### Epic 16: 开放平台基础（Phase 5）
开发者可以注册主体、组建团队、创建应用与密钥，通过 OAuth 授权调用开放 API，订阅 Webhook，使用机器人与 SDK，并在控制台查看统计。
**FRs covered:** FR-48, FR-49, FR-52, FR-53, FR-71, OP-FR-1, OP-FR-10, OP-FR-11, OP-FR-12, OP-FR-13, OP-FR-14, OP-FR-2, OP-FR-23, OP-FR-24, OP-FR-3, OP-FR-4, OP-FR-5, OP-FR-6, OP-FR-7, OP-FR-8, OP-FR-9  ·  **Stories:** 9

### Epic 17: 小程序运行时与市场（Phase 5）
用户可以在受限容器里安全使用小程序，开发者可提交、审核、灰度发布与回滚，用户可发现、评价、举报，平台可投诉处置。
**FRs covered:** FR-50, FR-51, FR-63, OP-FR-15, OP-FR-16, OP-FR-17, OP-FR-18, OP-FR-19, OP-FR-20, OP-FR-21, OP-FR-22, OP-FR-25  ·  **Stories:** 7

### Epic 18: 订阅、权益与支付（Phase 6）
平台可以按套餐赋予服务端强制的权益，并在合规支付渠道落地后完成订单、支付回调与对账。
**FRs covered:** FR-54, FR-55, OP-FR-26  ·  **Stories:** 3

### Epic 19: 官网、下载与客户端更新（Phase 6）
访客可以在官网了解产品并下载对应平台的应用；已安装的客户端可以安全更新与灰度。
**FRs covered:** FR-38, FR-56, FR-57  ·  **Stories:** 3

### Epic 20: 硬化与发布就绪（Phase 7）
平台通过压测、灾备演练、安全测试、WCAG 验收与合规检查，具备逐项签字的发布就绪证据。
**FRs covered:** FR-57  ·  **Stories:** 6

### Epic 21: 未排期扩展（P1/P2 backlog）（未排期）
架构已预留、PRD 评审后再排期的扩展能力；本期不实现，也不出现在任何里程碑。
**FRs covered:** FR-58, FR-59, FR-60  ·  **Stories:** 3

## Epic 1: 账号与最小聊天纵切（可信基座）

用户可以注册、登录并在两台设备间收发单聊消息且离线后不丢；运营可以初始化管理员并查看审计。同时建立后续一切故事依赖的安全、测试与原生部署基座（IM 内核隔离、迁移、outbox、CI 门禁）。

### Story 1.1: 服务骨架、配置校验与健康探针

As a 平台所有者,
I want 一个按模块化单体组织、用 systemd 原生部署、启动时拒绝不安全配置的 Go 服务骨架,
So that 后续故事能在统一结构上增量实现，并且默认值不会把弱口令带进生产.

*阶段：Phase 2｜需求：NFR-3,NFR-10,NFR-12｜架构：AD-1,AD-14,AD-20｜依赖：无｜规模：M*

**Acceptance Criteria:**

**Given** 生产配置含固定短信验证码、默认管理员口令、短 JWT 密钥或 `*` CORS
**When** 服务启动
**Then** 进程拒绝启动并逐项列出违规配置

**Given** 已按 infrastructure/systemd 部署
**When** 检查进程
**Then** 服务以非 root 专用账号运行，/healthz 与 /readyz 仅内网可达

**Given** 服务启动
**When** 访问 /healthz 与 /readyz
**Then** 仅内网可达，依赖未就绪时 readyz 返回失败

详见 `docs/stories/1.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.2: 原生部署骨架与 CI 基线

As a 平台所有者,
I want 原生 systemd 部署骨架与 CI 基线,
So that 后续每个 Story 都能被可重复地构建、部署与回滚.

*阶段：Phase 2｜需求：NFR-3,NFR-10,NFR-12｜架构：AD-1,AD-14,AD-20｜依赖：1.1｜规模：M*

**Acceptance Criteria:**

**Given** 已按 infrastructure/systemd 部署
**When** 检查进程
**Then** 服务以非 root 专用账号运行，/healthz 与 /readyz 仅内网可达

**Given** 提交 PR
**When** CI 运行
**Then** 执行 lint、go test -race、OpenAPI diff、模块依赖方向检查，任一失败则阻断

**Given** 部署产物
**When** 构建
**Then** 二进制与 systemd unit 版本化、可回滚，Nginx 模板仅转发白名单路径

详见 `docs/stories/1.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.3: 迁移基座与幂等表

As a 平台所有者,
I want 带版本的前向迁移与幂等键表,
So that 数据库变更可追踪、可回滚且重复请求不产生副作用.

*阶段：Phase 2｜需求：NFR-4,NFR-7,FR-46｜架构：AD-5,AD-7,AD-15｜依赖：1.1｜规模：M*

**Acceptance Criteria:**

**Given** 空的 MySQL 8.0 库
**When** 运行迁移
**Then** 创建 schema_migration 与 idempotency_key，所有表为 InnoDB + utf8mb4_0900_ai_ci，TEXT/BLOB/JSON 列无 DEFAULT（静态检查与 information_schema 双重验证）

**Given** 请求带 Idempotency-Key
**When** 同一请求在 24 小时内重复提交
**Then** 返回首次响应并标记 Idempotent-Replayed，不重复执行；同一键但请求内容不同返回 422，首次仍在处理中返回 409，5xx 不缓存

**Given** 迁移执行中途失败
**When** 再次运行或回滚
**Then** 失败的版本不写入 schema_migration，状态被标记为 dirty 并拒绝继续（MySQL DDL 会隐式提交，无法依赖事务回滚）；对应 down 脚本可恢复到上一版本

**Given** 并发运行迁移
**When** 两个进程同时启动
**Then** 由数据库命名锁保证只有一个执行；已应用迁移被改动时校验和不符并拒绝运行

**Given** 迁移执行中途失败
**When** 回滚
**Then** 事务回滚且 schema_migration 不记录该版本，可用回滚脚本恢复到上一版本

详见 `docs/stories/1.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.4: Outbox、worker 与审计日志

As a 平台所有者,
I want 事务 outbox、worker 与追加写审计日志,
So that 状态变更与事件原子提交，高危操作可追溯且不可被应用账号篡改.

*阶段：Phase 2｜需求：NFR-4,NFR-7,FR-46｜架构：AD-5,AD-7,AD-15｜依赖：1.3｜规模：M*

**Acceptance Criteria:**

**Given** 应用数据库账号
**When** 对 audit_log 执行 UPDATE 或 DELETE
**Then** 被数据库权限拒绝

**Given** worker 处理 outbox
**When** 同一事件被重复投递
**Then** 消费者按事件 ID 幂等，失败退避后进入死信并可重放

**Given** 审计日志
**When** 每日
**Then** 链头哈希锚定到主机外存储（对象存储版本化/WORM 或独立日志主机）

详见 `docs/stories/1.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.5: 可观测性基座（指标、日志、追踪与告警）

As a 平台所有者,
I want 统一的指标、结构化日志、追踪与告警基座,
So that 每个后续 Story 的监控要求（DoD #7）与 Phase 2 Exit“基本监控贯通”有可落地的载体.

*阶段：Phase 2｜需求：NFR-7｜架构：AD-15｜依赖：1.1｜规模：M*

**Acceptance Criteria:**

**Given** 服务运行
**When** 暴露指标
**Then** Prometheus 兼容端点仅内网可达并包含投递延迟、连接数、ACK 率、DB 连接池、推送/文件/风控/开放 API 错误率的占位指标

**Given** 日志
**When** 输出
**Then** 为结构化 JSON 带 trace_id，令牌、手机号、消息正文被脱敏过滤器移除

**Given** 告警规则
**When** 指标越界
**Then** 触发告警并路由到运维通道；监控端点本身需认证且不对公网开放（T-17）

详见 `docs/stories/1.5-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.6: WuKongIM imgw 适配器与内部 webhook

As a 平台所有者,
I want 业务服务通过 imgw 适配器调用 WuKongIM 并接收内部 webhook,
So that 所有对 IM 内核的访问有唯一受控入口.

*阶段：Phase 2｜需求：NFR-2,NFR-15,NFR-16｜架构：AD-2,AD-18｜依赖：1.1｜规模：M*

**Acceptance Criteria:**

**Given** IM 回调业务服务
**When** 请求来自非白名单来源或缺少路径密钥
**Then** 返回 403 并告警

**Given** imgw 适配器
**When** 调用 IM API
**Then** 只经回环/私网，调用失败重试并上报指标

**Given** 固定的 WuKongIM SHA
**When** 运行契约测试
**Then** 接口契约通过

详见 `docs/stories/1.6-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.7: 暴露面加固与负面测试

As a 平台所有者,
I want WuKongIM 与其他内部端口对公网与非白名单来源不可达，默认密钥全部覆盖,
So that 堵住无鉴权发信/改令牌的攻击面（R-21）.

*阶段：Phase 2｜需求：NFR-2,NFR-15,NFR-16｜架构：AD-2,AD-18｜依赖：1.6,1.2｜规模：M*

**Acceptance Criteria:**

**Given** 生产部署
**When** 从公网、同网段非白名单主机访问 5001/5301/7001/9099/pprof/MQTT
**Then** 全部不可达

**Given** WuKongIM 默认配置
**When** 部署校验运行
**Then** 覆盖 cluster.join_token、manager 用户与 jwt_secret、bench.api_token，并关闭插件热加载与 MQTT

**Given** 部署校验
**When** 在 CI 与预发运行
**Then** 暴露面扫描（外部+内网）为必过门禁，任何可达的内部端口即失败

详见 `docs/stories/1.7-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.8: WuKongIM fork、补丁与升级流程

As a 平台工程团队,
I want 固定 SHA 的 WuKongIM fork、补丁（webhook HMAC）与升级验证流程,
So that 内核升级可控，无签名 webhook 等已知缺口有闭环.

*阶段：Phase 2｜需求：NFR-2,NFR-5｜架构：AD-2,AD-18｜依赖：1.6｜规模：M*

**Acceptance Criteria:**

**Given** fork
**When** 建立
**Then** 保留 upstream remote、记录固定 SHA 与 go1.26.10 工具链，变更保留许可证与署名

**Given** webhook 无签名
**When** 补丁
**Then** 增加 HMAC 签名与时间戳校验并在 imgw 校验

**Given** 升级
**When** 切换 SHA
**Then** 运行 imgw 契约测试与核心 E2E 才能放行

详见 `docs/stories/1.8-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.9: 客户端构建基线：Android 与 iOS

As a 客户端开发者,
I want 四端客户端在隔离环境可复现构建，依赖与包名问题清零,
So that 后续各端界面与功能 Story 有可编译、可测试的起点（R-12/13/15/17/19）.

*阶段：Phase 2｜需求：NFR-5,NFR-13,NFR-14｜架构：AD-17,AD-18｜依赖：1.2｜规模：M*

**Acceptance Criteria:**

**Given** Android
**When** 构建
**Then** 使用 VeyroGram 包名而非 com.test.ts，应用 ID 与 R 导入重构完成，debug APK 可复现构建

**Given** iOS
**When** 构建
**Then** 过期 Pods（含 AFNetworking）已替换或升级，在 Xcode 26 工具链下无签名模拟器构建通过

**Given** Android 与 iOS
**When** 在隔离环境运行
**Then** 无签名 debug 构建与单元测试可由脚本一键复现，并保存日志

详见 `docs/stories/1.9-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.10: 客户端构建基线：Web、桌面与后台

As a 客户端开发者,
I want 四端客户端在隔离环境可复现构建，依赖与包名问题清零,
So that 后续各端界面与功能 Story 有可编译、可测试的起点（R-12/13/15/17/19）.

*阶段：Phase 2｜需求：NFR-5,NFR-13,NFR-14｜架构：AD-17,AD-18｜依赖：1.2｜规模：M*

**Acceptance Criteria:**

**Given** Web 与桌面
**When** 构建
**Then** 锁文件不含第三方镜像 URL，React/Electron 升级到受支持版本，高危漏洞清零，Manager 使用官方源冻结安装

**Given** Web 与桌面
**When** 冻结安装并构建
**Then** 构建通过且产物可复现，Electron 安全配置检查通过

**Given** 后台 Manager
**When** 使用官方源冻结安装
**Then** 构建通过，高危漏洞清零或有受审批的豁免记录

详见 `docs/stories/1.10-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.11: 生成客户端与各端 IM SDK 接入

As a 客户端开发者,
I want 由 OpenAPI 生成的各端客户端与固定版本的 IM SDK 接入层,
So that 各端不各自实现协议与状态规则（AD-17）.

*阶段：Phase 2｜需求：NFR-12｜架构：AD-8,AD-17｜依赖：1.4,1.6,1.9,1.10｜规模：M*

**Acceptance Criteria:**

**Given** OpenAPI 契约
**When** 生成
**Then** 产出 TS/Kotlin/Swift/Go 客户端并随版本发布

**Given** IM SDK
**When** 接入
**Then** 固定到与服务端 WuKongIM 兼容的版本，并有对该固定版本的契约测试

**Given** 令牌
**When** 保存
**Then** 使用系统安全存储，校验服务器证书，敏感数据排除系统备份

详见 `docs/stories/1.11-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.12: 手机号注册与登录（服务端）

As a 终端用户,
I want 用手机号和一次性验证码注册并登录,
So that 快速、安全地开始使用且无法被固定验证码绕过.

*阶段：Phase 2｜需求：FR-1｜架构：AD-9,AD-16,AD-20｜依赖：1.3,1.6,1.11｜规模：M*

**Acceptance Criteria:**

**Given** 生产配置
**When** 请求验证码
**Then** 只发送随机一次性验证码，不存在固定值，且超限返回 429 并写风控事件

**Given** 验证码正确
**When** 登录
**Then** 创建用户与设备记录并签发短期访问令牌与轮换刷新令牌

**Given** 同一手机号/设备/IP 的连续失败
**When** 达到阈值
**Then** 延迟与设备风控生效，并写 login_event

详见 `docs/stories/1.12-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.13: 设备、令牌与会话撤销（服务端）

As a 终端用户,
I want 登录后每台设备有独立记录，并能撤销任一设备,
So that 丢失设备或账号被盗时能立即止损.

*阶段：Phase 2｜需求：FR-3,NFR-14,NFR-15｜架构：AD-9｜依赖：1.12｜规模：M*

**Acceptance Criteria:**

**Given** 用户在两台设备登录
**When** 撤销其中一台
**Then** 该设备访问令牌在 60 秒内失效并通过 /user/device_quit 关闭其 IM 会话

**Given** 刷新令牌被重放
**When** 检测到复用
**Then** 整条令牌链吊销并通知其他设备

**Given** 同设备类别的两台设备
**When** 撤销一台
**Then** IM 令牌轮换，另一台通过刷新令牌自动重取，不出现永久掉线

**Given** 访问令牌
**When** 设备被撤销
**Then** Redis 拒绝列表覆盖全部已认证路由（每请求一次查询），Redis 不可用时降级为令牌 TTL（≤15 分钟）并告警

**Given** 多节点 IM 集群
**When** 调用 /user/device_quit
**Then** 仅保证 owner-local 会话关闭；v1 按单节点交付，集群上线前必须通过踢线传播测试

**Given** 用户退出登录
**When** 确认
**Then** 撤销该设备令牌与 IM 令牌并清除本地凭据

详见 `docs/stories/1.13-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.14: 登录与设备页客户端（四端最小实现）

As a 终端用户,
I want 在四端用手机号登录并管理设备,
So that Phase 2 的登录纵切在四端可用.

*阶段：Phase 2｜需求：FR-1,FR-3｜架构：AD-17,AD-9｜依赖：1.11,1.13｜规模：M*

**Acceptance Criteria:**

**Given** 登录页
**When** 输入手机号与验证码
**Then** 登录成功并把令牌保存到系统安全存储，错误与限流有明确提示

**Given** 设备列表
**When** 打开
**Then** 显示当前设备与其他设备并可下线（界面使用设计令牌与图标库，不沿用上游样式）

**Given** 键盘与读屏
**When** 操作登录与设备页
**Then** 全部可达、焦点环可见、控件有名称

详见 `docs/stories/1.14-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.15: 单聊消息收发（服务端与 before_send 骨架）

As a 终端用户,
I want 在两位用户之间发送并接收文本消息，重试不重复、顺序不乱,
So that 最小可用的聊天闭环.

*阶段：Phase 2｜需求：FR-6｜架构：AD-4,AD-21｜依赖：1.13,1.11｜规模：M*

**Acceptance Criteria:**

**Given** 客户端用 ULID 作为 client_msg_no
**When** 网络抖动导致重发
**Then** IM 按（发送者、client_msg_no、频道）只产生一条消息

**Given** 发送失败
**When** 用户查看
**Then** 显示失败状态与重试入口且不静默丢弃

**Given** 测试夹具在 IM 中写入禁言/黑名单规则
**When** 该用户经 IM 网关发送
**Then** 被 IM 原生 send_ban/黑名单拒绝；Given before_send 超时

详见 `docs/stories/1.15-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.16: 离线同步与多端一致（服务端）

As a 终端用户,
I want 换设备或离线后重新上线，消息、未读数与草稿自动补齐,
So that 跨设备体验一致，不丢消息.

*阶段：Phase 2｜需求：FR-7｜架构：AD-4,AD-5｜依赖：1.15｜规模：M*

**Acceptance Criteria:**

**Given** 新设备首次登录
**When** 同步
**Then** 按游标分页补齐会话与消息，未读数与旧设备一致

**Given** 用户在 A 设备编辑草稿
**When** 5 秒后
**Then** B 设备显示同一草稿 [假设值待测]

**Given** 离线 10 分钟
**When** 重连
**Then** 离线期间消息按序补齐无重复

详见 `docs/stories/1.16-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.17: 聊天与同步客户端：Android 与 iOS

As a 终端用户,
I want 在 Android 与 iOS 上收发单聊消息并离线同步,
So that 移动端聊天纵切可用.

*阶段：Phase 2｜需求：FR-6,FR-7｜架构：AD-4,AD-17｜依赖：1.14,1.15,1.16｜规模：M*

**Acceptance Criteria:**

**Given** 两台设备
**When** 一端发送文本
**Then** 另一端按序收到，重试不重复，失败显示重试入口

**Given** 离线后重连
**When** 同步
**Then** 消息、未读与草稿补齐

**Given** 聊天页
**When** 键盘与读屏
**Then** 消息列表为 log 区域、发送与重试可操作

详见 `docs/stories/1.17-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.18: 聊天与同步客户端：Web 与桌面

As a 终端用户,
I want 在 Web 与桌面上收发单聊消息并离线同步,
So that 桌面与网页聊天纵切可用.

*阶段：Phase 2｜需求：FR-6,FR-7｜架构：AD-4,AD-17｜依赖：1.14,1.15,1.16｜规模：M*

**Acceptance Criteria:**

**Given** 两台设备
**When** 一端发送文本
**Then** 另一端按序收到，重试不重复，失败显示重试入口

**Given** 离线后重连
**When** 同步
**Then** 消息、未读与草稿补齐

**Given** 聊天页
**When** 键盘与读屏
**Then** 消息列表为 log 区域、发送与重试可操作

详见 `docs/stories/1.18-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.19: 管理员初始化与 RBAC 骨架（服务端）

As a 平台所有者,
I want 通过安装向导创建首个管理员，后台可按角色访问并查看审计日志,
So that 没有默认口令，所有管理动作可追溯.

*阶段：Phase 2｜需求：FR-45,FR-46,NFR-3｜架构：AD-3,AD-9,AD-15,AD-24｜依赖：1.4,1.12｜规模：M*

**Acceptance Criteria:**

**Given** 全新部署
**When** 运行安装向导
**Then** 创建首个管理员并强制设置强口令，install.lock 不作为鉴权依据

**Given** 无权限角色
**When** 调用管理接口
**Then** 返回 403，且生成式越权测试覆盖每个路由

**Given** 管理员执行高危动作
**When** 完成
**Then** audit_log 写入操作人、对象、理由与时间并可在后台只读查询

**Given** 管理员账号
**When** 首次登录
**Then** 强制绑定 TOTP 二次验证（管理面独立于用户侧 MFA）；无 MFA 的管理员无法访问任何管理接口

详见 `docs/stories/1.19-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.20: 管理面最小界面（登录、首页、审计）

As a 平台所有者,
I want 在管理后台登录并查看审计日志,
So that 运营能看见并核对每一次管理动作.

*阶段：Phase 2｜需求：FR-45,FR-46｜架构：AD-17,AD-24｜依赖：1.19,1.10｜规模：M*

**Acceptance Criteria:**

**Given** 管理员
**When** 登录管理后台
**Then** 通过 MFA 后进入首页，显示基础概览

**Given** 审计页
**When** 查询
**Then** 只读展示审计日志并支持筛选

**Given** 每个后台页面
**When** 渲染
**Then** 具备加载、空、错误、无权限状态，键盘可达、焦点环可见

详见 `docs/stories/1.20-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.21: 安装向导与运维套件

As a 平台所有者,
I want 环境预检、数据库迁移、首次管理员初始化与配置管理一体的安装向导,
So that 原生 systemd 部署可重复且不依赖 install.lock 作为鉴权.

*阶段：Phase 2｜需求：NFR-10,NFR-3｜架构：AD-14,AD-20｜依赖：1.2,1.3,1.19｜规模：M*

**Acceptance Criteria:**

**Given** 全新主机
**When** 运行预检
**Then** 检查 MySQL 8.0/Redis 7/Nginx/端口/权限并给出阻断项

**Given** 安装
**When** 完成
**Then** 创建独立 dev/test/staging/production 配置模板、日志轮转、滚动/灰度发布与回滚脚本

**Given** 已安装系统
**When** 删除 install.lock 后再次运行向导
**Then** 拒绝重新初始化（见 1.11）

详见 `docs/stories/1.21-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.22: 备份实现（MySQL PITR、IM 存储、对象存储）

As a 平台所有者,
I want 加密的 MySQL 全量+binlog 备份、WuKongIM 数据目录快照、对象存储版本化,
So that 试运营数据从第一天起就有备份，20.2 才有可演练的对象.

*阶段：Phase 2｜需求：NFR-6｜架构：AD-22｜依赖：1.3,1.21｜规模：M*

**Acceptance Criteria:**

**Given** 生产数据库
**When** 备份
**Then** 全量+binlog 加密存放到主机外并限权

**Given** WuKongIM 数据目录
**When** 快照
**Then** 有文档化的静默与一致性步骤并可恢复

**Given** 对象存储
**When** 配置
**Then** 开启版本化与生命周期

详见 `docs/stories/1.22-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.23: 上游兼容与数据迁移决策

As a 产品负责人,
I want 决定是否需要兼容或迁移上游 TangSengDaoDao 的数据与旧客户端，并落实,
So that 避免默默破坏既有部署或假设“绿地”.

*阶段：Phase 2｜需求：NFR-12｜架构：AD-8｜依赖：1.6｜规模：M*

**Acceptance Criteria:**

**Given** 上游数据表与旧路径
**When** 评估
**Then** 输出书面决策：是否提供 compat 层与数据迁移、兼容窗口与下线日期

**Given** 决定提供迁移
**When** 执行
**Then** 迁移脚本幂等、可回滚并有校验报告

**Given** 决定不提供
**When** 发布
**Then** 在文档明确声明并保留“全新部署”路径

详见 `docs/stories/1.23-*.md`（测试、跨端任务、配置与回滚）。

### Story 1.24: 测试与安全门禁基座

As a 工程团队,
I want 自动化的契约、越权、重放与供应链门禁,
So that 防止回归和默认值/依赖风险再次进入主干.

*阶段：Phase 2｜需求：NFR-1,NFR-5,NFR-17｜架构：AD-18,AD-23｜依赖：1.2｜规模：M*

**Acceptance Criteria:**

**Given** 提交 PR
**When** CI 运行
**Then** 产出 SBOM、SCA、许可证扫描、secret 扫描并对高危阻断（含 UNLICENSED/Custom 商业许可）

**Given** 新增路由
**When** 运行生成式越权测试
**Then** 非属主、已移除成员、已拉黑、跨租户、过期令牌全部被拒

**Given** 修改 OpenAPI
**When** 运行 diff 门禁
**Then** 破坏性变更需升版本或 ADR

详见 `docs/stories/1.24-*.md`（测试、跨端任务、配置与回滚）。

## Epic 2: 完整的消息体验

用户可以发送并管理各种类型的消息——图片、语音、名片、链接，并撤回、引用、转发、删除；旧客户端遇到新类型也能优雅降级。

### Story 2.1: 内容类型注册表与未知类型降级

As a 客户端开发者,
I want 一个版本化的消息类型注册表，所有扩展消息都登记其 schema 与降级文案,
So that 新类型不会让旧客户端崩溃.

*阶段：Phase 3｜需求：FR-8｜架构：AD-4｜依赖：1.15｜规模：M*

**Acceptance Criteria:**

**Given** 新增消息类型
**When** 提交 PR
**Then** 缺少 schema、示例、各端渲染与降级文案任一项则 CI 失败

**Given** 旧版本客户端
**When** 收到未知类型
**Then** 显示“当前版本不支持该消息”而非崩溃或空白

**Given** 注册表更新
**When** 客户端启动
**Then** 能力探测返回其支持的类型版本

详见 `docs/stories/2.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 2.2: 文本、图片与链接预览

As a 终端用户,
I want 发送文本与最多 9 张图片，并看到链接预览,
So that 日常沟通的基本表达.

*阶段：Phase 3｜需求：FR-65,FR-6｜架构：AD-3,AD-6｜依赖：2.1｜规模：L*

**Acceptance Criteria:**

**Given** 发送图片
**When** 选择相册多图或拍照
**Then** 可选原图，上传有进度，失败可重试

**Given** 文本含链接
**When** 服务端取预览
**Then** 禁止内网、回环、链路本地地址，限制重定向与大小，超时 ≤5 秒

**Given** 文本含手机号
**When** 点击
**Then** 提示拨号或保存到通讯录

**Given** 文本消息含系统 emoji 或自定义表情图片
**When** 渲染
**Then** 各端一致显示（表情商店由 11.3/11.5 提供数据）

详见 `docs/stories/2.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 2.3: 语音消息与名片消息

As a 终端用户,
I want 长按录制语音并发送，分享联系人名片,
So that 更自然的沟通方式.

*阶段：Phase 3｜需求：FR-65｜架构：AD-6｜依赖：2.1｜规模：M*

**Acceptance Criteria:**

**Given** 长按录音
**When** 松手
**Then** 发送并显示时长；上滑取消不发送；权限被拒有引导

**Given** 选择联系人
**When** 发送名片
**Then** 接收方可查看资料并发起添加

**Given** 语音消息
**When** 播放
**Then** 显示进度并可用辅助技术操作

详见 `docs/stories/2.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 2.4: 消息撤回

As a 终端用户,
I want 在时限内撤回自己发送的消息，所有设备一致,
So that 发错内容可以补救.

*阶段：Phase 3｜需求：FR-10｜架构：AD-4｜依赖：1.16｜规模：M*

**Acceptance Criteria:**

**Given** 自己的消息在时限内（默认 3 分钟，可配置）
**When** 撤回
**Then** 所有设备含离线设备最终显示撤回提示

**Given** 消息已撤回
**When** 通过历史同步、搜索、预览读取
**Then** 服务端均不返回原内容；下载 URL 在 TTL（≤5 分钟）内失效，敏感内容走代理下载则立即失效

**Given** 超过时限
**When** 尝试撤回
**Then** 被拒绝并提示

详见 `docs/stories/2.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 2.5: 引用、转发、合并转发与删除范围

As a 终端用户,
I want 引用回复，逐条或合并转发，选择仅自己删除或双方删除,
So that 更高效地整理与分享沟通.

*阶段：Phase 3｜需求：FR-9｜架构：AD-3,AD-4｜依赖：2.4｜规模：M*

**Acceptance Criteria:**

**Given** 转发人无权访问原消息
**When** 转发
**Then** 被拒绝，合并转发中的原消息权限独立校验

**Given** 选择“对所有人删除”
**When** 超出时限或角色不允许
**Then** 被拒绝并写审计

**Given** 引用消息
**When** 原消息被撤回
**Then** 引用块显示“已撤回”而不泄露原文

详见 `docs/stories/2.5-*.md`（测试、跨端任务、配置与回滚）。

### Story 2.6: 输入状态、系统消息与历史分页

As a 终端用户,
I want 看到对方正在输入、系统通知，并分页浏览历史,
So that 会话更有临场感且历史可追溯.

*阶段：Phase 3｜需求：FR-11｜架构：AD-4｜依赖：1.16｜规模：M*

**Acceptance Criteria:**

**Given** 对方正在输入
**When** 单聊
**Then** 在会话中显示输入提示，隐私设置关闭时不发送

**Given** 群/单聊系统事件
**When** 发生
**Then** 生成系统消息（创建群、邀请、改名、加好友打招呼等）

**Given** 下拉历史
**When** 分页
**Then** 按序加载，不出现已无权限的消息

**Given** 向非好友发送消息
**When** 发送
**Then** 会话中提示需要先加好友或对方设置了限制（统一措辞，不泄露拉黑）

详见 `docs/stories/2.6-*.md`（测试、跨端任务、配置与回滚）。

### Story 2.7: 清空聊天记录与保留策略

As a 终端用户,
I want 清空本地会话记录，管理员按审批清理远程记录,
So that 隐私可控且合规可审计.

*阶段：Phase 3｜需求：FR-66｜架构：AD-4,AD-15｜依赖：2.4｜规模：S*

**Acceptance Criteria:**

**Given** 用户清空会话
**When** 确认
**Then** 服务端记录该用户的 cleared_before_seq，多设备同步隐藏历史（保留上游“清空聊天记录”语义，仅影响本人可见范围），其他成员不受影响

**Given** WuKongIM 无单条物理删除接口
**When** 需要物理清理
**Then** 能力边界写入文档：用墓碑/内容替换实现，物理清理走 9.1 的审批流程

**Given** 保留期到期
**When** worker 运行
**Then** 按策略清理且清理动作可审计

详见 `docs/stories/2.7-*.md`（测试、跨端任务、配置与回滚）。

## Epic 3: 联系人与社交关系

用户可以添加好友、管理黑名单、通过通讯录或二维码找到人，并用标签整理联系人。

### Story 3.1: 好友申请与新朋友

As a 终端用户,
I want 发起、接受或拒绝好友申请，并在“新朋友”里看到状态,
So that 建立社交关系.

*阶段：Phase 3｜需求：FR-12｜架构：AD-3｜依赖：1.15｜规模：M*

**Acceptance Criteria:**

**Given** 发送好友申请
**When** 对方接受
**Then** 双向好友关系一致并生成打招呼系统消息

**Given** 新申请
**When** 到达
**Then** 通讯录显示红点并可处理

**Given** 重复或超限申请
**When** 提交
**Then** 返回统一错误（被拉黑的统一表现由 3.2 保证）

详见 `docs/stories/3.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 3.2: 黑名单

As a 终端用户,
I want 拉黑或解除拉黑用户,
So that 避免骚扰.

*阶段：Phase 3｜需求：FR-12｜架构：AD-3,AD-21｜依赖：3.1｜规模：S*

**Acceptance Criteria:**

**Given** 用户 A 拉黑 B
**When** B 发消息
**Then** 经 IM 原生黑名单拒绝且 B 看到统一失败状态

**Given** 拉黑生效
**When** 涉及群邀请或动态可见性
**Then** 由 4.4 与 12.1 接入同一黑名单服务（本 Story 提供查询契约）

**Given** 解除拉黑
**When** 完成
**Then** 恢复沟通但不恢复已删除的好友关系

**Given** 被拉黑方
**When** 查看拉黑方资料
**Then** 不可查看（统一失败，不泄露拉黑）

详见 `docs/stories/3.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 3.3: 通讯录匹配、可搜索性与二维码

As a 终端用户,
I want 授权后用通讯录找到朋友，控制能否被手机号/短号搜到，扫码加好友或入群,
So that 更快找到人且隐私可控.

*阶段：Phase 3｜需求：FR-13,FR-11｜架构：AD-3,AD-16｜依赖：3.1｜规模：M*

**Acceptance Criteria:**

**Given** 用户授权通讯录
**When** 匹配
**Then** 使用带服务端密钥的 HMAC（非裸哈希）与限频接口，原始通讯录不落盘明文，防止对手机号空间的暴力还原

**Given** 关闭“手机号可被搜索”
**When** 他人按手机号搜索
**Then** 无结果且不泄露存在性

**Given** 二维码
**When** 过期或被撤销
**Then** 失效；群二维码入群的邀请确认由 4.4 实现

**Given** 用户拒绝通讯录权限
**When** 再次进入
**Then** 功能降级并给出设置引导，不循环弹窗

**Given** 扫描个人名片二维码
**When** 识别
**Then** 需用户确认后才发起添加

详见 `docs/stories/3.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 3.4: 联系人列表、个人名片与备注

As a 终端用户,
I want 按拼音浏览联系人，查看个人名片并设置备注,
So that 管理关系更方便.

*阶段：Phase 3｜需求：FR-12｜架构：AD-3｜依赖：3.1｜规模：M*

**Acceptance Criteria:**

**Given** 联系人
**When** 浏览
**Then** 按拼音分组排序，搜索可用

**Given** 个人名片
**When** 查看
**Then** 显示头像、昵称、备注、共同群组、来源，并受对方隐私设置约束

**Given** 修改备注
**When** 保存
**Then** 多设备同步

**Given** 联系人列表
**When** 浏览
**Then** 显示在线时长；名片显示入群方式与时间；官方与机器人账号分类展示（机器人由 16.7 提供）

详见 `docs/stories/3.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 3.5: 联系人标签

As a 终端用户,
I want 创建、修改、删除标签并批量整理联系人,
So that 分组管理联系人.

*阶段：Phase 4｜需求：FR-14｜架构：AD-3｜依赖：3.4｜规模：M*

**Acceptance Criteria:**

**Given** 创建标签
**When** 选择联系人
**Then** 标签仅本人可见并跨设备同步

**Given** 批量打标签
**When** 超过数量上限
**Then** 返回明确错误

**Given** 越权读取他人标签
**When** 直接调用接口
**Then** 被拒绝

详见 `docs/stories/3.5-*.md`（测试、跨端任务、配置与回滚）。

## Epic 4: 群聊与高级群管理

用户可以创建和管理群：邀请成员、设置公告、@成员，群主与管理员可以审核、禁言、控制历史可见性并撤回违规消息，所有管理动作可审计。

### Story 4.1: 创建群、邀请成员与群系统消息

As a 终端用户,
I want 创建群并邀请成员，群内有系统通知,
So that 多人沟通的基础.

*阶段：Phase 3｜需求：FR-64｜架构：AD-3｜依赖：1.15｜规模：M*

**Acceptance Criteria:**

**Given** 选择联系人
**When** 创建群
**Then** 创建者为群主并生成创建系统消息

**Given** 邀请成员
**When** 对方已被拉黑
**Then** 被拒；群黑名单见 4.4

**Given** 成员退出/被移除
**When** 完成
**Then** 系统消息生成且权限即时收回

**Given** 创建群
**When** 未设置群名与头像
**Then** 默认群名由成员昵称拼接，群头像由成员头像拼图

**Given** 群主退出
**When** 确认
**Then** 自动指定第二位入群成员为群主

详见 `docs/stories/4.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 4.2: 群公告、群名称与 @ 提醒

As a 终端用户,
I want 设置群公告与名称，@成员或 @所有人,
So that 重要信息触达群成员.

*阶段：Phase 3｜需求：FR-64｜架构：AD-3｜依赖：4.1｜规模：S*

**Acceptance Criteria:**

**Given** 群主或管理员
**When** 修改公告
**Then** 生成系统消息与未读提醒，普通成员无权修改

**Given** @所有人
**When** 超过频率或角色不允许
**Then** 被拒绝

**Given** 被 @ 成员免打扰
**When** 收到
**Then** 会话列表仍显示醒目标记

详见 `docs/stories/4.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 4.3: 群角色与权限

As a 群主,
I want 指派和撤销管理员，所有群管理动作由服务端校验,
So that 分担群管理.

*阶段：Phase 4｜需求：FR-15｜架构：AD-3,AD-15｜依赖：4.1｜规模：M*

**Acceptance Criteria:**

**Given** 绕过客户端直接调用群管理接口
**When** 非群主/管理员
**Then** 被拒绝并写审计

**Given** 转让群主
**When** 确认
**Then** 需二次确认并通知全体管理员

**Given** 角色变更
**When** 完成
**Then** 权限在 IM 与业务两侧一致

**Given** 群头像设置
**When** 群主或管理员修改
**Then** 校验权限并经内容审核（9.3）后生效

详见 `docs/stories/4.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 4.4: 邀请确认与群黑名单

As a 群主/管理员,
I want 开启邀请审核，并把成员加入群黑名单,
So that 控制谁能进群.

*阶段：Phase 4｜需求：FR-16｜架构：AD-3,AD-21｜依赖：4.3｜规模：M*

**Acceptance Criteria:**

**Given** 开启邀请确认
**When** 成员邀请他人
**Then** 进入待审核并通知管理员，待审核有过期时间

**Given** 群黑名单成员
**When** 通过邀请或二维码入群
**Then** 被拒绝

**Given** 审核通过
**When** 完成
**Then** 生成系统消息并同步 IM 订阅

**Given** 群二维码
**When** 扫码入群
**Then** 遵守邀请确认设置与群黑名单

**Given** 邀请
**When** 对方已被邀请人拉黑
**Then** 被拒绝（沿用 3.2 黑名单服务）

详见 `docs/stories/4.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 4.5: 禁言（全员、个人、时长）

As a 群主/管理员,
I want 全员禁言或对个人设置禁言时长,
So that 维持群秩序.

*阶段：Phase 4｜需求：FR-17｜架构：AD-3,AD-21｜依赖：4.3｜规模：M*

**Acceptance Criteria:**

**Given** 禁言期间
**When** 被禁言者发送
**Then** 在 IM 层（send_ban/before_send）被拒绝，界面显示剩余时长

**Given** 禁言到期
**When** 到期时间到达
**Then** 自动恢复且多设备一致

**Given** 管理员禁言管理员
**When** 角色不允许
**Then** 被拒绝

**Given** 点击成员头像
**When** 管理员
**Then** 可查看资料并单独设置禁言及时长

详见 `docs/stories/4.5-*.md`（测试、跨端任务、配置与回滚）。

### Story 4.6: 历史可见性与禁止互加好友

As a 群主,
I want 控制新成员能否看到入群前历史，并禁止群内互加好友,
So that 保护群内信息与隐私.

*阶段：Phase 4｜需求：FR-18｜架构：AD-3,AD-4｜依赖：4.3｜规模：S*

**Acceptance Criteria:**

**Given** 关闭历史可见
**When** 新成员拉取历史
**Then** 通过任何接口都取不到入群前消息

**Given** 开启禁止互加好友
**When** 从群发起好友申请
**Then** 入口隐藏且服务端拒绝

**Given** 切换设置
**When** 保存
**Then** 即时生效并写审计

详见 `docs/stories/4.6-*.md`（测试、跨端任务、配置与回滚）。

### Story 4.7: 撤回成员消息与群管理审计

As a 群主/管理员,
I want 撤回成员违规消息，并能查询管理操作记录,
So that 处置违规且可追溯.

*阶段：Phase 4｜需求：FR-19,FR-46｜架构：AD-3,AD-15｜依赖：2.4,4.3｜规模：M*

**Acceptance Criteria:**

**Given** 管理员撤回成员消息
**When** 完成
**Then** 在线成员 5 秒内收到撤回提示 [目标值待测]，离线设备重连后一致

**Given** 管理操作
**When** 发生
**Then** 审计记录含操作者、对象、时间、原因，普通管理员不可删除

**Given** 撤回后
**When** 他人搜索或下载
**Then** 原内容不可得

详见 `docs/stories/4.7-*.md`（测试、跨端任务、配置与回滚）。

### Story 4.8: 群设置页与成员列表

As a 终端用户,
I want 在群设置里查看成员、在线人数、保存的群聊与设置,
So that 一站式查看群信息.

*阶段：Phase 3｜需求：FR-64｜架构：AD-3｜依赖：4.2｜规模：S*

**Acceptance Criteria:**

**Given** 群设置
**When** 打开
**Then** 显示总成员数、在线人数、公告与自己的角色

**Given** 保存群聊
**When** 开启
**Then** 出现在通讯录“保存的群聊”

**Given** 大群
**When** 成员列表分页
**Then** 保持性能预算内并遵守隐私设置

**Given** 群备注与群内昵称
**When** 设置
**Then** 保存并可选择是否显示群昵称

详见 `docs/stories/4.8-*.md`（测试、跨端任务、配置与回滚）。

## Epic 5: 个人资料、隐私与账号安全

用户可以管理个人资料与隐私，查看并撤销设备，设置密码与二次验证，通过扫码登录桌面端，并能注销账号或导出数据。

### Story 5.1: 个人资料与头像

As a 终端用户,
I want 编辑头像、昵称、签名等资料,
So that 展示自己.

*阶段：Phase 3｜需求：FR-61｜架构：AD-3,AD-6｜依赖：1.12｜规模：S*

**Acceptance Criteria:**

**Given** 编辑资料
**When** 保存
**Then** 多设备同步，头像格式与大小经校验

**Given** 他人查看
**When** 访问
**Then** 仅返回对其可见的字段

**Given** 头像文件
**When** 上传
**Then** 校验魔数、格式与大小；违规内容的审核由 9.3 处理

**Given** 平台短号
**When** 修改
**Then** 只允许修改一次

详见 `docs/stories/5.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 5.2: 隐私设置

As a 终端用户,
I want 控制手机号/短号可搜索、在线状态、已读回执展示,
So that 隐私可控.

*阶段：Phase 4｜需求：FR-29｜架构：AD-3｜依赖：2.6,7.1｜规模：M*

**Acceptance Criteria:**

**Given** 关闭已读回执
**When** 保存
**Then** 偏好服务端强制并多设备同步，13.1 的已读详情与回执消费该偏好

**Given** 仅联系人可见在线状态
**When** 非联系人查询
**Then** 不泄露

**Given** 修改设置
**When** 保存
**Then** 服务端强制并多设备同步

详见 `docs/stories/5.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 5.3: 设备管理、设备锁与新设备批准流程

As a 终端用户,
I want 查看登录设备并下线，开启设备锁，并通过手机号验证或已信任设备批准新设备,
So that 账号安全可见可控.

*阶段：Phase 4｜需求：FR-3｜架构：AD-9｜依赖：1.13｜规模：M*

**Acceptance Criteria:**

**Given** 设备列表
**When** 打开
**Then** 显示当前设备、其他设备、位置、最近活跃与下线按钮

**Given** 开启设备锁
**When** 新设备登录
**Then** 需验证手机号验证码（对齐官方设备锁基线），或经已信任设备批准；批准请求经 IM 系统消息送达已信任设备（推送增强见 7.4），超时 4 分钟

**Given** 异地/新设备登录
**When** 发生
**Then** 向其他设备发提醒且不含敏感消息正文

**Given** 用户拒绝新设备
**When** 选择“不是我”
**Then** 请求作废并提示修改密码，写 login_event

详见 `docs/stories/5.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 5.4: 密码设置与重设

As a 终端用户,
I want 设置登录密码并通过验证码重设,
So that 多一种登录与找回方式.

*阶段：Phase 4｜需求：FR-2｜架构：AD-9｜依赖：1.12｜规模：S*

**Acceptance Criteria:**

**Given** 设置密码
**When** 保存
**Then** 以 Argon2id 存储

**Given** 验证码重设
**When** 完成
**Then** 其他设备会话全部失效

**Given** 连续失败
**When** 达阈值
**Then** 延迟与设备风控

详见 `docs/stories/5.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 5.5: 二次验证与 Passkey

As a 终端用户,
I want 启用 TOTP 或 Passkey 作为二次验证,
So that 更强的账号保护.

*阶段：Phase 4｜需求：FR-4｜架构：AD-9｜依赖：5.4｜规模：L*

**Acceptance Criteria:**

**Given** 启用二次验证
**When** 改密、注销或关闭设备锁
**Then** 必须二次验证

**Given** 丢失因子
**When** 走恢复流程
**Then** 书面化流程不能绕过设备锁

**Given** Passkey
**When** 登录
**Then** 使用 WebAuthn 并绑定设备

详见 `docs/stories/5.5-*.md`（测试、跨端任务、配置与回滚）。

### Story 5.6: 账号注销与数据导出

As a 终端用户,
I want 注销账号并申请导出我的数据,
So that 掌控自己的数据.

*阶段：Phase 4｜需求：FR-5｜架构：AD-3,AD-7｜依赖：5.4｜规模：L*

**Acceptance Criteria:**

**Given** 通过验证码注销
**When** 完成
**Then** 个人资料与好友关系销毁或匿名化，保留项在隐私政策列明

**Given** 导出申请
**When** worker 完成
**Then** 产物受签名下载保护且有过期时间

**Given** 注销后
**When** 他人访问
**Then** 不返回已注销用户的可识别资料

**Given** 已注销用户
**When** 他人尝试拉入群或搜索
**Then** 不可被拉入群并提示“该用户已注销”，搜索不返回

详见 `docs/stories/5.6-*.md`（测试、跨端任务、配置与回滚）。

### Story 5.7: Web/桌面扫码登录与设备批准

As a 终端用户,
I want 用手机扫码登录 Web/桌面，并由已信任设备批准新设备,
So that 电脑端快速且安全地登录.

*阶段：Phase 4｜需求：FR-67,FR-3｜架构：AD-9｜依赖：5.3｜规模：M*

**Acceptance Criteria:**

**Given** 二维码
**When** 扫码
**Then** 一次性、带过期，手机端显示设备与位置并要求确认

**Given** 扫码登录会话
**When** 未在手机端确认
**Then** 不能读取历史消息

**Given** 请求超时
**When** 过期
**Then** 可刷新二维码

详见 `docs/stories/5.7-*.md`（测试、跨端任务、配置与回滚）。

### Story 5.8: 通用设置：语言、深色模式、字体与通知偏好

As a 终端用户,
I want 切换语言与深浅色主题，调整字体与通知偏好,
So that 符合个人习惯.

*阶段：Phase 3｜需求：FR-61,NFR-8｜架构：AD-17｜依赖：5.1｜规模：M*

**Acceptance Criteria:**

**Given** 切换语言
**When** 保存
**Then** 无需重启，至少简体中文与英文，文案走 i18n 资源

**Given** 深色模式
**When** 默认
**Then** 跟随系统，可手动选择

**Given** 字体 200%
**When** 浏览关键页
**Then** 无裁切、无重叠

**Given** 聊天背景
**When** 设置与清除
**Then** 保存并多设备同步；清空缓存清除本地媒体缓存且不影响云端数据

详见 `docs/stories/5.8-*.md`（测试、跨端任务、配置与回滚）。

## Epic 6: 统一品牌与设计系统落地

所有客户端使用同一套品牌、令牌与原创图标库，桌面端有专属体验，视觉与无障碍有可回归的基线。

### Story 6.1: 设计令牌包与多端接入

As a 客户端开发者,
I want 一个由 tokens.json 生成的多端令牌包,
So that 各端颜色/字号/间距一致，不再各自为政.

*阶段：Phase 3｜需求：FR-68,NFR-8｜架构：AD-17｜依赖：1.24｜规模：M*

**Acceptance Criteria:**

**Given** 修改 tokens.json
**When** 构建
**Then** 生成 CSS、Android colors（含 night）、iOS Swift，且对比度脚本不通过则构建失败

**Given** 客户端使用令牌
**When** 评审
**Then** 不允许出现未登记的硬编码颜色（lint）

**Given** 渐变背景上的正文
**When** 使用
**Then** 只能使用 gradient.action

**Given** 任何端的界面
**When** 评审
**Then** 不得保留上游页面样式或仅替换 Logo/主题色，需对照设计规范

详见 `docs/stories/6.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 6.2: 原创图标库与 Emoji 门禁

As a 客户端开发者,
I want 导航、工具栏、Tab、后台菜单只用原创 SVG 图标库，并由门禁阻断 Emoji,
So that 品牌一致且满足“菜单不得用 Emoji”约束.

*阶段：Phase 3｜需求：FR-68｜架构：AD-17｜依赖：6.1｜规模：S*

**Acceptance Criteria:**

**Given** 图标库
**When** 构建
**Then** 生成单文件、雪碧图、选中态与 icons.json（中英文无障碍名称）

**Given** 导航源码含 Emoji
**When** 运行 tools/lint_no_emoji_nav.py
**Then** 退出码 1（含 router 与 menu 定义行）

**Given** 聊天内容与表情包数据
**When** 含 Emoji
**Then** 不受限制

详见 `docs/stories/6.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 6.3: 品牌资产集成

As a 客户端开发者,
I want 各端集成启动页、应用图标、通知图标与官网资产,
So that 品牌从第一眼开始一致.

*阶段：Phase 3｜需求：FR-69｜架构：AD-17｜依赖：6.1｜规模：M*

**Acceptance Criteria:**

**Given** Android
**When** 构建
**Then** 使用自适应图标三层、启动图标、状态栏通知图标

**Given** iOS
**When** 构建
**Then** 使用不透明 1024 图标及暗色/着色变体，并在真机验证着色

**Given** Web/PWA/桌面
**When** 构建
**Then** 使用 favicon、maskable、ICO/ICNS 与托盘图标

**Given** 矢量资产
**When** 生成
**Then** 由 SVG 源导出，禁止用位图放大冒充矢量

详见 `docs/stories/6.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 6.4: 桌面端壳与专属体验

As a 桌面用户,
I want 三栏布局、快捷键、托盘、文件拖拽与加固的 Electron 壳,
So that 桌面不是放大的手机页，且默认安全.

*阶段：Phase 3｜需求：FR-70,NFR-13｜架构：AD-17｜依赖：6.1｜规模：L*

**Acceptance Criteria:**

**Given** Electron 壳
**When** 审计
**Then** 禁用 nodeIntegration、启用 contextIsolation、IPC allowlist、外部链接校验、签名更新

**Given** 桌面端
**When** 使用
**Then** 提供 Ctrl/Cmd K 搜索、文件拖拽、系统通知、托盘、离线恢复

**Given** 升级 Electron/React
**When** 构建
**Then** 使用受支持的版本并通过依赖审计

详见 `docs/stories/6.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 6.5: 视觉回归基线与无障碍门禁

As a 工程团队,
I want 截图基线与无障碍自动化检查进入 CI,
So that 设计不被后续改动悄悄破坏.

*阶段：Phase 3｜需求：FR-69,NFR-8,NFR-17｜架构：AD-23｜依赖：6.1｜规模：M*

**Acceptance Criteria:**

**Given** 关键屏幕
**When** CI 运行
**Then** 与 360/390 手机、平板、桌面、明暗、中英、200% 字号基线比对

**Given** 页面
**When** 运行 a11y_scan
**Then** axe、键盘焦点、目标尺寸、裁切、回流检查全部通过

**Given** 新增界面
**When** 提交
**Then** 必须附 WCAG 清单对应条目的证据

详见 `docs/stories/6.5-*.md`（测试、跨端任务、配置与回滚）。

## Epic 7: 会话列表、搜索与通知

用户可以高效管理会话（置顶、免打扰、红点、状态），在本地搜索聊天记录，并收到不泄露敏感内容、多设备不重复的推送。

### Story 7.1: 会话列表与状态

As a 终端用户,
I want 会话列表显示头像、时间、预览、红点、置顶、免打扰、发送状态与网络状态,
So that 快速掌握所有会话.

*阶段：Phase 3｜需求：FR-7,FR-61｜架构：AD-4,AD-17｜依赖：1.16｜规模：L*

**Acceptance Criteria:**

**Given** 未读超过 100
**When** 显示
**Then** 红点显示 99+，免打扰会话以图标而非仅变灰表示

**Given** 草稿
**When** 在会话列表
**Then** 显示草稿标记并跨设备同步

**Given** 无网络或未连接
**When** 打开
**Then** 顶部显示离线条与队列中的发送

**Given** 长按会话
**When** 菜单
**Then** 提供置顶、免打扰、删除聊天；置顶与免打扰跨设备同步

**Given** 基础在线状态
**When** 单聊与群聊
**Then** 单聊显示在线或一小时内最近在线时长，群标题显示总人数与在线人数（遵守隐私默认；13.3 提供隐私控制与细化）

**Given** 官方/系统账号
**When** 出现在会话
**Then** 显示官方标识（账号实体由 9.4 配置）

**Given** 置顶或免打扰变更
**When** 保存
**Then** 在其他设备 5 秒内一致 [目标值待测]

详见 `docs/stories/7.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 7.2: 全局搜索与客户端本地消息搜索

As a 终端用户,
I want 搜索联系人、群、文件，并在本地已同步的聊天记录里搜索关键词,
So that 快速找回信息，且服务端不保存消息正文索引.

*阶段：Phase 3｜需求：FR-41｜架构：AD-19｜依赖：2.4,3.4,4.1｜规模：M*

**Acceptance Criteria:**

**Given** 搜索联系人/群
**When** 查询
**Then** 结果受成员资格与可见性约束（文件搜索由 10.3 启用）

**Given** 关键词搜索
**When** 在客户端执行
**Then** 仅搜索本地已同步历史，被撤回或无权限内容不出现

**Given** 服务端索引
**When** 未通过隐私评审
**Then** 不创建（ADR-0010）

**Given** 本地索引
**When** 用户登出或注销
**Then** 索引被擦除；索引在设备上加密存储且用户可手动清除

详见 `docs/stories/7.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 7.3: 会话媒体中心

As a 终端用户,
I want 查看会话内图片、视频、文件、链接的汇总,
So that 集中找回资料.

*阶段：Phase 3｜需求：FR-41｜架构：AD-3,AD-6｜依赖：2.2｜规模：M*

**Acceptance Criteria:**

**Given** 会话
**When** 打开媒体中心
**Then** 先展示图片与链接，视频与文件由 11.2 与 10.3 交付后接入；内容均受成员资格与撤回约束

**Given** 文件下载
**When** 点击
**Then** 使用短时签名 URL

**Given** 被移出群
**When** 访问
**Then** 在缓存与 URL 的 TTL 内不可见

详见 `docs/stories/7.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 7.4: 推送通道（APNs/FCM）与内容脱敏

As a 终端用户,
I want 通过 APNs、FCM 与厂商通道收到通知，正文可脱敏,
So that 不错过消息且不泄露隐私.

*阶段：Phase 3｜需求：FR-42｜架构：AD-7｜依赖：1.15,1.5｜规模：L*

**Acceptance Criteria:**

**Given** 推送
**When** 发送
**Then** 默认正文可配置为“你有一条新消息”，不含敏感内容

**Given** 通道
**When** 设备注册
**Then** 按设备类别选择 APNs 或 FCM，密钥不入源码；国内厂商通道见 7.6

**Given** 推送失败
**When** 重试
**Then** 退避后记录失败率并告警

详见 `docs/stories/7.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 7.5: 通知偏好与多设备去重

As a 终端用户,
I want 设置免打扰与通知偏好，在一台设备已读后其他设备清除通知,
So that 通知不打扰也不重复.

*阶段：Phase 3｜需求：FR-42｜架构：AD-7｜依赖：7.4｜规模：M*

**Acceptance Criteria:**

**Given** 免打扰
**When** 收到消息
**Then** 不推送，但 @ 强提醒按偏好

**Given** 多设备
**When** 其中一台已读
**Then** 其他设备清除对应通知

**Given** 静默推送
**When** 到达
**Then** 仅更新角标与同步

详见 `docs/stories/7.5-*.md`（测试、跨端任务、配置与回滚）。

### Story 7.6: 国内厂商推送通道与 Web Push

As a 终端用户,
I want 在华为、小米、OPPO、VIVO 手机和浏览器上收到通知,
So that 国内 Android 设备也能可靠收到消息.

*阶段：Phase 3｜需求：FR-42｜架构：AD-7｜依赖：7.4｜规模：M*

**Acceptance Criteria:**

**Given** 厂商设备
**When** 注册
**Then** 使用对应厂商通道，密钥通过受保护配置注入，不入源码

**Given** 通道不可用
**When** 发送
**Then** 回退到 FCM 或应用内同步并记录失败率

**Given** 浏览器
**When** 用户授权
**Then** 支持 Web Push，内容可脱敏

详见 `docs/stories/7.6-*.md`（测试、跨端任务、配置与回滚）。

## Epic 8: 举报、内容安全与风控

用户可以举报并申诉，平台能检测垃圾、危险链接与文件并限速，真实 IP 只信任可信代理。

### Story 8.1: 举报与申诉

As a 终端用户,
I want 举报用户、群、消息，并对处置申诉（动态举报见 12.4）,
So that 有渠道对抗滥用，也有救济.

*阶段：Phase 3｜需求：FR-43｜架构：AD-3,AD-15｜依赖：1.19｜规模：M*

**Acceptance Criteria:**

**Given** 举报
**When** 提交
**Then** 进入后台工单并可追踪状态

**Given** 被处置用户
**When** 申诉
**Then** 由与处置人不同的审核员处理

**Given** 处置完成
**When** 通知
**Then** 双方收到结果且写审计

**Given** 群设置与用户资料页
**When** 点击投诉
**Then** 可投诉本群或该用户并进入举报队列（9.3 通用化队列）

详见 `docs/stories/8.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 8.2: 反垃圾、限速与风控事件

As a 平台运营,
I want 策略可配置的限速与风控规则，命中写事件可复核,
So that 压制群发与异常登录.

*阶段：Phase 3｜需求：FR-44｜架构：AD-7,AD-16｜依赖：1.12｜规模：M*

**Acceptance Criteria:**

**Given** 异常行为
**When** 命中规则
**Then** 限速或拦截并写 risk_event，可在后台复核与解除

**Given** 误伤
**When** 解除
**Then** 恢复并记录原因

**Given** 规则变更
**When** 保存
**Then** 版本化并可回滚

详见 `docs/stories/8.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 8.3: 恶意链接与危险文件检测

As a 终端用户,
I want 消息里的恶意链接与危险文件被检测并标记,
So that 降低钓鱼与恶意软件风险.

*阶段：Phase 3｜需求：FR-44,FR-23｜架构：AD-6｜依赖：8.2｜规模：M*

**Acceptance Criteria:**

**Given** 链接
**When** 命中恶意库
**Then** 显示风险提示或拦截

**Given** 扫描适配器
**When** 提供接口
**Then** 10.3 与 11.2 复用它；用测试夹具验证危险扩展名与魔数不符会被判隔离（上传入口在 10.3）

**Given** 扫描引擎不可用
**When** 上传
**Then** 文件保持“待检”不默认放行

详见 `docs/stories/8.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 8.4: 可信代理与真实客户端 IP

As a 平台运营,
I want 只信任可信代理列表中的转发头来取得真实 IP,
So that 防止伪造 IP 绕过限速.

*阶段：Phase 3｜需求：FR-44,NFR-16｜架构：AD-16｜依赖：1.1｜规模：S*

**Acceptance Criteria:**

**Given** 请求来自非可信代理
**When** 带 X-Forwarded-For
**Then** 忽略并使用 socket 地址

**Given** WuKongIM PROXY 协议
**When** 启用
**Then** 仅接受 Nginx 网段

**Given** CDN 变更
**When** 更新可信 CIDR
**Then** 热加载并写审计

详见 `docs/stories/8.4-*.md`（测试、跨端任务、配置与回滚）。

## Epic 9: 管理后台商业运营能力

运营、风控、审核可以在最小权限下处理用户、群、消息、举报、配置与报表，高危动作需审批，所有操作可追溯。

### Story 9.1: 后台权限与审批流

As a 平台运营,
I want 完整的角色权限与高危动作审批,
So that 最小权限，避免单人滥权.

*阶段：Phase 3｜需求：FR-45,FR-46｜架构：AD-3,AD-15,AD-24｜依赖：1.19｜规模：L*

**Acceptance Criteria:**

**Given** 角色
**When** 分配权限
**Then** 以资源+动作授权，默认最小，变更写审计

**Given** 封号/导出/改配置/清理远程记录
**When** 发起
**Then** 进入审批并有理由字段

**Given** 管理员“代发消息/冒用身份”
**When** 默认
**Then** 关闭，开启需审批并在会话留标识

**Given** 管理员物理清理远程记录或删除消息
**When** 发起
**Then** 需要审批、理由并写审计

**Given** 删除用户消息
**When** 管理员发起
**Then** 需审批并写审计（FR-46）

详见 `docs/stories/9.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 9.2: 用户/设备/群/消息索引查询

As a 平台运营,
I want 在受限条件下按用户、设备、群、消息索引查询,
So that 处理问题所需的最小可见.

*阶段：Phase 3｜需求：FR-72｜架构：AD-3,AD-15｜依赖：9.1｜规模：L*

**Acceptance Criteria:**

**Given** 查询敏感信息
**When** 提交
**Then** 默认脱敏，查看明文或聊天记录需理由并留痕

**Given** 导出
**When** 发起
**Then** 需审批

**Given** 查询聊天内容
**When** 目标会话
**Then** 仅在授权范围内且遵守保留策略

详见 `docs/stories/9.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 9.3: 内容审核队列

As a 内容审核员,
I want 统一的待审队列处理举报内容、头像、动态、表情,
So that 高效一致的审核.

*阶段：Phase 3｜需求：FR-72,FR-43｜架构：AD-3｜依赖：8.1,9.1｜规模：M*

**Acceptance Criteria:**

**Given** 待审内容
**When** 审核
**Then** 通过、下架或升级，结果写审计并通知

**Given** 多审核员
**When** 并发
**Then** 单条内容不被重复处理

**Given** 申诉
**When** 复核
**Then** 与首审不同人

**Given** 用户头像与群头像
**When** 命中规则或被举报
**Then** 进入审核队列，违规被替换并通知

详见 `docs/stories/9.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 9.4: 运营配置、版本更新与公告

As a 平台运营,
I want 功能开关、下载链接、版本更新、公告可在后台配置,
So that 无需发版即可运营.

*阶段：Phase 3｜需求：FR-47,FR-57｜架构：AD-3｜依赖：9.1｜规模：M*

**Acceptance Criteria:**

**Given** 配置变更
**When** 保存
**Then** 生成版本历史并可回滚

**Given** 版本配置
**When** 保存
**Then** 保存最低版本、强制更新开关与灰度比例字段（客户端行为见 19.2，发布演练见 20.6）

**Given** 页面
**When** 加载失败或无数据
**Then** 有 Loading/Empty/Error 状态

**Given** 官方/系统账号
**When** 配置
**Then** 在后台创建系统账号与官方标识，客户端据此显示

**Given** 客户端“关于/检查新版本”
**When** 点击
**Then** 读取版本配置并提示（升级行为见 19.2）

详见 `docs/stories/9.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 9.5: 数据报表与统计口径

As a 平台运营,
I want 查看关键运营指标，口径文档化且不含个人数据,
So that 用数据决策而不泄露隐私.

*阶段：Phase 3｜需求：FR-72｜架构：AD-15｜依赖：9.1｜规模：M*

**Acceptance Criteria:**

**Given** 报表
**When** 查看
**Then** 指标口径可查，聚合数据不含个人级信息

**Given** 导出报表
**When** 发起
**Then** 需审批并留痕

**Given** 大数据量
**When** 查询
**Then** 异步生成不拖垮在线库

详见 `docs/stories/9.5-*.md`（测试、跨端任务、配置与回滚）。

### Story 9.6: 后台全新界面与状态

As a 平台运营,
I want 全新企业风格的后台：概览、列表、详情抽屉与完整状态,
So that 高效、一致、可访问的运营体验.

*阶段：Phase 3｜需求：FR-47,FR-68｜架构：AD-17,AD-24｜依赖：6.1,9.1｜规模：L*

**Acceptance Criteria:**

**Given** 每个后台页面
**When** 渲染
**Then** 具备加载、空、错误、无权限状态与真实数据动作

**Given** 键盘
**When** 操作
**Then** 全部功能可达，焦点环可见

**Given** 1024–1440 宽度
**When** 浏览
**Then** 无横向滚动与裁切

详见 `docs/stories/9.6-*.md`（测试、跨端任务、配置与回滚）。

### Story 9.7: IM运维入口（WuKongIM 后台经管理后台鉴权代理）

As a 平台运维,
I want 在管理后台“运维”下进入 WuKongIM 自带后台，免二次登录且仅限有权限的管理员,
So that 统一登录、权限与审计，同时保持 WuKongIM 管理端口不对外.

*阶段：Phase 3｜需求：FR-45,FR-46｜架构：AD-15,AD-16｜依赖：1.16,9.1｜规模：M*

**Acceptance Criteria:**

**Given** 具备 ops.im.view 的管理员且已通过 MFA
**When** 点击“运维-IM运维”
**Then** 直接进入 WuKongIM 后台且不出现其登录页，并写审计（谁、何时、角色级别）

**Given** 无该权限、会话失效或未通过 MFA 的请求
**When** 访问 IM 运维路径
**Then** 一律 401/403，不向 WuKongIM 转发任何请求

**Given** 外部网络
**When** 直接访问 WuKongIM 管理端口、后台路径或带伪造 Authorization 的请求
**Then** 不可达或被剥离，暴露面闸门含此负面用例

**Given** ops.im.view 与 ops.im.admin 两个级别
**When** 分别进入
**Then** 分别映射到 WuKongIM 只读与可写管理用户，只读用户的写操作被 WuKongIM 拒绝

**Given** 管理员权限被撤销或会话结束
**When** 已打开的 IM 运维页继续请求
**Then** 在会话有效期内最迟下一次请求即被拒绝，代理签发的 WuKongIM 令牌有效期短且不落入浏览器持久存储

详见 `docs/stories/9.7-*.md`（测试、跨端任务、配置与回滚）。

## Epic 10: 收藏、位置与文件

用户可以收藏消息、发送位置、发送并预览文件，文件有类型校验、病毒扫描与签名下载。

### Story 10.1: 消息收藏与我的收藏

As a 终端用户,
I want 收藏文本/图片消息，管理我的收藏并转发给朋友,
So that 保存重要信息.

*阶段：Phase 4｜需求：FR-20,FR-20｜架构：AD-3,AD-4｜依赖：2.4｜规模：M*

**Acceptance Criteria:**

**Given** 消息菜单
**When** 收藏
**Then** 进入我的收藏，含作者、昵称、收藏时间

**Given** 收藏列表
**When** 复制文本、缩放图片、转发给朋友、删除
**Then** 各操作可用且跨设备同步

**Given** 源消息被撤回
**When** 查看收藏副本
**Then** 按策略保留或标记且有书面说明

**Given** 收藏删除
**When** 同步
**Then** 多设备在 10 秒内一致 [目标值待测]

详见 `docs/stories/10.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 10.2: 位置消息

As a 终端用户,
I want 发送当前位置或搜索选点，对方查看位置卡片并跳转地图,
So that 快速分享位置.

*阶段：Phase 4｜需求：FR-21｜架构：AD-3｜依赖：2.1｜规模：M*

**Acceptance Criteria:**

**Given** 发送位置
**When** 选择当前位置或搜索地点
**Then** 仅在用户操作时申请前台定位，不后台定位

**Given** 位置卡片
**When** 对方查看
**Then** 显示截图缩略图并可跳转地图

**Given** 缺少地图 Provider 密钥
**When** 入口
**Then** 隐藏而非崩溃

详见 `docs/stories/10.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 10.3: 文件上传、扫描门、预览、转发与签名下载

As a 终端用户,
I want 发送文件并预览常见类型，接收方下载与转发,
So that 协作传递文件.

*阶段：Phase 4｜需求：FR-22,FR-62｜架构：AD-6｜依赖：2.2,8.3｜规模：L*

**Acceptance Criteria:**

**Given** 上传
**When** 校验
**Then** 经 MIME+魔数+扩展名三重校验，危险类型被拒，大小/频率超限返回明确错误

**Given** 下载
**When** 点击
**Then** 使用带过期的签名 URL，成员被移除后在签名 URL 的 TTL（≤5 分钟）内失效，敏感内容改走 API 代理下载则立即失效

**Given** 大文件
**When** 网络中断
**Then** 支持断点续传

**Given** 文件上传
**When** 状态机运行
**Then** uploaded→scanning→clean|quarantined，仅 clean 可下载，扫描引擎不可用时保持 scanning；使用 8.3 提供的扫描适配器

**Given** 会话搜索与存储
**When** 切换提供方
**Then** 业务代码不变（对象存储端口），图片缩略图/变换由服务端生成；搜索结果包含文件（供 7.2 使用）

详见 `docs/stories/10.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 10.4: 文件隔离复核与存储生命周期

As a 平台运营,
I want 文件上传后经病毒扫描，命中的隔离，过期对象按策略清理,
So that 防止恶意文件传播并控制存储成本.

*阶段：Phase 4｜需求：FR-23,FR-62｜架构：AD-6,AD-7｜依赖：10.3,8.3｜规模：M*

**Acceptance Criteria:**

**Given** 命中隔离
**When** 后台复核
**Then** 仅见元数据，误隔离可申请复核并恢复

**Given** 命中隔离
**When** 通知
**Then** 发送者收到提示，后台仅见元数据

**Given** 会话被删除或文件到期
**When** 到期
**Then** 关联对象在期限内被清理且旧 URL 在 TTL 内失效

详见 `docs/stories/10.4-*.md`（测试、跨端任务、配置与回滚）。

## Epic 11: 小视频与表情商店

用户可以录制或选择小视频发送，使用表情面板与自定义表情，并从表情商店添加官方表情包。

### Story 11.1: 小视频录制、选择与上传

As a 终端用户,
I want 录制 15 秒（可配置）或选择相册视频并发送,
So that 更丰富的表达.

*阶段：Phase 4｜需求：FR-24｜架构：AD-6｜依赖：10.3｜规模：M*

**Acceptance Criteria:**

**Given** 录制
**When** 达到上限
**Then** 自动停止并预览，显示大小

**Given** 上传
**When** 进行
**Then** 显示进度，失败可重试

**Given** 权限被拒
**When** 进入
**Then** 给出引导

详见 `docs/stories/11.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 11.2: 小视频转码、封面、播放与转发

As a 终端用户,
I want 视频自动转码并生成封面，可播放、保存、转发,
So that 各端都能顺畅观看.

*阶段：Phase 4｜需求：FR-24｜架构：AD-6,AD-7｜依赖：11.1｜规模：L*

**Acceptance Criteria:**

**Given** 视频上传
**When** 转码
**Then** 生成封面与适配码率，失败时发送者看到可重试状态

**Given** 未下载视频
**When** 点击
**Then** 显示下载进度后播放

**Given** 编解码
**When** 上线
**Then** 许可与版权在 license-and-ip 登记（OQ-6）

**Given** 会话媒体中心
**When** 有视频
**Then** 视频接入 7.3 的媒体中心

详见 `docs/stories/11.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 11.3: 表情面板与收藏分类

As a 终端用户,
I want 在消息面板切换 Emoji、收藏、表情包并搜索,
So that 快速找到表情.

*阶段：Phase 4｜需求：FR-25｜架构：AD-3｜依赖：2.1｜规模：M*

**Acceptance Criteria:**

**Given** 表情面板
**When** 打开
**Then** 显示搜索入口、Emoji 分类、收藏分类（首项为“制作表情”）

**Given** 收藏分类
**When** 查看
**Then** 显示用户收藏的表情

**Given** 旧客户端
**When** 收到动图
**Then** 降级为静态图

详见 `docs/stories/11.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 11.4: 自定义表情制作与整理

As a 终端用户,
I want 从相册制作表情并排序、删除,
So that 个性化表达.

*阶段：Phase 4｜需求：FR-25｜架构：AD-6｜依赖：11.3,9.3｜规模：M*

**Acceptance Criteria:**

**Given** 选择图片
**When** 保存制作
**Then** 自定义表情需经内容审核后才可被他人使用

**Given** 整理
**When** 排序或删除
**Then** 多设备同步，提供非拖拽的上移/下移替代控件

**Given** 违规表情
**When** 下架
**Then** 对所有人立即不可见

详见 `docs/stories/11.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 11.5: 平台表情包商店

As a 终端用户,
I want 在表情商店搜索、查看详情并添加或移除表情包，管理员维护表情包,
So that 丰富的官方表情内容.

*阶段：Phase 4｜需求：FR-25｜架构：AD-3｜依赖：11.3,9.6｜规模：L*

**Acceptance Criteria:**

**Given** 商店
**When** 搜索与浏览
**Then** 展示列表与详情，可添加、移除、排序

**Given** 管理员上传表情包
**When** 发布
**Then** 登记版权来源与授权期限，到期自动下架

**Given** 用户分享表情包
**When** 接收方打开
**Then** 可添加

详见 `docs/stories/11.5-*.md`（测试、跨端任务、配置与回滚）。

## Epic 12: 朋友圈（动态）

用户可以发布图文/视频动态，用公开、私有、部分可见、不给谁看控制可见性，好友可评论点赞，违规内容可举报并审核。

### Story 12.1: 发布动态与可见性规则

As a 终端用户,
I want 发布文本/最多 9 图/单视频，选择位置、提醒谁看，并设置可见性,
So that 分享生活且可控范围.

*阶段：Phase 4｜需求：FR-26｜架构：AD-3,AD-6｜依赖：10.3,3.5,10.2,11.2,7.4｜规模：L*

**Acceptance Criteria:**

**Given** 发布
**When** 选择可见性（公开/私有/部分可见/不给谁看）并从通讯录或标签选人
**Then** 规则按当前好友与标签关系计算

**Given** 被排除者
**When** 通过 ID 直接访问动态、点赞、评论、媒体 URL
**Then** 全部被拒绝

**Given** 9 图
**When** 排序
**Then** 可拖拽也可用前移/后移按钮；上传显示进度与结果

**Given** 黑名单关系
**When** 计算可见性
**Then** 同步生效（沿用 3.2 黑名单服务）

详见 `docs/stories/12.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 12.2: 动态列表与相册封面

As a 终端用户,
I want 浏览好友动态，查看发布者的朋友圈，设置相册封面,
So that 关注好友近况.

*阶段：Phase 4｜需求：FR-27｜架构：AD-3｜依赖：12.1,10.1｜规模：M*

**Acceptance Criteria:**

**Given** 动态列表
**When** 浏览
**Then** 显示发布者备注名优先、头像、时间、地址与可见范围内的内容

**Given** 点击头像
**When** 查看他人朋友圈
**Then** 仅显示对自己可见的动态

**Given** 相册封面
**When** 设置
**Then** 从相册或拍摄选择并同步

**Given** 动态中的文本与图片
**When** 长按
**Then** 可复制、收藏（进入 10.1 的我的收藏）、转发或保存

详见 `docs/stories/12.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 12.3: 评论、点赞、提醒与新消息入口

As a 终端用户,
I want 评论点赞好友动态并被提醒，从新消息入口查看,
So that 互动与触达.

*阶段：Phase 4｜需求：FR-27｜架构：AD-3,AD-7｜依赖：12.2,7.4｜规模：M*

**Acceptance Criteria:**

**Given** 动态可见
**When** 评论或点赞
**Then** 只有互为好友且可见的人能看到彼此的互动

**Given** 被提醒
**When** 通知
**Then** 收到提醒并可进入动态

**Given** 删除动态
**When** 完成
**Then** 评论、点赞、缓存与通知同步清除

详见 `docs/stories/12.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 12.4: 动态举报、审核与清理

As a 平台运营,
I want 举报动态，后台审核下架，缓存与通知同步清理,
So that 内容安全.

*阶段：Phase 4｜需求：FR-28,FR-43｜架构：AD-3,AD-15｜依赖：12.1,8.1｜规模：M*

**Acceptance Criteria:**

**Given** 举报动态
**When** 提交
**Then** 进入审核队列并可追踪

**Given** 下架
**When** 完成
**Then** 对所有人在列表与接口层立即不可见并写审计，媒体 URL 在 TTL（≤5 分钟）内失效，缓存与通知被清理

**Given** 申诉
**When** 提交
**Then** 由不同审核员复核

详见 `docs/stories/12.4-*.md`（测试、跨端任务、配置与回滚）。

## Epic 13: 旗舰消息能力

用户可以看到已读/未读详情、编辑消息、给消息点赞、看到在线状态、编辑图片、按日期/成员/类型查找历史，并在系统支持时收到截屏与撤回通知。

### Story 13.1: 已读未读与成员详情

As a 终端用户,
I want 消息显示已读未读标记，并查看已读/未读成员列表，可开关,
So that 知道对方是否看到了.

*阶段：Phase 4｜需求：FR-30｜架构：AD-3,AD-4｜依赖：5.2,4.1｜规模：L*

**Acceptance Criteria:**

**Given** 关闭已读回执的成员
**When** 阅读消息
**Then** 不产生对其他人可见的已读状态，也不能反推

**Given** 大群
**When** 查看已读详情
**Then** 按成员上限分页，超限有降级说明

**Given** 已读详情
**When** 打开
**Then** 显示已读/未读/点赞成员及时间，仅包含已开启回执的成员

详见 `docs/stories/13.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 13.2: 消息编辑与版本

As a 终端用户,
I want 编辑已发送的消息并显示“已编辑”，保留版本策略,
So that 纠正错误而不丢失上下文.

*阶段：Phase 4｜需求：FR-10｜架构：AD-4｜依赖：2.4｜规模：M*

**Acceptance Criteria:**

**Given** 编辑
**When** 在时限内保存
**Then** 所有设备含离线设备最终一致，界面标注“已编辑”

**Given** 版本策略
**When** 配置
**Then** 保留全部版本或仅最新（可配置）

**Given** 被撤回消息
**When** 尝试编辑
**Then** 被拒绝

详见 `docs/stories/13.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 13.3: 消息点赞与在线状态

As a 终端用户,
I want 给消息点赞并查看点赞列表，查看对方在线状态与最近在线,
So that 更轻量的互动与临场感.

*阶段：Phase 4｜需求：FR-31｜架构：AD-3｜依赖：2.6,7.1｜规模：M*

**Acceptance Criteria:**

**Given** 点赞
**When** 长按消息
**Then** 显示点赞内容与点赞列表

**Given** 在线状态
**When** 对方关闭展示
**Then** 不泄露在线与最近在线

**Given** 在线状态
**When** 单聊与会话列表
**Then** 一小时内显示时长，群聊显示在线人数

详见 `docs/stories/13.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 13.4: 图片编辑

As a 终端用户,
I want 发送图片前涂鸦、裁剪、加文字、加滤镜,
So that 发送前完成简单处理.

*阶段：Phase 4｜需求：FR-32｜架构：AD-6｜依赖：2.2｜规模：M*

**Acceptance Criteria:**

**Given** 编辑图片
**When** 涂鸦/裁剪/文字/滤镜
**Then** 在本地完成并可撤销

**Given** 发送
**When** 选择移除位置元数据
**Then** EXIF 位置被剥离

**Given** 低端设备
**When** 编辑大图
**Then** 有降采样与内存保护

**Given** 上传的图片
**When** 服务端处理
**Then** 服务端也剥离 EXIF 位置等元数据

详见 `docs/stories/13.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 13.5: 会话内查找（日期/成员/类型）与定位跳转

As a 终端用户,
I want 在会话内按日期、成员、类型查找历史（关键词在本地）,
So that 快速定位历史.

*阶段：Phase 3｜需求：FR-33｜架构：AD-19｜依赖：7.2,2.6｜规模：M*

**Acceptance Criteria:**

**Given** 按日期/成员/类型
**When** 查询
**Then** 通过 IM 消息同步与 MySQL 元数据，并按可见性过滤

**Given** 被移出成员或已撤回内容
**When** 查询
**Then** 不出现

**Given** 关键词
**When** 查询
**Then** 仅在本地已同步历史执行

**Given** 查找结果
**When** 点击
**Then** 跳转到原消息位置并高亮提醒

详见 `docs/stories/13.5-*.md`（测试、跨端任务、配置与回滚）。

### Story 13.6: 截屏通知与撤回通知

As a 终端用户,
I want 在系统支持时发送截屏通知，撤回时显示撤回提示,
So that 知情与透明.

*阶段：Phase 3｜需求：FR-34,FR-34｜架构：AD-3｜依赖：2.4｜规模：S*

**Acceptance Criteria:**

**Given** 平台支持截屏检测
**When** 截屏
**Then** 在会话显示系统通知（可由设置开关）

**Given** 平台不支持
**When** 设置页
**Then** 隐藏该选项并说明替代交互

**Given** 界面文案
**When** 展示
**Then** 明确“并非所有平台/场景可检测截屏”

**Given** 敏感会话
**When** 启用防截屏（拦截）
**Then** Android 使用 FLAG_SECURE、iOS 使用安全视图；平台不支持处明确声明不提供拦截

详见 `docs/stories/13.6-*.md`（测试、跨端任务、配置与回滚）。

## Epic 14: 客服系统

访客与用户可以联系客服，系统按技能组自动分配，客服可转接、使用快捷回复与知识库并创建工单，主管有 SLA、质检与绩效视图。

### Story 14.1: 应用内客服入口、技能组与自动分配

As a 访客/用户,
I want 从通讯录进入专属客服咨询并自动匹配合适客服,
So that 快速得到帮助.

*阶段：Phase 4｜需求：FR-38｜架构：AD-3,AD-10｜依赖：1.15,9.1｜规模：L*

**Acceptance Criteria:**

**Given** 通讯录内专属客服入口
**When** 登录用户咨询
**Then** 按技能组与负载自动分配，客服与用户身份有明确标记

**Given** 非工作时间
**When** 咨询
**Then** 留言转工单并在次日分派

**Given** 登录用户会话
**When** 查看
**Then** 会话与技能组隔离，仅暴露最小资料（官网匿名访客入口见 19.3）

详见 `docs/stories/14.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 14.2: 客服工作台、转接、快捷回复与工单

As a 客服,
I want 处理会话、转接、使用快捷回复、创建工单，并设置欢迎词与查看访客资料,
So that 高效解决问题.

*阶段：Phase 4｜需求：FR-39,FR-38｜架构：AD-3,AD-15｜依赖：14.1｜规模：L*

**Acceptance Criteria:**

**Given** 转接
**When** 完成
**Then** 保留上下文与权限边界，跨技能组不泄露

**Given** 欢迎词
**When** 访客发消息
**Then** 自动发送客服欢迎词，并显示访客资料详情与用户标签

**Given** 创建工单
**When** 保存
**Then** 会话与工单关联并可追踪

**Given** 主管
**When** 需要
**Then** 可手动分配与改派会话并写审计

详见 `docs/stories/14.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 14.3: SLA、知识库、质检与绩效

As a 客服主管,
I want 监控 SLA，维护知识库，抽检质检并查看绩效,
So that 提升服务质量.

*阶段：Phase 4｜需求：FR-39,FR-40｜架构：AD-15｜依赖：14.2｜规模：L*

**Acceptance Criteria:**

**Given** SLA 临界
**When** 接近超时
**Then** 主管收到提醒并可介入

**Given** 质检与导出
**When** 操作
**Then** 写审计，客服只能访问其技能组范围

**Given** 绩效报表
**When** 导出
**Then** 脱敏且需审批

详见 `docs/stories/14.3-*.md`（测试、跨端任务、配置与回滚）。

## Epic 15: 音视频通话与会议

用户可以发起一对一与多人音视频通话和会议，处理忙线/超时/重连，弱网自适应，录制需明确同意。

### Story 15.0: 媒体层选型、TURN 部署与可行性验证（spike）

As a 平台所有者,
I want 评估 SFU/TURN 候选并完成一次端到端可行性验证,
So that 在写通话代码前用证据选定媒体层.

*阶段：Phase 4｜需求：FR-35,FR-36｜架构：AD-13｜依赖：1.1｜规模：M*

**Acceptance Criteria:**

**Given** 候选（LiveKit/mediasoup/Janus/托管 RTC）
**When** 按许可、全端 SDK 成熟度、弱网表现、成本、自托管可行性评估
**Then** 输出 ADR-0007 结论

**Given** 选定方案
**When** 在测试环境部署 TURN 与 SFU
**Then** 完成 1 对 1 与 3 人通话联调

**Given** 许可
**When** 评估
**Then** 登记 license-and-ip，避免 GPL 传染性风险

详见 `docs/stories/15.0-*.md`（测试、跨端任务、配置与回滚）。

### Story 15.1: 信令与通话状态机

As a 终端用户,
I want 通话有统一的状态机：响铃、接通、结束、忙线、超时、未接,
So that 各种异常都有明确结果.

*阶段：Phase 4｜需求：FR-35｜架构：AD-13｜依赖：15.0,1.15,7.4,2.1｜规模：L*

**Acceptance Criteria:**

**Given** 对方忙线、无应答、拒绝
**When** 发生
**Then** 双方显示明确状态并生成通话记录

**Given** 通话令牌/TURN 凭据
**When** 签发
**Then** 由服务端按次短时签发，不写死在客户端

**Given** 信令
**When** 经业务服务
**Then** 鉴权并校验会话成员资格

**Given** 通话记录
**When** 生成
**Then** 使用已登记的通话记录消息类型（2.1），各端渲染与降级一致

详见 `docs/stories/15.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 15.2: 一对一音视频通话

As a 终端用户,
I want 发起和接听音视频通话，前后台切换与断网重连,
So that 随时语音视频沟通.

*阶段：Phase 4｜需求：FR-35｜架构：AD-13｜依赖：15.1,7.4｜规模：L*

**Acceptance Criteria:**

**Given** 通话中切后台
**When** 返回
**Then** 通话保持或按平台规则恢复

**Given** 断网
**When** 恢复
**Then** 自动重连

**Given** 摄像头麦克风权限被拒
**When** 通话
**Then** 提供引导而非静默失败

**Given** 后台或锁屏
**When** 来电
**Then** iOS 使用 VoIP 推送 + CallKit，Android 使用全屏意图 + ConnectionService（依赖 Apple 账号主体，OQ-5）

详见 `docs/stories/15.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 15.3: 群通话、会议与屏幕共享

As a 终端用户,
I want 在群里发起会议，多人入会，桌面端共享屏幕,
So that 多人协作.

*阶段：Phase 4｜需求：FR-36｜架构：AD-13｜依赖：15.2,4.1｜规模：L*

**Acceptance Criteria:**

**Given** 弱网
**When** 会议中
**Then** 优先降低视频质量保音频，恢复后自动重连

**Given** 录制
**When** 发起
**Then** 默认关闭，开启需所有参与者明确同意并有可见标识

**Given** 屏幕共享
**When** 桌面端发起
**Then** 需系统权限并显示共享中标识

详见 `docs/stories/15.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 15.4: 系统集成、权限与弱网自适应

As a 终端用户,
I want 通话遵守各平台来电与前后台规则，弱网自适应,
So that 体验自然.

*阶段：Phase 4｜需求：FR-37｜架构：AD-13｜依赖：15.2｜规模：M*

**Acceptance Criteria:**

**Given** 系统来电
**When** 与通话冲突
**Then** 按平台规则处理（挂起/保持）

**Given** 弱网
**When** 触发自适应
**Then** 分辨率/帧率逐级调整并显示提示

**Given** 权限
**When** 被拒
**Then** 引导到系统设置

详见 `docs/stories/15.4-*.md`（测试、跨端任务、配置与回滚）。

## Epic 16: 开放平台基础

开发者可以注册主体、组建团队、创建应用与密钥，通过 OAuth 授权调用开放 API，订阅 Webhook，使用机器人与 SDK，并在控制台查看统计。

### Story 16.1: 开发者注册与主体认证

As a 开发者,
I want 注册个人或企业主体并完成认证,
So that 获得上架与高风险权限的资格.

*阶段：Phase 5｜需求：OP-FR-1｜架构：AD-12｜依赖：9.1｜规模：M*

**Acceptance Criteria:**

**Given** 未认证主体
**When** 提交上架或申请需实名 scope
**Then** 被拒绝，只能创建沙箱应用

**Given** 认证材料
**When** 存储
**Then** 加密，访问需理由并留审计，可按保留策略删除

**Given** 认证审核
**When** 完成
**Then** 通知开发者并记录结论

详见 `docs/stories/16.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 16.2: 团队与角色

As a 开发者,
I want 邀请团队成员并分配所有者/管理员/开发者/只读角色,
So that 多人协作开发.

*阶段：Phase 5｜需求：OP-FR-2｜架构：AD-12,AD-3｜依赖：16.1｜规模：S*

**Acceptance Criteria:**

**Given** 角色
**When** 调用写接口
**Then** 只读成员返回 403

**Given** 转让所有者
**When** 提交
**Then** 需二次验证

**Given** 角色变更
**When** 完成
**Then** 写审计

详见 `docs/stories/16.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 16.3: 应用创建、域名绑定与密钥生命周期

As a 开发者,
I want 创建应用、验证域名并管理密钥,
So that 拥有可安全使用的应用身份.

*阶段：Phase 5｜需求：FR-48,OP-FR-3,OP-FR-4,OP-FR-5｜架构：AD-12｜依赖：16.2｜规模：L*

**Acceptance Criteria:**

**Given** 应用状态机
**When** 非法迁移
**Then** 被拒绝（draft→sandbox→in_review→approved→published→suspended→retired）

**Given** 域名
**When** 通过 DNS/文件校验
**Then** 才能在 manifest 声明；验证失效自动暂停相关版本

**Given** App Secret
**When** 创建或轮换
**Then** 仅展示一次，库中只存哈希，旧密钥宽限期后失效，吊销后 ≤60 秒内令牌失效

详见 `docs/stories/16.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 16.4: OAuth 授权码 + PKCE 与同意页

As a 终端用户,
I want 在宿主绘制的同意页里按最小 scope 授权应用，并随时撤销,
So that 授权透明可控.

*阶段：Phase 5｜需求：OP-FR-6,OP-FR-7,OP-FR-8,FR-49｜架构：AD-12,AD-11｜依赖：16.3｜规模：L*

**Acceptance Criteria:**

**Given** 授权码流程
**When** 缺少 PKCE 或 state、重定向 URI 不精确匹配
**Then** 被拒绝，授权码一次性且 ≤5 分钟

**Given** 同意页
**When** 展示
**Then** 由宿主绘制，列出 scope 的用户可读说明与数据用途，可部分同意，应用无法伪造或遮挡

**Given** 撤销
**When** 完成
**Then** 访问令牌 ≤60 秒内失效，刷新令牌复用触发整链吊销并告警

详见 `docs/stories/16.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 16.5: 开放 API、限流与调用统计

As a 开发者,
I want 调用独立监听的开放 API，有限流、幂等与统计,
So that 稳定可预期的集成.

*阶段：Phase 5｜需求：OP-FR-9,OP-FR-24,FR-52｜架构：AD-12,AD-8｜依赖：16.4｜规模：M*

**Acceptance Criteria:**

**Given** 超限
**When** 调用
**Then** 返回 429 与 Retry-After，错误含 trace_id

**Given** 个人用户 API 令牌
**When** 调用开放 API
**Then** 被拒绝

**Given** 统计
**When** 查看
**Then** 为聚合口径，不含个人数据

详见 `docs/stories/16.5-*.md`（测试、跨端任务、配置与回滚）。

### Story 16.6: Webhook、沙箱与调试日志

As a 开发者,
I want 订阅事件并在沙箱里调试，失败可重放,
So that 可靠的事件集成.

*阶段：Phase 5｜需求：OP-FR-11,OP-FR-12｜架构：AD-12,AD-7｜依赖：16.5｜规模：M*

**Acceptance Criteria:**

**Given** Webhook
**When** 投递
**Then** 带 HMAC-SHA256(timestamp.body)、时间戳窗口，失败指数退避后入死信并可手动重放

**Given** 沙箱
**When** 使用
**Then** 沙箱令牌无法访问生产数据

**Given** 调试日志
**When** 查看
**Then** 脱敏且不含令牌与完整手机号

**Given** Webhook 回调 URL
**When** 注册或投递
**Then** 禁止内网、回环、链路本地地址，并防 DNS 重绑定（投递时重新解析并校验）

详见 `docs/stories/16.6-*.md`（测试、跨端任务、配置与回滚）。

### Story 16.7: 机器人与服务账号

As a 开发者,
I want 创建机器人与服务账号，在被邀请的会话里收发消息，向订阅用户发服务消息,
So that 自动化与通知能力.

*阶段：Phase 5｜需求：OP-FR-13,OP-FR-14,FR-53｜架构：AD-12,AD-2｜依赖：16.5,4.1｜规模：M*

**Acceptance Criteria:**

**Given** 机器人
**When** 访问未被邀请的会话
**Then** 返回 403，被移出后立即失效

**Given** 服务消息
**When** 用户未订阅或已退订
**Then** 不送达，模板需审核

**Given** 机器人调用
**When** 发信
**Then** 经业务服务而非直连 IM 内核

详见 `docs/stories/16.7-*.md`（测试、跨端任务、配置与回滚）。

### Story 16.8: SDK 生成与版本化

As a 开发者,
I want 使用 TS/Go/Android/iOS SDK,
So that 更快接入.

*阶段：Phase 5｜需求：OP-FR-10,FR-71｜架构：AD-8,AD-17｜依赖：16.5,6.1｜规模：L*

**Acceptance Criteria:**

**Given** OpenAPI 契约
**When** 发布
**Then** 生成并版本化四种 SDK，带兼容窗口

**Given** SDK 版本
**When** 发布
**Then** 带兼容窗口与变更日志

**Given** SDK 示例
**When** CI
**Then** 在沙箱上端到端运行

详见 `docs/stories/16.8-*.md`（测试、跨端任务、配置与回滚）。

### Story 16.9: 开发者控制台

As a 开发者,
I want 在控制台管理应用、密钥、授权、版本与团队,
So that 集中完成接入与运营.

*阶段：Phase 5｜需求：OP-FR-23,FR-71｜架构：AD-8,AD-17｜依赖：16.8,6.1｜规模：L*

**Acceptance Criteria:**

**Given** 控制台每页
**When** 渲染
**Then** 具备加载/空/错误/无权限状态，密钥展示块一次性展示并可复制

**Given** 控制台
**When** 键盘与读屏
**Then** 可完成全部操作

**Given** 页面
**When** 1024–1440 宽
**Then** 无横向滚动

详见 `docs/stories/16.9-*.md`（测试、跨端任务、配置与回滚）。

## Epic 17: 小程序运行时与市场

用户可以在受限容器里安全使用小程序，开发者可提交、审核、灰度发布与回滚，用户可发现、评价、举报，平台可投诉处置。

### Story 17.1: WebView 隔离可行性验证（spike）

As a 平台工程团队,
I want 验证 iOS WKWebView 与 Android WebView 上“每应用独立 origin + 分区存储”的可行实现,
So that 把未验证假设变成已验证决策（AD-11）.

*阶段：Phase 5｜需求：FR-50｜架构：AD-11｜依赖：1.6｜规模：M*

**Acceptance Criteria:**

**Given** 两个小程序
**When** 运行
**Then** 一个读不到另一个的存储与 Cookie，或明确记录不可行及替代方案

**Given** 自定义 scheme 与独立域
**When** 导航到未声明域名
**Then** 被拦截

**Given** 平台规则
**When** 审查
**Then** 输出 App Review 与 Android 政策风险清单（OQ-OP-4）

详见 `docs/stories/17.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 17.2: 小程序容器与 Bridge 协议

As a 终端用户,
I want 在宿主绘制顶栏与授权弹窗的受限容器里使用小程序，Bridge 调用安全可控,
So that 安全地使用第三方小程序.

*阶段：Phase 5｜需求：OP-FR-15,OP-FR-16,OP-FR-17,FR-50｜架构：AD-11｜依赖：17.1,16.4｜规模：L*

**Acceptance Criteria:**

**Given** Bridge 调用
**When** 缺 nonce、签名或能力未声明
**Then** fail-closed，重放同一 nonce 被拒

**Given** 敏感能力（位置、相机、文件、剪贴板、分享）
**When** 使用
**Then** 每次由宿主弹窗并由用户显式授权

**Given** 能力探测
**When** 旧容器遇新能力
**Then** 返回“不支持”而非崩溃

**Given** 小程序运行
**When** 网络与资源
**Then** 网络与导航受域名白名单限制，应用 CSP 生效，内存与资源有上限，存储/Cookie 按应用分区

详见 `docs/stories/17.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 17.3: 自动风险扫描与恶意样本集

As a 审核员,
I want 提交的包自动扫描域名、危险 API、敏感能力与依赖许可证,
So that 降低恶意小程序上架风险.

*阶段：Phase 5｜需求：OP-FR-18,FR-51｜架构：AD-11,AD-23｜依赖：17.2｜规模：M*

**Acceptance Criteria:**

**Given** 恶意测试集
**When** 扫描
**Then** 全部被标记，报告随审核可见

**Given** 依赖许可证
**When** 扫描
**Then** 标出 UNLICENSED/Custom 商业许可

**Given** 误报
**When** 申诉
**Then** 有复核通道

详见 `docs/stories/17.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 17.4: 人工审核、申诉与上架

As a 审核员,
I want 依据清单人工审核，开发者可申诉,
So that 把控质量与合规.

*阶段：Phase 5｜需求：OP-FR-19,FR-51｜架构：AD-3,AD-15｜依赖：17.3｜规模：M*

**Acceptance Criteria:**

**Given** 审核
**When** 结论
**Then** 写审计并给出可操作的整改项

**Given** 申诉
**When** 提交
**Then** 由不同审核员处理

**Given** 审核通过
**When** 上架
**Then** 包签名并进入灰度

详见 `docs/stories/17.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 17.5: 灰度、回滚与下架

As a 开发者,
I want 按比例灰度，监控自动暂停，一键回滚与紧急下架,
So that 安全发布.

*阶段：Phase 5｜需求：OP-FR-20,FR-51｜架构：AD-11｜依赖：17.4,1.5｜规模：M*

**Acceptance Criteria:**

**Given** 灰度
**When** 错误率超阈值
**Then** 自动暂停并告警

**Given** 回滚
**When** 触发
**Then** 新开启的小程序使用上一版本 [目标 ≤5 分钟]，被下架应用对已授权用户发通知

**Given** 紧急下架
**When** 触发
**Then** 下次启动不可用并写审计

详见 `docs/stories/17.5-*.md`（测试、跨端任务、配置与回滚）。

### Story 17.6: 小程序市场与评价举报

As a 终端用户,
I want 浏览、搜索、查看详情与评价，收藏与历史，举报小程序,
So that 发现好用且可信的应用.

*阶段：Phase 5｜需求：OP-FR-21,OP-FR-22,FR-63｜架构：AD-3｜依赖：17.5,8.1｜规模：M*

**Acceptance Criteria:**

**Given** 下架或被判恶意
**When** 浏览与搜索
**Then** 不出现在任何列表

**Given** 详情页
**When** 查看
**Then** 显示申请的权限与用途

**Given** 评价与举报
**When** 提交
**Then** 限频防刷，同一用户对同一应用仅保留一条有效评价，举报量达阈值触发复审

详见 `docs/stories/17.6-*.md`（测试、跨端任务、配置与回滚）。

### Story 17.7: 投诉与处置

As a 平台运营,
I want 受理用户与第三方对应用的投诉并处置，可申诉,
So that 平台责任与可追溯.

*阶段：Phase 5｜需求：OP-FR-25｜架构：AD-3,AD-15｜依赖：17.6｜规模：S*

**Acceptance Criteria:**

**Given** 投诉
**When** 受理
**Then** 处置包含警告、限权、下架、封禁主体，均写审计并可申诉

**Given** 封禁主体
**When** 生效
**Then** 其所有应用暂停并通知

**Given** 处置
**When** 复核
**Then** 可撤销并留痕

详见 `docs/stories/17.7-*.md`（测试、跨端任务、配置与回滚）。

## Epic 18: 订阅、权益与支付

平台可以按套餐赋予服务端强制的权益，并在合规支付渠道落地后完成订单、支付回调与对账。

### Story 18.1: 套餐与权益服务

As a 终端用户,
I want 获得由服务端强制的套餐权益与配额,
So that 功能与配额可控可审计.

*阶段：Phase 6｜需求：FR-54｜架构：AD-3,AD-21｜依赖：9.4｜规模：L*

**Acceptance Criteria:**

**Given** 套餐变化
**When** 生效
**Then** 权益实时更新并可审计

**Given** 客户端
**When** 尝试本地开启付费功能
**Then** 服务端拒绝

**Given** 配额
**When** 超限
**Then** 在 before_send 或接口层被拒并提示升级

详见 `docs/stories/18.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 18.2: 订单、支付回调与对账

As a 终端用户,
I want 创建订单并支付，回调验签幂等，日终对账,
So that 可靠、可核对的交易.

*阶段：Phase 6｜需求：FR-55｜架构：AD-7｜依赖：18.1｜规模：L*

**Acceptance Criteria:**

**Given** 支付回调
**When** 验签失败或重复
**Then** 拒绝且重放不重复入账

**Given** 日终对账
**When** 运行
**Then** 差异可见并有处理流程

**Given** 退款
**When** 发起
**Then** 状态机与审计完整

**Given** 订单金额与币种
**When** 客户端或回调篡改
**Then** 以服务端订单为准，金额/币种不一致拒绝，退款不得超过实付

**Given** 移动端数字内容订阅
**When** 上架
**Then** 通过商店内购（Apple IAP / Google Play Billing）并做服务端回执校验；或在 v1 明确不在移动端售卖并仅提供 Web 支付，由 OQ-3/OQ-5 决策

详见 `docs/stories/18.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 18.3: 开发者结算（条件项）

As a 开发者,
I want 在决策与法务批准后结算收入,
So that 开发者生态变现.

*阶段：Phase 6｜需求：OP-FR-26｜架构：AD-12｜依赖：18.2,16.1｜规模：L*

**Acceptance Criteria:**

**Given** 未获批准
**When** 访问
**Then** 不暴露任何收费入口

**Given** 获批
**When** 结算
**Then** 对账、发票/税务对接符合 FR-55 的幂等与审计要求

**Given** 争议
**When** 提交
**Then** 有处理流程

详见 `docs/stories/18.3-*.md`（测试、跨端任务、配置与回滚）。

## Epic 19: 官网、下载与客户端更新

访客可以在官网了解产品并下载对应平台的应用；已安装的客户端可以安全更新与灰度。

### Story 19.1: 官网与下载中心

As a 访客,
I want 官网展示价值、跨端下载、功能、隐私与公告，并可切换语言,
So that 清楚了解并获得应用.

*阶段：Phase 6｜需求：FR-56｜架构：AD-8,AD-17｜依赖：9.4,6.3｜规模：L*

**Acceptance Criteria:**

**Given** 下载中心
**When** 配置
**Then** 下载链接与版本由后台控制，并展示校验信息

**Given** 官网
**When** 评估
**Then** 满足 SEO、性能预算与 WCAG 2.1 AA 验收清单

**Given** 文案
**When** 发布
**Then** 不含“绝对安全/端到端加密/无限”等未经验证的说法，隐私与条款经法务确认（OQ-4）

详见 `docs/stories/19.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 19.2: 客户端更新与灰度

As a 终端用户,
I want 应用检查更新，支持强制/可选更新与灰度，桌面更新有签名校验,
So that 安全及时地升级.

*阶段：Phase 6｜需求：FR-57｜架构：AD-14｜依赖：9.4｜规模：M*

**Acceptance Criteria:**

**Given** 更新包签名不匹配
**When** 安装
**Then** 被拒绝

**Given** 强制更新
**When** 触发
**Then** 提供过渡窗口与说明

**Given** 灰度
**When** 配置比例
**Then** 仅命中设备收到更新并可回滚

详见 `docs/stories/19.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 19.3: 官网访客客服入口与匿名访客认证

As a 访客,
I want 在官网无需登录即可咨询客服，访客身份单独认证并受限,
So that 未注册用户也能获得帮助而不扩大攻击面.

*阶段：Phase 6｜需求：FR-38｜架构：AD-3,AD-10,AD-16｜依赖：14.2,19.1｜规模：M*

**Acceptance Criteria:**

**Given** 匿名访客
**When** 发起咨询
**Then** 获得短期、仅限客服会话范围的访客令牌，不能访问任何用户 API

**Given** 访客滥用
**When** 超限
**Then** 按 IP/指纹限频并可封禁

**Given** 访客资料
**When** 客服查看
**Then** 仅展示最小字段并受保留策略约束

详见 `docs/stories/19.3-*.md`（测试、跨端任务、配置与回滚）。

## Epic 20: 硬化与发布就绪

平台通过压测、灾备演练、安全测试、WCAG 验收与合规检查，具备逐项签字的发布就绪证据。

### Story 20.1: 性能压测与容量基线

As a 平台所有者,
I want 用测试环境模拟 1k→10k 连接与群发，记录硬件与流量模型,
So that 用证据而非猜测承诺规模.

*阶段：Phase 7｜需求：NFR-9｜架构：AD-23｜依赖：1.15,13.1,1.5,4.1,7.4,15.3｜规模：L*

**Acceptance Criteria:**

**Given** 压测
**When** 1k→10k 逐级
**Then** 记录硬件、配置、流量模型与 SM-1/SM-8 结果

**Given** 群发与已读扇出
**When** 压测
**Then** 确定群成员与已读详情上限

**Given** before_send
**When** 压测
**Then** 记录对发送时延的影响并确定预算

详见 `docs/stories/20.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 20.2: 备份恢复与灾备演练

As a 平台所有者,
I want 验证 MySQL PITR、IM 存储、对象存储的备份与恢复,
So that RPO/RTO 有证据.

*阶段：Phase 7｜需求：NFR-6｜架构：AD-22｜依赖：1.22｜规模：L*

**Acceptance Criteria:**

**Given** 备份
**When** 恢复演练
**Then** 达到配置的 RPO/RTO 并记录演练报告

**Given** 备份
**When** 存储
**Then** 加密并限权

**Given** 演练失败或缺失
**When** 发布评审
**Then** 阻止 Release Ready

详见 `docs/stories/20.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 20.3: 安全测试与暴露面复测

As a 安全工程师,
I want 执行 SAST/DAST/渗透与暴露面复测，高危清零,
So that 上线前清零高危阻断项.

*阶段：Phase 7｜需求：NFR-17,NFR-18,NFR-2,NFR-14,NFR-16｜架构：AD-2,AD-23｜依赖：1.24｜规模：L*

**Acceptance Criteria:**

**Given** 暴露面
**When** 外部与内网复测
**Then** 5001/5301/7001/9099/pprof/MQTT 均不可达

**Given** SAST/DAST/SCA/secret 扫描
**When** 运行
**Then** 无高危/严重漏洞，已知项有处置记录

**Given** 威胁模型 T-01…T-27
**When** 逐项验证
**Then** 每项有测试证据

详见 `docs/stories/20.3-*.md`（测试、跨端任务、配置与回滚）。

### Story 20.4: 各端 WCAG 2.1 AA 验收与第三方审计

As a 无障碍负责人,
I want 各端逐条完成 WCAG 清单并通过读屏、键盘、字号与回流人工测试，首发前第三方审计,
So that 可证明地满足无障碍标准.

*阶段：Phase 7｜需求：NFR-8｜架构：AD-17｜依赖：6.5｜规模：L*

**Acceptance Criteria:**

**Given** 每个客户端
**When** 发布前
**Then** 完成 WCAG-2.1-AA-CHECKLIST 的逐端副本，任一 A/AA 条未通过则不得宣称合规

**Given** MANUAL 与 IMPL 条目
**When** 测试
**Then** 有 a11y-manual-test-log 记录（测试人、设备/AT、结果）

**Given** 首个公开发布
**When** 评审
**Then** 第三方出具审计报告与 ACR/VPAT

详见 `docs/stories/20.4-*.md`（测试、跨端任务、配置与回滚）。

### Story 20.5: 许可证、SBOM 与合规材料

As a 合规负责人,
I want 汇总许可证清单、SBOM、NOTICE 与隐私/条款材料,
So that 许可证冲突与合规缺口清零.

*阶段：Phase 7｜需求：NFR-5,NFR-11｜架构：AD-18,AD-23｜依赖：1.24｜规模：M*

**Acceptance Criteria:**

**Given** 所有依赖
**When** 扫描
**Then** 无 UNLICENSED/未批准的 Custom 商业许可；@fancyapps/ui 已购买授权或替换

**Given** Manager/Web 无许可证风险
**When** 发布
**Then** 随发布留存用户的书面接受记录

**Given** 字体与素材
**When** 清点
**Then** 保留 OFL 署名与 Apache-2.0 NOTICE

详见 `docs/stories/20.5-*.md`（测试、跨端任务、配置与回滚）。

### Story 20.6: 签名、灰度发布与回滚演练

As a 发布经理,
I want 各端签名构建、灰度发布与回滚演练，形成 Release Readiness 签字清单,
So that 可控的上线与可逆的回退.

*阶段：Phase 7｜需求：NFR-10,FR-57｜架构：AD-14｜依赖：19.2,20.2｜规模：L*

**Acceptance Criteria:**

**Given** 签名构建
**When** 产出
**Then** 可重复构建、带校验和与 SBOM，签名密钥受保护

**Given** 灰度
**When** 发布
**Then** 分阶段放量，指标异常自动暂停

**Given** 回滚演练
**When** 执行
**Then** 在目标时间内恢复上一版本并有记录，Release Readiness 逐项签字

详见 `docs/stories/20.6-*.md`（测试、跨端任务、配置与回滚）。

## Epic 21: 未排期扩展（P1/P2 backlog）

架构已预留、PRD 评审后再排期的扩展能力；本期不实现，也不出现在任何里程碑。

### Story 21.1: 频道/广播与运营消息

As a 平台运营,
I want 广播频道、公告号、运营消息模板、分群触达与退订,
So that 规模化触达用户.

*阶段：未排期｜需求：FR-58｜架构：AD-7｜依赖：无｜规模：L*

**Acceptance Criteria:**

**Given** 用户
**When** 退订
**Then** 立即停止触达且可审计

**Given** 触达
**When** 发送
**Then** 有记录与限频

**Given** 分群
**When** 创建
**Then** 遵守隐私与最小化

详见 `docs/stories/21.1-*.md`（测试、跨端任务、配置与回滚）。

### Story 21.2: AI 辅助（翻译/总结/客服建议）

As a 终端用户,
I want 在明确同意下使用 AI 功能,
So that 效率提升.

*阶段：未排期｜需求：FR-59｜架构：AD-20｜依赖：无｜规模：L*

**Acceptance Criteria:**

**Given** 默认
**When** 使用
**Then** 功能关闭，需用户明确同意

**Given** 私聊内容
**When** 未同意
**Then** 绝不发送给第三方模型

**Given** 成本
**When** 超限
**Then** 限速

详见 `docs/stories/21.2-*.md`（测试、跨端任务、配置与回滚）。

### Story 21.3: 企业组织与租户

As a 企业管理员,
I want 组织通讯录、部门、企业空间与租户隔离,
So that 企业场景.

*阶段：未排期｜需求：FR-60｜架构：AD-10｜依赖：无｜规模：L*

**Acceptance Criteria:**

**Given** 租户设计
**When** 评审
**Then** 隔离方案经测试后再决定

**Given** 未通过评审
**When** 对外
**Then** 不宣称多租户

**Given** 隔离
**When** 验证
**Then** 有跨租户负面测试

详见 `docs/stories/21.3-*.md`（测试、跨端任务、配置与回滚）。

