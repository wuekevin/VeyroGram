# IM SDK ↔ WuKongIM 兼容矩阵（Story 1.11，AD-8 / AD-17）

服务端 WuKongIM 固定在 `tools/build_wukongim.sh` 的 `PIN`（现为 `21d905a3` = 上游标签 `v3.0.0-beta.30`，2026-11-24；此前是 `main@7b810e08`）加 `patches/wukongim/` 补丁队列。各端 IM SDK 版本固定如下；升级任一方必须先让对应互通测试通过。

| 端 | IM SDK | 固定方式 | 与服务端互通测试 |
|---|---|---|---|
| Web / 桌面 | `wukongimjssdk` **1.2.11**（Web 客户端 `yarn.lock` 解析出的版本） | `tests/imsdk/web/package.json` 精确版本 + `package-lock.json`（integrity，仅官方源） | **有**：`services/api/internal/platform/testim/imsdk_web_test.go`，用真实 SDK 对真实 WuKongIM（固定 SHA+补丁）：有效令牌握手成功、错误令牌被拒、一个 SDK 客户端发的消息被另一个 SDK 客户端收到。CI 的 `go` 作业里跑（`VG_REQUIRE_IMSDK=1`，缺前置条件即失败而不是跳过） |
| Android | `com.github.WuKongIM:WuKongIMAndroidSDK:1.5.0`（jitpack） | `wkbase/build.gradle` 固定，OSV 审计覆盖 | **无**：需要在模拟器里跑带 SDK 的插桩测试，未做 |
| iOS | `WuKongIMSDK`（上游仓库内 `Modules/WuKongIMiOSSDK`，随上游 PIN 固定） | 上游 PIN | **无**：iOS 应用已能在模拟器启动，但没有连服务端的测试，未做 |

## 注意
- 升级 `wukongimjssdk`：改 `tests/imsdk/web/package.json`，重新生成锁文件，跑 `go test -run WebSDK ./internal/platform/testim/`；同时确认 Web 客户端实际用的版本一致（目前靠人工对照 `apps/web` 补丁里的锁文件，没有自动校验）。
- 升级 WuKongIM 的 PIN：重跑 `imgw` 与本测试。
- Web 测试没有覆盖：断线重连、离线消息、群聊、大消息、消息回执之外的协议特性、TLS（测试走明文 ws 到回环地址）。
