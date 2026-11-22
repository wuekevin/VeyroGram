# apps/manager — 管理后台（Vue）构建基线（Story 1.10）

上游 `TangSengDaoDaoManager` 同样只存**固定 SHA + 补丁**：

- `PIN`：固定的上游提交。
- `patches/0001-…`：`pnpm-lock.yaml` 里 2 处镜像 tarball 改回官方源（sha512）；vite 4→6、`@vitejs/plugin-vue(-jsx)`、axios 1.21、vue-i18n 11；`pnpm-workspace.yaml` 的 `overrides` 把剩余传递依赖固定到已修复版本；Dockerfile 的镜像源改官方。

构建：`bash tools/build_clients.sh manager /绝对路径/新目录`（冻结安装 → 审计（高危/严重必须为 0，或有批准的豁免）→ 构建 → 输出哈希）。

## 已知限制（诚实说明）
- **没有在浏览器里点过**：`vite build` 通过且两次干净构建产物哈希一致，但 vite 6/vue-i18n 11 升级后的界面没有人工验证；`vue-tsc` 在上游代码上本来就有类型错误（构建脚本不跑它），这次没有修。
- Manager 的授权风险已由你接受（Story 1.10 配置/决策点）；这里没有改它的业务代码。
- 审计只覆盖 npm 注册表公开的公告；`moderate` 级别仍有若干条未处理，不在本 Story 门禁内。
