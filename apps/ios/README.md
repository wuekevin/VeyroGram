# iOS 客户端（上游 TangSengDaoDaoiOS + VeyroGram 补丁队列）

构建：`tools/build_ios.sh <不存在的目录>`（macOS + Xcode 26；需要 Metal 工具链：`xcodebuild -downloadComponent MetalToolchain`，约 690MB）。无签名，目标 iOS 模拟器。

## 补丁 0001
- Xcode 26 下 AFNetworking 4.0.1 因私有头 `netinet6/in6.h` 编译失败（4.0.1 已是 CocoaPods 上最新发布版，没有可升级的版本），按上游 master 的做法删掉这两处 import（`Pods/` 是随上游一起提交的源码，所以直接打在其上）。
- 身份：包名 `com.xinbida.tangsengdaodao*` → `app.veyrogram.ios*`（临时，待 PRD OQ-1）；去掉厂商开发者团队 ID `H8PU463W68`；去掉 `applinks:githubim.com`；URL scheme `botgate` → `veyrogram`（与代码里的 `appSchemaPrefix` 一致，上游两者本就不一致）；邀请文案去掉厂商网址。
- API 地址：去掉写死的 `api.botgate.cn`。默认 `127.0.0.1:8080`（http），用 `-DVG_SERVER_IP=@"host" -DVG_HTTPS_ON=1` 覆盖。

## 验证补充：在模拟器里启动时发现的问题
- **Bugly（腾讯崩溃上报）**：上游启动时用厂商的应用 ID `82f8dd98ff` 初始化 Bugly，并把登录用户的 uid 上报给第三方。已移除调用、podspec 引用和预编译的 `Bugly.framework`（由 `build_ios.sh` 删除，不放进补丁）。
- **YYImage 自带的 2015 年 `WebP.framework`**（只有真机 arm64 切片）改为链接 libwebp pod。
- **模拟器启动被挡住**：iOS 26 的模拟器只运行 arm64，而 `Modules/WuKongIMiOSSDK/.../private/arm/lib/libopencore-amr*.a`（AMR 语音编解码，预编译）里的 arm64 是真机切片，链接 arm64 模拟器会报 `built for 'iOS'`。当前模拟器构建只有 x86_64，装不进 iOS 26 模拟器（`Failed to find matching arch`）。要解决需要从源码重建这些静态库为 xcframework（或换掉 AMR 依赖），没做。所以 **iOS 应用至今没有真正启动过**，只验证了编译与链接（x86_64）。

## 已知未解决（诚实记录）
- **libwebp 1.2.4**（Pods 内随附）早于 CVE-2023-4863 的修复版本 1.3.2（该漏洞已被在野利用）。OSV 不覆盖 CocoaPods 生态，所以没有自动依赖审计；升级要重新生成 `Pods/`，没做。
- Pods 里三个来自 `github.com/tangtaoit/*` 的个人 fork（YBImageBrowser、YYImage、AsyncDisplayKit、librlottie）来源未审计。
- 没有单元测试目标（工程只有 App 与两个通知扩展），所以“iOS 单元测试”未做。
- 产物可复现性见故事记录。
