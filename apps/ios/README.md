# iOS 客户端（上游 TangSengDaoDaoiOS + VeyroGram 补丁队列）

构建：`tools/build_ios.sh <不存在的目录>`（macOS + Xcode 26；需要 Metal 工具链：`xcodebuild -downloadComponent MetalToolchain`，约 690MB）。无签名；产出模拟器 `.app`、未签名真机 `.app` 与 `VeyroGram-unsigned.ipa`。

## 补丁 0001
- Xcode 26 下 AFNetworking 4.0.1 因私有头 `netinet6/in6.h` 编译失败（4.0.1 已是 CocoaPods 上最新发布版，没有可升级的版本），按上游 master 的做法删掉这两处 import（`Pods/` 是随上游一起提交的源码，所以直接打在其上）。
- 身份：包名 `com.xinbida.tangsengdaodao*` → `com.veyrogram.app*`（临时，待 PRD OQ-1）；去掉厂商开发者团队 ID `H8PU463W68`；去掉 `applinks:githubim.com`；URL scheme `botgate` → `veyrogram`（与代码里的 `appSchemaPrefix` 一致，上游两者本就不一致）；邀请文案去掉厂商网址。
- API 地址：去掉写死的 `api.botgate.cn`。默认 `127.0.0.1:8080`（http），用 `-DVG_SERVER_IP=@"host" -DVG_HTTPS_ON=1` 覆盖。

## 验证补充：在模拟器里启动时发现的问题
- **Bugly（腾讯崩溃上报）**：保留（产品决定）。上游写死的厂商应用 ID `82f8dd98ff` 会把崩溃和用户 uid 报到厂商账号，已去掉；改为读 `Info.plist` 的 `VGBuglyAppId`，留空则不启动。
- **YYImage 自带的 2015 年 `WebP.framework`**（只有真机 arm64 切片）改为链接 libwebp pod，并把 libwebp 升到 1.6.0（修复 CVE-2023-4863）。
- **模拟器启动被挡住**：iOS 26 的模拟器只运行 arm64，而 `Modules/WuKongIMiOSSDK/.../private/arm/lib/libopencore-amr*.a`（AMR 语音编解码，预编译）里的 arm64 是真机切片，链接 arm64 模拟器会报 `built for 'iOS'`。当前模拟器构建只有 x86_64，装不进 iOS 26 模拟器（`Failed to find matching arch`）。要解决需要从源码重建这些静态库为 xcframework（或换掉 AMR 依赖），没做。所以 **iOS 应用至今没有真正启动过**，只验证了编译与链接（x86_64）。

## 已知未解决（诚实记录）
- OSV 不覆盖 CocoaPods 生态，所以 iOS 没有自动依赖审计。libwebp 已升到 1.6.0；其余 Pods（如 SQLCipher 4.5.4、SSZipArchive 2.2.3、SDWebImage 5.9.5）版本偏旧，未逐个核对公告。
- Pods 里三个来自 `github.com/tangtaoit/*` 的个人 fork（YBImageBrowser、YYImage、AsyncDisplayKit、librlottie）来源未审计。
- 没有单元测试目标（工程只有 App 与两个通知扩展），所以“iOS 单元测试”未做。
- 产物可复现性见故事记录。

**传输与存储加固（补丁 0002）：** 上游把 ATS 整个关掉（`NSAllowsArbitraryLoads=true`）；API 客户端、图片下载（SDWebImage 的 allow-invalid 选项被几十处调用传入）、视频播放都信任任意证书且不校验主机名；登录令牌和 IM 令牌以明文存在 `NSUserDefaults`（会进 iTunes/iCloud 备份）。0002：ATS 开启并只放行本地网络（开发用 `http://127.0.0.1`），API/图片/视频恢复系统证书与主机名校验，会话改存钥匙串（`AfterFirstUnlockThisDeviceOnly`，不进备份、不迁移设备；旧版 UserDefaults 里的会话启动时迁入并删除；全新安装时清掉可能残留的钥匙串旧令牌）。`check_ios.py` 拦这些回退。

**模拟器启动（已解决）：** `libopencore-amr*.a`、`libvo-amrwbenc.a` 与 `Bugly.framework` 的 arm64 切片是真机的，iOS 26 模拟器只跑 arm64，链接会报 `built for 'iOS'`。`build_ios.sh` 仅在**模拟器构建**时把它们换成同符号的桩库（AMR 为弱符号空实现，Bugly 为空的 `Bugly`/`BuglyConfig`），并加 `ARCHS=arm64 EXCLUDED_ARCHS=`（Podfile 里原本把模拟器 arm64 排除了）；真机构建前用 `git checkout` 还原真实库。所以：模拟器里没有语音消息编解码和崩溃上报；真机包用的是真实库。结果：模拟器构建能装进 iPhone 17 模拟器（iOS 26.5）并启动到登录页，进程保持运行。
