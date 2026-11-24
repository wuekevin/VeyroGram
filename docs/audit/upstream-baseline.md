# upstream-baseline（2026-11-19）

浅克隆（--depth 1，因磁盘仅剩 <1GB），位于 `upstream/`（未纳入 git，不覆盖原仓库）。

| 仓库 | 分支 | commit SHA | 提交时间 | 许可证(API/文件) |
|---|---|---|---|---|
| TangSengDaoDaoServer | main | fc81513d34650ee39401b5197553697b8c7b4012 | 2026-11-12 | Apache-2.0 / LICENSE |
| TangSengDaoDaoManager | main | 6f5fc4b361d81f7bda8136ef4d8852fcae04a66c | 2026-06-11 | **无 LICENSE 文件** |
| TangSengDaoDaoAndroid | master | 115e3544065b44d83ca8df4d87131c2c6ecd2423 | 2026-05-20 | Apache-2.0 / LICENSE |
| TangSengDaoDaoiOS | main | 6913a3b320f6b278676ac22e4a45e416b64cb56f | 2026-09-28 | Apache-2.0 / LICENSE |
| TangSengDaoDaoWeb | main | 1bd8a2d15064a48eff85be423530801eed9abc99 | 2026-06-11 | **无 LICENSE 文件** |
| TangSengDaoDaoDocs | main | 41e11cbd276ff3e1f44f7c16b95b663afe1f24f1 | 2026-01-01 | MIT / LICENSE |
| WuKongIM | tag v3.0.0-beta.30 | 21d905a328bb7141167bc6dbe20fdb65588e0420 | 2026-11-24 | Apache-2.0 / LICENSE（审计基线原为 main@7b810e08 2026-11-11，已升级固定点，见下） |

## 已在源码确认的事实 [FOUND_IN_SOURCE]
- Server：Go，`go.mod` 声明 `go 1.20`，依赖 `TangSengDaoDaoServerLib`（伪版本 9410aea0201d，2026-11-12）；`modules/` 含 base、channel、common、file、group、message、openapi、qrcode、report、robot、search、source、statistics、user、webhook、workplace；仅 23 个 `_test.go`。SECURITY.md 列 5.1.x/4.0.x 受支持。
- Manager：`tsdd-control` v1.5.0，Vue 3 + Pinia + Element Plus + vue-i18n + Vite；测试文件 0。
- Web：monorepo(turbo/yarn)，apps/web + packages(tsdaodaobase/contacts/datasource/login…)；**React ^17.0.2**、Electron 26.0.0、TypeScript ^5.3.3（Electron 26 已远落后于当前版本，待核验已知 CVE）。
- Android：模块 wkbase/wklogin/wkpush/wkscan/wkuikit + MyLibs；测试相关文件 8 个。
- iOS：Podfile + 已提交的 `Pods/` 与 `Vendor/`（仓库体积 337MB，供应链需审计）。
- Docs：82 个 md 页（VitePress/UnoCSS 站点）。
- 官方定价页含"客服模块（开发中）"，与总指令一致。

## 未验证
上游 Issue/PR/release、依赖漏洞（SCA）、各端构建、测试通过率、WuKongIM 与 Server 的版本兼容性：**UNVERIFIED**。

## 文档核对补充（2026-11-19）
- 82 页 Docs 已逐页读取（READ_LOCAL），见 docs-review/。功能矩阵已由 price/detail.md 的 HTML 表格（含 rowspan 层次）重新生成：基础版 139 行 + 付费 12 模块 121 行，共 260 行，含源文件行号。源表个别 rowspan 与实际行数不符，层次列在这些位置可能不准，叶子条目以 md_source 行号为准。
- 功能矩阵中各端状态仍全部 UNVERIFIED，须在 Phase 3/4 对照源码与测试逐行更新。

## 上游活跃度与发布（GitHub API，2026-11-19）
| 仓库 | open issues+PR | 最近 release | 最近 push |
|---|---|---|---|
| Server | 40 | v1.0.5 (2023-11-02) | 2026-11-12 |
| Manager | 1 | v1.5.0 (2025-02-11) | 2026-06-11 |
| Android | 0 | 无 | 2026-05-20 |
| iOS | 5 | 无 | 2026-09-28 |
| Web | 22 | v1.0.0 (2023-07-12) | 2026-06-11 |
| WuKongIM | 101 | v3.0.0-beta.27 (2026-11-04) | 2026-11-11 |
Issue/PR 内容未逐条阅读（仅计数），不据此认定具体缺陷。

## WuKongIM 固定点升级记录
- 2026-11-24：`7b810e08`（main，介于 v3.0.0-beta.27 与 beta.28 之间，未打标签）→ `21d905a3`（标签 **v3.0.0-beta.30**，领先 39 个提交，旧固定点是它的祖先）。补丁队列（`patches/wukongim/0001` webhook HMAC 签名）无需改动即可应用；用新二进制跑通：`go test -race ./...`（含 imgw/imwebhook 契约测试、Web IM SDK 互通测试）、暴露面检查、本地 CI 全套（结果见提交说明）。这也解决了 `docs/architecture/review-architecture-independent.md` 的 M-3（WuKongIM 不是"tagged version"）。
- 注意：ADR-0006、ADR-0010、risk-register R-21、architecture-as-is 里的"源码核对于 7b810e08"是当时的审计证据。复核：`internal/access/api`（R-21 的 `/message/send`、`/user/token` 无鉴权、ADR-0010 的无消息搜索接口所依据的目录）在 7b810e08 与 21d905a3 之间**零改动**，这几条结论在 beta.30 上同样成立；beta.30 其余 182 个文件的改动（网络层、集群、保留策略等）没有重新审计。
