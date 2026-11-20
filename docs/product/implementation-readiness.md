# 实施就绪度检查（2026-11-20）

**结论：CONCERNS — 可以启动 Phase 2；Phase 3 及之后不具备启动条件（设计门禁未签字）。**
判断依据：“开发者能否在不自行发明决策的前提下实现这些 Epic？”

## 通过的部分
- 需求→Story→行级追溯：FR 72/72、OP-FR 26/26、NFR 18/18、功能矩阵 260/260（`traceability*.csv`，生成器校验）。
- Story 依赖按“阶段→编号”排序无环；每个 Story 有 ≥3 条 Given/When/Then、测试、11 列跨端任务、配置与回滚。
- 架构决策已记录（AD-1…24、ADR-0001…0010）；独立评审（Epics、架构、设计、PRD）均已处理 Critical/High。

## CONCERNS（按严重度）
1. **设计门禁 G1–G6 全部 PENDING**（`docs/design/APPROVALS.md`）。影响：Phase 3 批量编码不得启动；Phase 2 的 1.14/1.17/1.18/1.20 含界面，只能按“最小实现、沿用已生成令牌与图标、不沿用上游样式”交付。*修复：你签字或书面豁免；`python3 tools/check_gates.py` 通过。*
2. **商业/法务决策未定，影响 Phase 2 的具体 Story**：
   - OQ-1 品牌/包名/域名/商标核验 → 1.9、1.10（开发期用临时包名，发布前必须替换）。
   - OQ-8 短信服务商 → 1.12（先做 SmsProvider 端口 + 沙箱适配器，staging 前必须选定）。
   - OQ-7 规模与 RPO/RTO → 1.21、1.22（先用保守默认值）。
   - 后续阶段的阻塞：OQ-2 地图（10.2）、OQ-3 支付（18.2/18.3）、OQ-4 法规辖区（保留期/实名/隐私文案）、OQ-5 Apple/Google 账号（推送/CallKit/商店）、OQ-6 音视频与转码、OQ-9 对象存储（10.3）、OQ-12 开发者实名与结算。
3. **未验证的前提**：WuKongIM 固定 SHA 为未打 tag 的 main（v3 beta，R-29）；Server `modules/*` 的表结构与权限未逐接口审计（影响 1.23 迁移决策）；`before_send` 时延预算（≤50 ms）、撤销时效（≤60 s）、同步时延等均为待测目标；小程序独立 origin 在 iOS/Android WebView 上可行性待 17.1 验证；MySQL 8.0 已 EOL（风险已接受 R-31）。
4. **Epic 跨阶段/跨 Epic 依赖**：Epic 3/4/5/13 的 Story 分布在 Phase 3 与 Phase 4；有 1 处依赖指向编号更大的 Epic（5.2→7.1）。执行顺序按“阶段→编号”已无环，但 Epic 不是严格“独立可交付”的价值切片，回顾与验收需按 Story 阶段而非 Epic 编号。
5. **规模估计粗略，部分 Story 仍偏大**：Phase 2 已全部拆到 ≤M；Phase 3+ 仍有 L 级 Story 未拆（9.6、6.4、17.2、19.1、14.3 及多端 L），开始对应阶段前必须再拆（见 `epics-revision-log.md`）。
6. **缺口**：i18n 流水线、环境体系、生产 TURN/SFU 部署、租户守卫、开发者文档站尚无 Story；Manager/Web 无许可证风险由你已接受，`@fancyapps/ui` 授权须在发布前解决（20.5）。

## 建议的下一步
1. 你签字（或豁免）设计门禁 G1–G6，并就 OQ-1/7/8 给出决策或同意我用临时默认值推进。
2. 按 `sprint-1-plan.md` 的批次启动 Phase 2（用 `bmad-build` 从 `1-1-…` 开始）。
3. 在 Phase 3 开始前：拆分 L 级 Story，补缺口 Story，并复核 OQ-4/5/9。
