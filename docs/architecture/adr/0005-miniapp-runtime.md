# ADR-0005 小程序运行时：受限 WebView + Native Bridge（第一阶段）
状态：Accepted｜日期：2026-11-19
决策：每个小程序独立 origin（`<app_id>.apps.<domain>`）、签名包、能力清单、CSP、分区存储、Bridge nonce+签名（AD-11）。
备选：自研 DSL/双线程运行时（成本高，延后评估）；iframe 直接嵌入（无隔离，拒绝）。
风险：iOS App Review 与各平台规则需在 Phase 5 前审查（Open Q：PRD OQ-5）。
