# PRD Addendum — VeyroGram

## A. 付费模块 → FR 映射（需求基线，逐行明细见 docs/audit/feature-matrix.csv）

|矩阵模块|行数|对应 FR|
|--|--|--|
|基础版|139|FR-1…13、FR-61、FR-64…FR-67、FR-65(链接预览/SSRF)、FR-68…70(设计/桌面)|
|旗舰模块|21|FR-30…34 (+FR-9,10)|
|音视频模块|6|FR-35…37|
|表情商店|14|FR-25|
|群管理模块|10|FR-15…19|
|安全与隐私模块|6|FR-1…5,FR-29|
|小视频模块|7|FR-24|
|位置模块|6|FR-21|
|文件模块|5|FR-22,23|
|标签模块|6|FR-14|
|朋友圈模块|27|FR-26…28|
|收藏模块|7|FR-20|
|客服模块|6|FR-38…40|

**注意**：FR 为归并后的可验收需求；矩阵逐行（260 行）在 Epics/Stories 阶段拆为 Story，任何一行未关联 Story 不得关闭对应 Epic。基础版中与付费模块重复的条目（消息收藏、在线状态、截屏/撤回通知、消息撤回、历史查找）以付费模块 FR 为准。

## B. 官方 Swagger 扩展模块（超出价格页，待 PRD 评审后决定是否纳入）
hotline(55 操作)、organization(46)、community(17)、web3(5)、tgocall(5)、invite(3)、search(1)。来源：docs/audit/apidocs-botgate-index.md。仅作契约参考，不复制闭源实现。

## C. 推迟到 ADR/架构的决策
IM 内核版本锁定（WuKongIM v3 beta）、WebRTC SFU/TURN 选型、对象存储适配、搜索引擎引入阈值、outbox/worker 方案、WuKongIM 内部 API 网关方案（见 docs/audit/adr/0001）。

## D. 风险追溯
NFR-2↔R-21；NFR-3↔R-02/03/20/24；NFR-5↔R-12/13/28；NFR-11↔R-04/28；FR-1↔R-02；FR-45/46↔R-25。

## E. 独立评审处置（review-prd-independent.md）
- C-1：已新增 NFR-13…NFR-18（Web/Electron、移动端、令牌、端口暴露、安全门禁、STRIDE）。
- C-2：已新增 §4.23 FR-68/69/70（设计系统、无 Emoji 图标、品牌资产、视觉回归、桌面体验）。
- C-3：已新增 FR-64…67（群创建/公告/@、语音名片链接预览(SSRF)、清空记录、Web/桌面登录）。基础版 139 行的逐行 Story 化与 TESTED 追踪放入 Epics/Stories 阶段。
- High：术语表 IM 内核矛盾已修；十列矩阵已在 CSV 补 api/db/notification/permission/e2e 列(全 UNVERIFIED)；客服/朋友圈/表情/截屏的缺口并入 FR-38/27/25/34；开放平台补 FR-71 与隔离/CSP；管理后台补 FR-72；FR-49 与 SM-6 对齐 60 秒；SM-1 绑定负载条件，新增 SM-8；许可证表述已拆分为“风险接受”与“阻断项”。
- 仍待办（Medium/Low）：约 15 条“立即/实时/一致”后果缺界限值，需在 Stories 阶段逐条量化；FR 编号在局部不连续(FR-71、FR-72 位于 FR-53/FR-47 之前)，ID 稳定优先于顺序。
