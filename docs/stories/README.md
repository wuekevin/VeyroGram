# Stories

按依赖排序的 Story 文件。命名：`<epic>.<story>-<slug>.md`。状态默认 `backlog`（尚未开始；设计门禁未签字前 Phase 3 不启动）。

## 通用完成定义（DoD，适用于每个 Story）
1. 代码经 PR 合并，评审通过；改动遵守 `docs/architecture/ARCHITECTURE.md` 的 AD 规则（尤其依赖方向 AD-1 与对象级授权 AD-3）。
2. 测试通过：单元 + 集成（真实 MySQL 8.0 / Redis 7）+ `go test -race`；本 Story 列出的**安全负面测试**已实现并通过；契约测试（OpenAPI / 消息类型 schema）通过。
3. 静态/供应链门禁通过：SAST、SCA、secret 扫描、许可证扫描、SBOM；高危/严重漏洞为零（AD-23）。
4. 涉及接口：OpenAPI 3.1 已更新且 diff 门禁通过；涉及数据库：迁移与回滚说明已评审并在 staging 演练。
5. 涉及界面：对照 `docs/design/` 的令牌与组件规格；明/暗、中/英、200% 字号截图基线已更新；`tools/a11y_scan.py` 通过；WCAG 清单对应条目已填证据；导航无 Emoji（lint）。
6. 跨端任务（Server/Android/iOS/Web/Desktop/Manager/API/DB/通知/权限/E2E）逐列完成或有正式“不适用”说明。
7. 配置项与回滚方案已写入运维文档；功能开关默认值经确认；监控指标与告警已接入（NFR-7）。
8. **追溯闭环：** 将 `docs/audit/feature-matrix.csv` 中本 Story 对应的行（见 `docs/product/traceability.csv`）的状态更新为 `TESTED` 并附证据路径（测试报告/截图），否则 Epic 不得关闭（SM-4）。
9. 文档：用户/管理员/开发者手册（如适用）与变更说明已更新；风险登记与威胁模型相关条目已更新。

## 状态约定
`backlog` → `ready` → `in-progress` → `review` → `done`。Story 只能依赖执行顺序（先按阶段，再按编号）更靠前的 Story，依赖图无环（生成器已校验）。
