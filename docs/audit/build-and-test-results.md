# 可复现构建与测试结果（Phase 0，2026-11-19，macOS arm64，Go 1.27.1 / Node 26.8.2）

| 目标 | 命令 | 结果 | 说明 |
|---|---|---|---|
| Server | `go build ./...` / `go vet ./...` | 通过 (exit 0) | 上游 go.mod 声明 go 1.20 |
| Server 测试 | `go test -timeout 90s ./...` | 4 ok / 11 FAIL / 24 无测试包 | 失败根因：本机 MySQL 拒绝默认 root 凭据（Error 1045）→ 环境阻断，**非代码缺陷结论**；需隔离 MySQL/Redis 测试实例 |
| WuKongIM | `go build ./...`(本机 go1.27.1) | **失败** | 依赖 cockroachdb/swiss 与 Go 1.27 不兼容；go.mod 要求 toolchain go1.26.10 |
| WuKongIM | `GOTOOLCHAIN=go1.26.10 go build ./cmd/... ./internal/... ./pkg/...` | 通过 | 必须锁定 Go 1.26.10；测试未运行 |
| Manager | `pnpm@12 install --frozen-lockfile` | **失败** | 新版 pnpm 供应链校验拒绝 lockfile（含 2 处 registry.npmmirror.com tarball URL） |
| Manager | `pnpm@9 install --frozen-lockfile && pnpm build` | 通过 | dist 8.2MB；无单元测试 |
| Web | `yarn@1 install --frozen-lockfile && yarn build` | 通过 | 安装 475s；yarn.lock 含 1788 处 npmmirror/taobao 镜像引用；Electron 桌面构建未运行 |
| Android | `./gradlew :app:assembleDebug`(JDK17, Gradle 8.13, AGP 8.13) | 通过 | debug APK 50MB；须在 gitignored 的 local.properties 设 `applicationId=com.test.ts`(源码硬导入 `com.test.ts.R`)，否则构建抛错；未跑 instrumentation 测试 |
| iOS | `xcodebuild -workspace … -scheme WuKongChatiOS -sdk iphonesimulator CODE_SIGNING_ALLOWED=NO build` | **失败 [BROKEN]** | Pods/AFNetworking 在 Xcode 26.6 报 `Use of private header … netinet6/in6.h`；Pods 已提交入库。工作区无 TangSengDaoDaoiOS scheme，可用 scheme 为 WuKongChatiOS |
| Docs | — | 未尝试 | 82 页未逐页阅读 |

## SCA（依赖审计，原样计数，未做可达性分析）
- Manager（pnpm audit）：critical 6 / high 72 / moderate 71 / low 11
- Web（yarn audit）：high 123 / moderate 99 / low 14 / critical 0
- Server/WuKongIM：govulncheck 未安装，**UNVERIFIED**

日志保存在会话临时目录（scratchpad），未入库。

## Server 测试复测（隔离环境，2026-11-19）
环境：共用本机 MySQL 8.0.46（独立库 veyrogram_test + 仅限该库的专用账号）、临时 Redis(6379)、本地编译的 WuKongIM(go1.26.10，端口 5001)。
上游 testutil 把 DSN 写死为 `root:demo@…/test`（库 TangSengDaoDaoServerLib testutil/test.go:31），故用 `-modfile` + 临时 replace 改 DSN，**未修改上游仓库与 go.mod，未改动 root 密码**。
- 数据库与 IM 连接已通；多包失败的根因变为：`panic: handlers are already registered for path ...`（common/group/report/statistics/user）。**单个测试隔离运行仍复现**，根因 UNVERIFIED（可能为上游路由重复注册、gin 版本行为或测试框架问题，待定位）。
- file：nil pointer；robot：硬编码 root 账号（modules/robot 内另有自己的 DSN）；webhook：厂商推送测试(HMS/MI/OPPO)依赖外部凭据；workplace：登录凭证失效；message：TestReadedMessage 超时 31s。
- 结论：上游 Server 单测在本环境**不能作为回归基线**；Phase 2 需自建可靠测试基座。pkg/* 4 个包通过。

## Server 测试 panic 根因（已定位）[FOUND_IN_SOURCE]
`testutil.NewTestServer()`（库 TangSengDaoDaoServerLib testutil/test.go:32 `module.Setup(ctx)`）已注册全部模块路由；测试随后又手动 `f.Route(s.GetRoute())`（例：modules/common/api_test.go:18），gin v1.9.1 对重复路由直接 panic。即**上游测试与其依赖库版本不同步**，非环境问题。修复方向：测试内去掉手动 Route，或固定旧版库。

## 漏洞扫描补充
- govulncheck（Server，Go 1.27.1 工具链）：41 项被调用路径命中，其中 14 项为标准库（取决于本机 Go 版本，非上游代码），第三方主要在 golang.org/x/net(9)、x/image(8)、grpc(5)、go-ethereum(5)、gomarkdown(3)。逐项可达性与升级方案待 Phase 2。
- WuKongIM（go1.26.10，cmd/internal/pkg）：2 项（含 klauspost/compress）。另：`test/e2e/suite/send_retry.go:35 undefined: WKProtoClient` 使 `go build ./...` 失败（上游缺陷）。
- Go 依赖清单：`dependency-sbom-server.txt`(508)、`dependency-sbom-wukongim.txt`(428)；npm/Pods/Gradle 的 SBOM 尚未生成。
## 其他产物
- `endpoints-inventory.yaml`：Server 静态扫描 244 条路由注册（含 group 变量名，完整路径前缀待人工核对）。
- `docs-heading-index.md`：82 页标题与体积索引（非阅读证明）。
