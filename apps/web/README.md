# apps/web — Web 与桌面（Electron）客户端构建基线（Story 1.10）

上游 `TangSengDaoDaoWeb` 不入库（`upstream/` 被忽略），这里只放**固定 SHA + 补丁队列**，做法与 WuKongIM 相同：

- `PIN`：固定的上游提交。
- `patches/0001-…`：依赖改走官方源（`registry.npmjs.org`），锁文件重写并通过 `integrity` 校验；React 18.3、Electron 45、axios 0.35 等升级；用 `resolutions`/重新解析清掉全部高危传递依赖。
- `patches/0002-…`：`ReactDOM.render` → `createRoot`（含 `flushSync`）；去掉写死的上游 SaaS 地址（`api.botgate.cn` 作为默认 API 与更新服务器）、appId 改为 `app.veyrogram.desktop`（临时，待 OQ-1）。
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

## 已知限制（诚实说明）
- **没有在界面里点过**：构建通过、Electron 运行时烟雾测试通过，但 React 18 的整体界面行为没有人工验证。`qrcode.react@1`、`react-spinners@0.11` 的 peer 声明仍是 React ≤17（yarn 有警告），能构建，运行效果需要在 Story 1.14 前后在真机/浏览器上看一遍。
- **选 React 18 而非 19**：Semi UI 2.x 的 Toast/Modal 仍用 `ReactDOM.render`，React 19 已移除；18.3 是能保持现有界面不坏的最新版本。是否继续升 19 取决于是否换 UI 库（设计门禁之后再定）。
- **Electron 打包、签名、公证、自动更新通道没有做**（需要证书与更新服务器，见 `docs/ops/MANUAL-CHECKLIST.md` C13）；`electron-builder` 配置只去掉了镜像与厂商地址。
- Tauri 目录（`src-tauri`）未升级也未纳入构建，只把更新端点指向保留域名 `updates.invalid`；本项目桌面端选 Electron。
- CI 里的 Electron 烟雾测试在 Linux 上用 `--no-sandbox` + xvfb 运行（runner 没有 SUID 沙箱），`webPreferences.sandbox` 仍在静态与运行时检查中断言；真正的沙箱在 macOS 上已跑过。
