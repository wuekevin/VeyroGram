# apps/manager — 管理后台（Vue）构建基线（Story 1.10）

上游 `TangSengDaoDaoManager` 同样只存**固定 SHA + 补丁**：

- `PIN`：固定的上游提交。
- `patches/0001-…`：`pnpm-lock.yaml` 里 2 处镜像 tarball 改回官方源（sha512）；vite 4→6、`@vitejs/plugin-vue(-jsx)`、axios 1.21、vue-i18n 11；`pnpm-workspace.yaml` 的 `overrides` 把剩余传递依赖固定到已修复版本；Dockerfile 的镜像源改官方。

构建：`bash tools/build_clients.sh manager /绝对路径/新目录`（冻结安装 → 审计（高危/严重必须为 0，或有批准的豁免）→ 构建 → 输出哈希）。

## 许可证发现：`@fancyapps/ui`
许可证门禁发现 Manager 依赖的 `@fancyapps/ui`（Fancybox 5）是**商业许可**（“使用需同意其 License Agreement，价格见 fancyapps.com/pricing”），用于 3 处图片预览。已移除，改用 Element Plus 的 `ElImageViewer`（MIT，`src/utils/imagePreview.ts`）。

## 验证情况与已知限制
- **已验证（本机）：** 冻结安装、构建、两次干净构建哈希一致、审计高危/严重为 0、许可证门禁与 SBOM（788 个包）、登录页在隐藏的 Electron 窗口里渲染无应用错误。
- **没有验证：** 登录之后的页面；**新的图片预览组件没有在页面里点过**（只验证了它能编译进构建）；vite 6、vue-i18n 11、element-plus 升级后的其余界面；`vue-tsc` 在上游代码上本来就有类型错误（构建脚本不跑它），没有修。
- 安全公告：`moderate` 剩 3 条（`echarts` 需升大版本 6、`minimatch` 跨大版本、`vue-template-compiler` 无修复版本）。
- Manager 的授权风险已由你接受（Story 1.10 配置/决策点）；这里没有改它的业务代码。
