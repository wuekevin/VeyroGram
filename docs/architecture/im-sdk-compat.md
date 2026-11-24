# IM SDK ↔ WuKongIM 兼容矩阵（Story 1.11，AD-8 / AD-17）

服务端 WuKongIM 固定在 `tools/build_wukongim.sh` 的 `PIN`（现为 `21d905a3` = 上游标签 `v3.0.0-beta.30`，2026-11-24；此前是 `main@7b810e08`）加 `patches/wukongim/` 补丁队列。各端 IM SDK 版本固定如下；升级任一方必须先让对应互通测试通过。

| 端 | IM SDK | 固定方式 | 与服务端互通测试 |
|---|---|---|---|
| Web / 桌面 | `wukongimjssdk` **1.2.11**（Web 客户端 `yarn.lock` 解析出的版本） | `tests/imsdk/web/package.json` 精确版本 + `package-lock.json`（integrity，仅官方源） | **有**：`services/api/internal/platform/testim/imsdk_web_test.go`，用真实 SDK 对真实 WuKongIM（固定 SHA+补丁）：有效令牌握手成功、错误令牌被拒、一个 SDK 客户端发的消息被另一个 SDK 客户端收到。CI 的 `go` 作业里跑（`VG_REQUIRE_IMSDK=1`，缺前置条件即失败而不是跳过） |
| Android | `com.github.WuKongIM:WuKongIMAndroidSDK:1.5.0`（jitpack） | `wkbase/build.gradle` 固定，OSV 审计覆盖 | **有**：`services/api/internal/platform/testim/android_imsdk_test.go` + `tests/imsdk/android`（探针应用，插桩测试），在 API 37 arm64 模拟器上用真实 SDK 经 `adb reverse` 连宿主机上的真实 WuKongIM（固定 SHA+补丁）：有效令牌连接成功（含会话同步阶段）、错误令牌始终连不上、**跨 SDK 双向消息**（Web 发→Android 收，Android 发→Web 收，均有发送确认）。一键运行 `tests/imsdk/android/run.sh`。**CI 里暂未运行**（需要带加速的模拟器，见下） |
| iOS | `WuKongIMSDK`（上游仓库内 `Modules/WuKongIMiOSSDK`，随上游 PIN 固定） | 上游 PIN | **有**：`services/api/internal/platform/testim/ios_imsdk_test.go` + `tests/imsdk/ios`（探针应用，源码取自 `apps/ios/PIN` 的 `Modules/WuKongIMiOSSDK`，依赖 CocoaAsyncSocket/FMDB+SQLCipher/25519 由 `Podfile.lock` 固定），在 iPhone 17 / iOS 26.5 模拟器上连宿主机真实 WuKongIM（固定 SHA+补丁）：有效令牌连接成功（含会话/离线同步阶段）、错误令牌**明确上报断开+认证失败（原因码 2）且不连接**、**跨 SDK 双向消息**（Web 发→iOS 收，iOS 发→Web 收，均有发送确认）。一键运行 `tests/imsdk/ios/run.sh`。**CI 里暂未运行**（需要 macOS 与模拟器） |

## 注意
- 升级 `wukongimjssdk`：改 `tests/imsdk/web/package.json`，重新生成锁文件，跑 `go test -run WebSDK ./internal/platform/testim/`；同时确认 Web 客户端实际用的版本一致（目前靠人工对照 `apps/web` 补丁里的锁文件，没有自动校验）。
- 升级 WuKongIM 的 PIN：重跑 `imgw` 与本测试。
- Android 测试的限制与观察：
  - 探针里 SDK 版本是手工与 `wkbase/build.gradle` 保持一致的（目前没有自动校验）。
  - 会话同步由应用经 HTTP 应答；本测试没有业务接口，所以探针对 SDK 答复“无会话可同步”（否则 SDK 停在同步状态不发消息）。**真实应用的会话同步接口是否与 SDK 对得上，不在本测试范围内**。
  - 错误令牌：Android SDK 1.5.0 对 WuKongIM 不会明确上报认证失败，而是被服务端断开后按退避策略一直重连；契约只断言“永远连不上”。**含义：令牌失效时客户端会无限重连，必须由应用层的 HTTP 401 处理来登出**（Story 1.14/1.17 需覆盖）。
  - 探针必须在主线程初始化 SDK（它用调用线程的 Looper）；应用声明 INTERNET 权限（忘了会得到 “Operation not permitted”）。
  - 没有覆盖：断线重连成功、离线消息、群聊、TLS（明文 TCP 到回环）、Android 6.x（只在 API 37 镜像上跑过）。
- iOS 测试的限制与观察：
  - SDK 版本即上游 PIN 的源码，无需单独对齐；但模拟器构建把设备专用的 AMR 编解码静态库换成了空桩（语音编解码没被测，和 `tools/build_ios.sh` 同）。
  - 与 Android 相同：会话同步和离线消息由应用经 HTTP 应答，探针对它们答复“没有”，真实接口是否对得上不在本测试范围内。
  - 与 Android 的行为差异：**iOS SDK 对错误令牌会明确上报认证失败，Android SDK 不会而是无限退避重连**——两端的“令牌失效”处理不能共用同一套假设。
  - 没有覆盖：断线重连成功、离线消息、群聊、TLS（明文 TCP 到回环）、真机、iOS 12 到 18 的旧系统（只在 iOS 26.5 模拟器上跑过）。
- Web 测试没有覆盖：断线重连、离线消息、群聊、大消息、消息回执之外的协议特性、TLS（测试走明文 ws 到回环地址）。
