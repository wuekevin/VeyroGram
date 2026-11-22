# apps/web — Web 与桌面（Electron）客户端构建基线（Story 1.10）

上游 `TangSengDaoDaoWeb` 不入库（`upstream/` 被忽略），这里只放**固定 SHA + 补丁队列**，做法与 WuKongIM 相同：

- `PIN`：固定的上游提交。
- `patches/0001-…`：依赖改走官方源（`registry.npmjs.org`），锁文件重写并通过 `integrity` 校验；React 18.3、Electron 45、axios 0.35 等升级；用 `resolutions`/重新解析清掉全部高危传递依赖。
- `patches/0002-…`：`ReactDOM.render` → `createRoot`（含 `flushSync`）；去掉写死的上游 SaaS 地址（`api.botgate.cn` 作为默认 API 与更新服务器）、appId 改为 `com.veyrogram.app`（临时，待 OQ-1）。
- `patches/0003-…`：Electron 加固（见下）。

构建（需要网络访问官方 npm 源）：

```
git clone https://github.com/TangSengDaoDao/TangSengDaoDaoWeb.git upstream/TangSengDaoDaoWeb && git -C upstream/TangSengDaoDaoWeb checkout $(cat apps/web/PIN)
bash tools/build_clients.sh web /绝对路径/新目录             # 应用补丁、冻结安装、审计、构建、静态检查
VG_ELECTRON_SMOKE=1 bash tools/build_clients.sh web …       # 另外下载 Electron 并跑运行时安全烟雾测试
```

## Electron 安全配置
- 所有窗口 `contextIsolation: true`、`nodeIntegration: false`、`sandbox: true`、`webSecurity: true`、禁用 `<webview>`（`src-election/main/security.ts`）。
- 预加载脚本不再透传任意通道：只暴露**白名单**里的发送/调用/接收通道；主进程每个 `ipcMain` 处理器都经 `onTrusted`/`handleTrusted`，校验调用方帧是应用自己的内容。
- 导航离开应用内容被拦，`window.open` 一律拒绝（`https:` 链接交给系统浏览器），权限请求只放行媒体/通知/剪贴板写入/全屏。
- DevTools 快捷键只在开发模式注册。
- 自动更新服务器改为构建时注入（`VEYROGRAM_UPDATE_URL`），为空则**关闭自动更新**；没有厂商默认值。
- 门禁：`tools/check_electron_security.py`（静态：不安全设置、裸 IPC、白名单与处理器不一致、Electron 过旧、下载镜像、禁用 asar、非 https 更新地址）+ `src-election/test/smoke.js`（运行时：真实预加载下 `require`/`process` 不可用、未知通道被拒、导航/弹窗被拦）。

## 验证情况与已知限制
- **已验证（本机 macOS）：** 构建；Electron 45 运行时烟雾测试 20 项；`electron-builder --mac --dir`（未签名）打出 `.app` 并实际启动 10 秒无报错（日志里只有“自动更新已关闭”）；登录页在隐藏的 Electron 窗口里渲染（`tools/render_smoke.js`），点“登录”弹出 Semi UI 提示（React 18 下 Toast 正常）、点“扫描登录”切换页面，均无应用错误；`qrcode.react@1`、`react-spinners@0.11` 在 React 18 下能渲染（仅有 defaultProps 弃用警告）。
- **没有验证：** 登录之后的聊天主界面（需要后端，等 Story 1.14）；扫码页的二维码（需要后端发码）；macOS 签名/公证、Windows/Linux 安装包。
- **选 React 18 而非 19**：Semi UI 2.x 的 Toast/Modal 仍用 `ReactDOM.render`，React 19 已移除。
- **打包、签名、更新通道**需要证书与更新服务器（清单 C13）。`electron` 必须固定精确版本（electron-builder 解析不了范围，打包时发现的问题，已由门禁强制）。
- 安全公告：高危/严重为 0；`moderate` 剩 5 条（`glob`/`minimatch` 需跨大版本、`@babel/runtime@7.4.5` 被上游精确锁定等），不在门禁内。
- npm 依赖有许可证门禁与 CycloneDX SBOM（`tools/npm_supply.py`，1554 个包）。`keypair`/`node-forge` 是“BSD 或 GPL”双许可，按 BSD 取用。
- Tauri 目录（`src-tauri`）未升级也未纳入构建，更新端点指向保留域名 `updates.invalid`；桌面端选 Electron。
- **没有内容安全策略（CSP）**：渲染冒烟时 Electron 报“Insecure Content-Security-Policy”，Web 的 `index.html` 没有 CSP。这次没加（加严格 CSP 会影响上游内联脚本与第三方资源，需要逐项验证），已记为后续加固项。
- CI 的 Electron 烟雾/渲染测试在 Linux 上用 `--no-sandbox` + xvfb（runner 没有 SUID 沙箱）；真正的沙箱在 macOS 上跑过。
