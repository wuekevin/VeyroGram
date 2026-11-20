# 唐僧叨叨上游文档审计：客户端（Android / iOS / Web）与 guide 部分

- 审计范围：`upstream/TangSengDaoDaoDocs/src` 下 `index.md`、`dev/android`（8 页）、`dev/ios`（8 页）、`dev/web`（7 页）、`guide`（7 页），共 31 页，全部逐页完整阅读，每页标记 `READ_LOCAL`。
- 性质：只读审计，未修改 `upstream/` 下任何内容。
- 关于提交号：该目录不是 git 仓库（`Is a git repository: false`），因此本次未能在本地核实 commit `41e11cb`。以任务描述为准，此点待核实。
- 约定：只转述页面所述；带「推断」的为审计者推断，非页面原文。
- 图片（png/jpg）未解析；凡页面靠截图说明的步骤，只能记录「页面引用了截图」。

---

## 一、根页面

### `index.md`  [READ_LOCAL]
- 标题：唐僧叨叨（VitePress `layout: home`，hero 文案「让企业轻松拥有自己的即时通讯」）。
- 摘要：官网首页。宣称「历时八年」「运营级别」的开源 IM，通讯由 WuKongIM 提供。列出 6 个特性卡片，并有 3 个入口按钮：什么是唐僧叨叨、GitHub、私有化部署。
- 具体事实：
  - 按钮链接：`/guide/`；`https://github.com/TangSengDaoDao/TangSengDaoDaoServer`；`/guide/quickstart`（按钮主题 `cta buy`，文案「私有化部署」）。
  - 特性：Apache2.0（可商用）；服务端、APP、Web/PC 源码全部开源；tcp + ack 保证消息必达；「私有二进制加密协议、DH+流式加密」；1 个移动端 + 多个 Web/PC 端同时在线并同步；消息永久存储（WuKongIM 自研消息 db）。
  - 页尾脚本 `fetchReleaseTag` 来自 `../.vitepress/utils/fetchReleaseTag.js`，页面加载后执行，具体做什么页面未说明。
- 阻碍换品牌的项：标题、hero 文案、GitHub 链接、`titleTemplate` 均写死「唐僧叨叨」。这是文档站，不是客户端本体，影响有限（推断）。
- 安全相关：「内容安全」仅一句宣传，没有给出加密协议细节。
- 矛盾 / 缺口：首页称「全部开源」，而 android 页的 AAR/源码引入页称群管理等模块是「购买的收费模块」，两者并存（见 `dev/android/aar_import.md`、`code_import.md`）。「私有化部署」按钮指向快速搭建页，而不是价格页。

---

## 二、dev/android（8 页）

### `dev/android/intro.md`  [READ_LOCAL]
- 标题：介绍
- 摘要：Android 端是模块化结构；每个模块在 `app` 主 module 里用一句代码引入。分层为：启动骨架 -> 唐僧叨叨层 -> `wkbase` 基础模块，通讯层依赖悟空IM。
- 具体事实：示例 `WKLoginApplication.getInstance().init(this)`；通讯层依赖 `https://githubim.com`；引用 `android_architecture.png`。
- 换品牌阻碍：无直接项。模块前缀 `wk`、`WK*Application` 类名属于上游命名。
- 安全：无。
- 矛盾 / 缺口：此处示例为 `getInstance().init(this)`，而 `code_import.md` / `aar_import.md` 写 `getInstance().init()`（无参数），`module.md` 的自建模块又用 `WKFileApplication.instance.init(this)`。三种写法不一致。

### `dev/android/structure.md`  [READ_LOCAL]
- 标题：项目结构
- 摘要：列出 Android 工程模块：`app`、`wkbase`、`wkuikit`、`wklogin`、`wkpush`、`wkscan`，并说明各自职责。重点是 `wkpush` 的厂商推送 key 替换方法。
- 具体事实：
  - `app`：启动页、所有权限注册、各模块初始化、初始化 API 地址。
  - `wkbase`：含 `WKBaseApplication`（网络库、本地 db 初始化）、`WKChatBaseProvider`（消息 item 基类，处理气泡、头像、间距）。
  - `wkuikit`：`ChatActivity`、`ChatFragment`，并含首页、联系人、我的页面。
  - `wklogin`：登录注册、第三方授权登录、改密码、授权 pc/web 登录。
  - `wkscan`：扫码加好友、入群。
  - 推送（`wkpush`）集成 华为、小米、vivo、oppo 四家厂商推送：
    - 华为：在官方开通推送后，下载 `agconnect-services.json` 覆盖 `app` 模块下同名文件；`wkpush` 的 `AndroidManifest.xml` 中 `com.huawei.hms.client.appid` 的 meta-data 换 appID；`PushKeys` 文件中换 `huaweiAPPID`。
    - 小米：`PushKeys` 中 `xiaoMiAppID`、`xiaoMiAppKey`。
    - OPPO：`PushKeys` 中 `oppoAppKey`、`oppoAppSecret`。
    - VIVO：`AndroidManifest.xml` 中 `com.vivo.push.api_key` 与 `com.vivo.push.app_id` 两个 meta-data 的 value。
  - 其他厂商推送「按官方文档在此模块集成」。
- 换品牌阻碍：上述全部推送 appID / appKey / appSecret / `agconnect-services.json` 都是上游的，换品牌必须全部换成自己申请的；且华为/小米/OPPO/VIVO 的应用均与包名绑定（推断，页面未说明绑定关系）。
- 安全：推送 key、secret 放在源码 `PushKeys` 与 manifest 中（明文，页面所述方式）。页面未提及如何防止它们进入公开仓库（缺口）。
- 矛盾 / 缺口：只提到四家国内厂商推送，未提 FCM（海外 Google 推送）、也未提地图、第三方登录 SDK 的 key；但 `wklogin` 又写「第三方授权登录」，没说明用的是哪家 SDK、需要什么 key。`app` 被说成「所有权限注册」，具体权限未列出。

### `dev/android/build.md`  [READ_LOCAL]
- 标题：项目编译
- 摘要：源码编译、改应用名、改包名、改 Logo、签名打包的步骤。
- 具体事实：
  - 版本徽章：platform android；compileSdkVersion 33；minSdkVersion 23。
  - 获取：`git clone https://github.com/TangSengDaoDao/TangSengDaoDaoAndroid.git`，用 Android Studio 打开 `TangSengDaoDaoAndroid`。
  - 改名：`app/src/main/res/values/strings.xml` 中 `app_name`。
  - 改包名：第一步，在 `app` 的 `java` 目录下用系统文件夹改目录结构；第二步，复制 `app/build.gradle` 中 `namespace` 的包名，用 `Replace in Files`（In Project）全局替换为新包名。
  - 改 Logo：替换 `app` 模块 `res` 下的 `ic_logo.png`。
  - 打包：`Build -> Generate Signed Bundle/APK`，选/建 keystore，产物 `app-release.apk`；提醒保管密钥文件和密码。
- 换品牌阻碍：包名（`namespace`）、`app_name`、`ic_logo.png`。包名替换仅靠全局字符串替换，页面没说明 `applicationId` 是否同步（该页只提 `namespace`，缺口）。
- 安全：提醒保管签名密钥，没有说明密钥不要提交仓库。
- 矛盾 / 缺口：
  - 没有写 JDK、Gradle、AGP、Android Studio 版本要求。
  - 只给了 `ic_logo.png`，没说启动图标（mipmap）、通知图标、启动页等。
  - 包名替换后，各厂商推送、`agconnect-services.json` 与包名需要匹配，页面没有提醒这一联动（推断）。
  - 没有 CI / 命令行（`./gradlew`）构建方式。

### `dev/android/config.md`  [READ_LOCAL]
- 标题：配置
- 摘要：配置服务器地址与音视频（RTC/TURN）服务器地址。
- 具体事实：
  - 服务器地址：`TSApplication.kt` 的 `initApi` 方法里 `WKApiConfig.initBaseURL("http://xx.x.xx.x:8090")`，改为自己的 IP 或域名。
  - 音视频：`TSApplication.kt` 的 `initAll()` 中 `WKUIRTCApplication.init(getList())`；`getList()` 构造 `IceServer`，URL 形如 `turn:xx.xx.xx.x:xxxx?transport=udp`，并写死 `setUsername("xxx")`、`setPassword("xxx")`。
- 换品牌阻碍：服务器地址、TURN 地址与账号密码均要替换；这些配置写在源码里。
- 安全：示例默认 `http://`（明文）；TURN 用户名/密码写死在客户端源码里，任何拿到 APK 的人都能取出（推断）；示例仅一个 UDP 的 TURN，未提 TLS（`turns:`）。
- 矛盾 / 缺口：Android 写端口 `8090`，iOS 写 `SERVER_IP` 带 `:8090`，Web 写 `:8090/v1/`；但 `guide/quickstart.md` 说「在 App 设置里改服务器地址」，说明运行时也可改，两种方式并存，页面没解释区别。TURN 服务如何部署页面未说明（属 backend 文档）。

### `dev/android/module.md`  [READ_LOCAL]
- 标题：模块开发
- 摘要：讲 Android 模块的创建与模块间通信规则（EndpointManager）。
- 具体事实：
  - 创建模块三步：新建 Module（填模块名、包名）-> 入口类（如 `WKFileApplication`，单例 + `init(context)`）-> 模块 `build.gradle` 加 `api project(':wkbase')`，`app` 里加 `api project(':模块名称')` -> 在主 `Application` 里 `WKFileApplication.instance.init(this)`。
  - 模块间通信：`EndpointManager.getInstance().setMethod("show_login_view") {...}` 注册，`EndpointManager.getInstance().invoke("show_login_view", this)` 调用；两侧 sid 必须一致。示例：`wklogin` 注册跳登录页，`wkuikit` 点「退出登录」调用。
  - 提供 Kotlin 与 Java 两种写法。
- 换品牌阻碍：无。
- 安全：无。
- 矛盾 / 缺口：正文写「`setMethod` 和 `invokeMethod` 的 sid 必须一致」，但代码里调用的是 `invoke`，没有 `invokeMethod`（命名不一致）。Java 示例中 `setMethod` 回调里注册的 lambda 没有判断 `object instanceof Context`（Kotlin 版有判断）。页面引用 `module.png`。

### `dev/android/code_import.md`  [READ_LOCAL]
- 标题：源码引入
- 摘要：购买收费模块后，以源码方式引入，示例为群管理模块 `wkgroupmanage`。
- 具体事实：
  1. 把 `wkgroupmanage` 源码放在项目根目录，`settings.gradle` 末尾加 `include ':wkgroupmanage'`。
  2. `app/build.gradle` 加 `implementation project(path: ':wkgroupmanage')`，Sync Now。
  3. `Application` 中 `WKGroupManageApplication.getInstance().init()`。
- 换品牌阻碍：收费模块源码包名/资源若带上游品牌，需要同步替换（推断，页面没说）。
- 安全：无。
- 矛盾 / 缺口：页面写明「如果你购买了…收费模块」，但未说明购买渠道与授权条款（价格页在 `price/`，不在本次范围）。此页 `init()` 无参，与 `intro.md` 的 `init(this)` 不同。

### `dev/android/aar_import.md`  [READ_LOCAL]
- 标题：AAR（执行文件）引入
- 摘要：购买收费模块后，以 aar 二进制方式引入，示例仍是群管理模块。
- 具体事实：
  1. 在项目 `MyLibs` 目录下新建文件夹 `wkgroupmanage`，放入 `wkgroupmanager_v1.aar`，并新建 `build.gradle`，内容为 `configurations.maybeCreate("default")` 与 `artifacts.add("default", file('wkgroupmanager_v1.aar'))`；强调 file 名必须与 aar 文件名一致。
  2. `settings.gradle` 末尾加 `include ':MyLibs:wkgroupmanager'`。
  3. `app/build.gradle` 加 `implementation project(path: ':MyLibs:wkgroupmanage')`，Sync Now。
  4. `Application` 中 `WKGroupManageApplication.getInstance().init()`。
- 换品牌阻碍：aar 是上游编译好的二进制，其内部包名、资源、可能写死的品牌文案/key 无法由客户端侧替换（推断）。这是与「开源可二开」对立的一点。
- 安全：aar 为不可审计的二进制（推断），来源与签名校验页面未提。
- 矛盾 / 缺口：文件夹名不一致，页面本身自相矛盾——第一步的目录是 `wkgroupmanage`，第二步 include 的是 `:MyLibs:wkgroupmanager`，第三步依赖写的是 `:MyLibs:wkgroupmanage`；而目录名决定 Gradle 项目路径，所以第二步与第三步不可能同时正确（推断，需以实际构建核实）。另 aar 文件名 `wkgroupmanager_v1.aar`（带 r）与目录名（无 r）也不一致。页面缩进有多余空格（`### ` 前有空格），属排版。

### `dev/android/question.md`  [READ_LOCAL]
- 标题：常见问题
- 摘要：占位页，只有一句「以后关于 Android 一些常见问题会整理到这里」。
- 具体事实：无 FAQ 条目。
- 其余项：无。缺口：Android 没有任何 FAQ，而 iOS、Web 各有 1 条。

---

## 三、dev/ios（8 页）

### `dev/ios/intro.md`  [READ_LOCAL]
- 标题：介绍
- 摘要：iOS 端模块化，每个模块是独立 pod，在 Podfile 里一句配置引入；分层与 Android 相同，通讯层依赖悟空IM。
- 具体事实：示例 `pod 'WuKongLogin', :path => './Modules/WuKongLogin'  ## 登录模块`；引用 `ios_architecture.png`；链接 `https://githubim.com`。
- 换品牌阻碍：无直接项；pod 名带 `WuKong` 前缀。
- 安全：无。
- 矛盾 / 缺口：介绍与 Android 一致，无冲突。

### `dev/ios/structure.md`  [READ_LOCAL]
- 标题：项目结构
- 摘要：列出 iOS 工程目录。
- 具体事实：`Modules/`：`WuKongBase`（基础、消息 UI、会话列表、消息列表、网络封装）、`WuKongContacts`（好友、群组、黑名单）、`WuKongDataSource`（给 SDK 提供用户、群组、会话数据）、`WuKongLogin`（登录、注册、找回密码）；`TangSengDaoDaoiOS` 为程序入口（启动页、项目 Logo 等）。
- 换品牌阻碍：入口 target 名 `TangSengDaoDaoiOS`、workspace 名 `TangSengDaoDaoiOS.xcworkspace`、启动页、Logo。
- 安全：无。
- 矛盾 / 缺口：iOS 只有 4 个模块，没有扫码、推送、UI 套件这类模块，而 Android 有 `wkpush`、`wkscan`、`wkuikit`。iOS 推送（APNs）、扫码、聊天 UI 放在哪一模块，页面没有说（`WuKongBase` 含消息 UI，部分可对应；推送与扫码无说明）。

### `dev/ios/build.md`  [READ_LOCAL]
- 标题：iOS / 项目编译
- 摘要：克隆、`pod install`、打开 workspace。
- 具体事实：`git clone https://github.com/TangSengDaoDao/TangSengDaoDaoiOS.git`；`cd TangSengDaoDaoiOS`；`pod install`；`open TangSengDaoDaoiOS.xcworkspace`。
- 换品牌阻碍：仓库、workspace 名。
- 安全：无。
- 矛盾 / 缺口：未写 Xcode、iOS 最低版本、CocoaPods 版本、Ruby 版本。没有 bundle id、签名证书、URL scheme、推送证书相关说明。

### `dev/ios/config.md`  [READ_LOCAL]
- 标题：配置
- 摘要：说明所有配置项在 `WKApp.shared.config` 对象属性中，并给出服务器地址配置。
- 具体事实：`AppDelegate.m` 中 `#define SERVER_IP @"xx.x.xx.x:8090"`、`#define HTTPS_ON false`（https 开关）。
- 换品牌阻碍：服务器地址、https 开关。
- 安全：示例默认 `HTTPS_ON false`（明文）；页面未提 ATS 例外配置（缺口，推断：iOS 默认禁止明文 HTTP，若页面所述方式生效，需要额外 ATS 设置，未说明）。
- 矛盾 / 缺口：iOS 的服务器地址是「IP 加端口，无协议头 + 单独 https 开关」，Android 是「完整 URL」，Web 是「完整 URL 且需 `/v1/` 结尾」，三端格式不同。iOS 没有 RTC/TURN 配置的说明（Android 有），不知 iOS 是否提供音视频。

### `dev/ios/module.md`  [READ_LOCAL]
- 标题：模块开发
- 摘要：iOS 模块宗旨、创建、规则、podspec、入口文件、模块方法。
- 具体事实：
  - 创建：`cd Modules` + `pod lib create 模块名`；必须删除新建模块下的 `.git` 文件夹，否则会搞乱项目 git 结构；Podfile 加 `pod '模块名', :path => './Modules/模块名'`，再 `pod install`。
  - podspec：`s.source_files = '模块名/Classes/**/*'`；`s.resource_bundles = {'模块名_images' => ['模块名/Assets/Images.xcassets']}`；`s.resources = ['模块名/Assets/Lang']`（多语言）；`s.dependency 'WuKongBase'`。
  - 入口：`WKLoginModule : WKBaseModule`；宏 `@WKModule(WKLoginModule)`（类名必须与文件名一致，否则模块不会被注册）；必须实现 `moduleId`（建议 `WuKong` + 模块名）与 `moduleInit:`；可选：`moduleDidFinishLaunching:`、`moduleDidDatabaseLoad:`（可在此建业务表）、`moduleType`（`WKModuleTypeDefault` / `WKModuleTypeResource`）、`moduleSort`（越大越靠前）。
  - 模块方法：`[self setMethod:WKPOINT_LOGIN_SHOW handler:...]`，`WKPOINT_LOGIN_SHOW` 为 `WuKongBase` 里定义的注册 ID，`WKNavigationManager shared resetRootViewController:`。
- 换品牌阻碍：`moduleId` 建议以 `WuKong` 开头，自建模块可自行命名（页面原话为「建议」）。
- 安全：无。
- 矛盾 / 缺口：示例代码 `return true` 缺分号（文档笔误）；`pod install` 后一段 `pod install` 单独出现，意义不明；`moduleType` 的 `WKModuleTypeResource` 页面说「后续有需要再细说」，未说明。

### `dev/ios/moduleimport.md`  [READ_LOCAL]
- 标题：模块引入
- 摘要：以收费的群管理模块 `WuKongGroupManager` 为例，如何引入 iOS 模块（页面此处没有写「购买」，但用的是群管理模块，推断同为收费模块）。
- 具体事实：1) 目录复制到项目 `Modules` 目录；2) Podfile 加 `pod 'WuKongGroupManager', :path => './Modules/WuKongGroupManager'  ## 群管理`；3) 根目录 `pod install`，打开 `TangSengDaoDaoiOS.xcworkspace` 运行验证。
- 换品牌阻碍：workspace 名。
- 安全：无。
- 矛盾 / 缺口：与 Android 不同，iOS 引入后没有 `init()` 调用，靠 `@WKModule` 宏自动注册（来自 `module.md`），一致，但页面没明说。

### `dev/ios/package_notes.md`  [READ_LOCAL]
- 标题：打包注意事项
- 摘要：两步：Podfile 的 `dev_team` 换成自己开发者账号的 team id；修改 `Pods-{product名称}-frameworks.sh`，把 `readlink "${source}"` 换成 `readlink -f "${source}"`。
- 具体事实：Podfile 变量 `dev_team`；脚本路径 `/Pods/Target Support Files/Pods-{product名称}/Pods-{product名称}-frameworks.sh`。
- 换品牌阻碍：`dev_team`（Apple 开发者 team id）。
- 安全：无。
- 矛盾 / 缺口：该脚本位于 `Pods/` 下，`pod install` 会重新生成，改动可能被覆盖（推断，页面未说明如何持久化，例如用 Podfile post_install）。bundle id、证书、描述文件、推送证书页面均未提。

### `dev/ios/question.md`  [READ_LOCAL]
- 标题：常见问题
- 摘要：仅 1 条 FAQ。
- 具体事实：序号 2001：`IntermediateBuildFilesPath/UninstalledProducts/iphoneos/Aspects.framework" failed: No such file or directory`；解法同 `package_notes.md` 第二步（`readlink` 换 `readlink -f`）。
- 其余项：无。缺口：序号 2001 与 Web 的 4001 说明有编号规则，但 Android 没有条目；规则页面未写。

---

## 四、dev/web（7 页）

### `dev/web/intro.md`  [READ_LOCAL]
- 标题：介绍
- 摘要：Web 端模块化，每个模块是独立 npm 项目；在 `index.tsx` 一句话注册模块，如 `WKApp.shared.registerModule(new LoginModule())`。分层同 Android/iOS，通讯层依赖 `wukongimjssdk`。
- 具体事实：通讯 SDK 链接 `https://githubim.com/sdk/javascript.html`；引用 `web_architecture.png`。
- 换品牌阻碍：无。其余项无。

### `dev/web/structure.md`  [READ_LOCAL]
- 标题：项目结构
- 摘要：monorepo 目录：`apps/web`（主骨架）、`docs`、`packages`（`tsdaodaobase`、`tsdaodaocontacts`、`tsdaodaodatasource`、`tsdaodaologin`）。
- 具体事实：模块说明：基础聊天功能与常用组件、通讯录、悟空IM 数据源、登录注册。
- 换品牌阻碍：npm 包 scope（见 `moduleimport.md` 的 `@tsdaodao/*`）、目录名含 `tsdaodao`。
- 安全：无。缺口：没列 Electron 相关目录；而 `build.md` 又有 `yarn run dev-ele`、`build-ele:*`，Electron 主进程代码放在哪页面没说。

### `dev/web/build.md`  [READ_LOCAL]
- 标题：项目编译
- 摘要：Web、Mac、Windows、Linux 的编译与 Electron 打包。
- 具体事实：
  - 技术栈徽章：React 17.0.2、TypeScript 5.0.4、Yarn 1.22.17、Node 18.17.1、Webpack 5.88.2、Electron 26.0.0、ElectronBuilder 24.9.1、Semi UI 2.24.2、turbo 2.0.9、WukongIm 1.2.10。
  - 提示：本地开发建议 `node v18.17.1`、`yarn 1.22.17`。
  - 命令：`git clone https://github.com/TangSengDaoDao/TangSengDaoDaoWeb.git`（也有 gitee 地址）；`cd TangSengDaoDaoWeb`；`yarn install`；`yarn run dev`（web）；`yarn run dev-ele`（electron）；`yarn run build`；`yarn run build-ele:mac`、`build-ele:win`、`build-ele:linux`（页面代码块标注写作 `linx`）。
  - 打包提示：先运行 `yarn build` 编译；win 打包需要在 `apps/web` 下执行。
- 换品牌阻碍：Electron builder 的应用名/appId/图标不在此页（缺口）；仓库名。
- 安全：无。
- 矛盾 / 缺口：
  - `WukongIm 1.2.10` 徽章没说是服务端版本还是 JS SDK 版本，也与 `intro.md` 里的 `wukongimjssdk` 对不上（推断可能是 SDK 版本，需核实）。
  - win 打包「在 `apps/web` 下执行」，mac/linux 在哪里执行没说。
  - 没有 Electron 的签名、公证、自动更新说明。
  - 提示行用 code-group 名称 `[linx]` 是笔误。

### `dev/web/config.md`  [READ_LOCAL]
- 标题：项目配置
- 摘要：配置服务端地址。
- 具体事实：`apps/web/src/index.tsx` 中 `WKApp.apiClient.config.apiURL = "http://xxx.xx.xx.xx:8090/v1/"`，注意最后的斜杠。
- 换品牌阻碍：服务端地址。
- 安全：示例默认 `http://`；带 `/v1/` 版本前缀。
- 矛盾 / 缺口：Android、iOS 的地址都没有 `/v1/`，只有 Web 要求写。是客户端内部各自拼接，还是真有差异，页面没说（需对照源码核实）。

### `dev/web/module.md`  [READ_LOCAL]
- 标题：模块开发
- 摘要：只有「待补充」。
- 具体事实：无。缺口：Web 无模块开发说明。

### `dev/web/moduleimport.md`  [READ_LOCAL]
- 标题：模块引入
- 摘要：以群管理模块 `tsdaodaogroupmanager` 为例引入 Web 模块。
- 具体事实：1) 目录复制到 `packages`；2) `apps/web/package.json` 的 dependencies 加 `"@tsdaodao/groupmanager": "*"`；3) `apps/web/src/index.tsx` 中 `import { GroupManagerModule } from '@tsdaodao/groupmanager';` 并 `WKApp.shared.registerModule(new GroupManagerModule());`；4) 根目录 `yarn install` 后 `yarn run dev` 验证。
- 换品牌阻碍：npm scope `@tsdaodao`、目录名 `tsdaodaogroupmanager`。
- 安全：依赖版本 `"*"`（任意版本），页面所述写法（推断：对工作区内部包通常可接受）。
- 矛盾 / 缺口：目录名 `tsdaodaogroupmanager` 与包名 `@tsdaodao/groupmanager` 命名不一致，页面没说明对应关系。

### `dev/web/question.md`  [READ_LOCAL]
- 标题：常见问题
- 摘要：1 条 FAQ。
- 具体事实：序号 4001：直接源码运行后没有出现二维码，接口提示 `404 Not Found`；解法：检查 api 基地址，改 `apps/web/src/index.tsx` 的 `WKApp.apiClient.config.apiURL = "http://xxx.xx.xx.xx/v1/"`，末尾斜杠不能少。
- 矛盾 / 缺口：此处示例没有 `:8090` 端口，`config.md` 示例带 `:8090`；两处示例不一致（可能只是占位）。

---

## 五、guide（7 页）

### `guide/index.md`  [READ_LOCAL]
- 标题：什么是唐僧叨叨
- 摘要：产品介绍：轻量级、高性能、重安全、专注私有化部署的开源 IM。列特性、对比竞品、组成与许可证。
- 具体事实：
  - 特性：开源免费、私有化部署、消息必达（tcp + ack）、内容安全（私有二进制加密协议、DH+流式加密）、消息永久存储、多设备同步、全平台（iOS、Android、Windows、MAC、Ubuntu、Web）。
  - 竞品对比表：网易云信（群小于 5000、存储 30天/1年）、融云（3000、7天）、环信（3000、7天）、腾讯云IM（最多 6000、30天）。唐僧叨叨标注群人数「无限制」、存储「永久」、「运营级 Demo，换个皮就能直接上线运营」。
  - 组成：客户端（iOS、Android、Web、PC，后两者同为 TangSengDaoDaoWeb 仓库）；服务端（WuKongIM 通讯端 + TangSengDaoDaoServer 业务端）；管理端 TangSengDaoDaoManager。
  - 许可证：Apache-v2.0，「请遵守开源协议」。
  - 联系：微信群二维码 `tsdaodaowechat.jpg`；问题反馈 `https://github.com/TangSengDaoDao/TangSengDaoDaoServer/issues`。
- 换品牌阻碍：「换个皮就能直接上线运营」是页面原话，但页面没有给出换皮清单；换品牌实际要点见本文「换品牌清单」。
- 安全：加密只有宣传性描述；竞品对比数据（群上限、保存期）为页面单方面陈述，未给来源（推断：无法核实）。
- 矛盾 / 缺口：「全平台」中的 Ubuntu 对应 Web 页的 Linux Electron 打包；「群人数无限制」是页面声明，未与服务端文档对照（待核实）。Apache-2.0 要求保留版权与 NOTICE，页面只说「请遵守」，换品牌时的署名/版权要求未展开（推断需自行核对 LICENSE）。

### `guide/quickstart.md`  [READ_LOCAL]
- 标题：快速搭建体验
- 摘要：三步：装服务端（链接 `/dev/backend/deploy-compose`）、下载 App、改服务器地址；并说明手机号注册的隐藏入口。
- 具体事实：App 右上角设置按钮可以改服务器地址；登录页面长按「欢迎登录唐僧叨叨」进入隐藏注册页，输入手机号，默认验证码为 `123456`（不用点获取验证码）。
- 换品牌阻碍：登录页标题文案「欢迎登录唐僧叨叨」（硬编码文案，推断在语言资源中）。
- 安全：默认万能验证码 `123456`，页面提供给任何人注册；生产部署必须关闭（推断，页面没有提关闭方法，也没指向相应配置项）。隐藏入口靠长按文案，属于安全隐患的默认行为。
- 矛盾 / 缺口：`demo.md` 说「app 端在登录页面长按标题可以进入手机号登录」，而本页说长按是进入「注册页」，两页对同一手势的行为描述不同（登录 vs 注册）。

### `guide/demo.md`  [READ_LOCAL]
- 标题：Demo下载
- 摘要：提供 Android/iOS 扫码体验图、Web/Windows/Mac 下载链接和演示账号。
- 具体事实：
  - iOS：App Store 搜「唐僧叨叨」。
  - Web 体验：`https://web.botgate.cn`。
  - Windows：`https://file.githubim.com/download/TangSengDaoDao-latest.exe`；Mac：`...-latest.dmg`；Mac 新芯片：`...-latest-arm64.dmg`（均在 `file.githubim.com`）。
  - 演示账号：`15900000002` 至 `15900000006`（5 个），密码均为 `a1234567`；App 端需在登录页长按标题进入手机号登录。
  - 动画演示与 PC 截图来自 `https://file.githubim.com/screenshot/*`（webp/png）。
  - 引用本地图 `download/android.png`、`download/iOS.png`。
- 换品牌阻碍：无（这是上游演示页）。
- 安全：公开演示账号与弱密码（`a1234567`），仅限演示环境；下载链接指向第三方域名（`file.githubim.com`，推断属于 WuKongIM 项目方）；页面没有给出安装包的校验值/签名信息。
- 矛盾 / 缺口：Web 体验域名为 `web.botgate.cn`，与项目品牌域名（`tsdaodao.com`、`githubim.com`）不同，页面没解释。

### `guide/fastentry.md`  [READ_LOCAL]
- 标题：快速入口
- 摘要：列出各代码仓库的 Github/Gitee 地址和文档链接，分唐僧叨叨与 WuKongIM 两组。
- 具体事实：
  - 唐僧叨叨：TangSengDaoDaoServer（业务端：好友关系、群组、朋友圈等）、TangSengDaoDaoAndroid、TangSengDaoDaoiOS、TangSengDaoDaoWeb（Web/PC）、TangSengDaoDaoManager（后台管理，无文档）。文档域名 `https://tsdaodao.com/dev/...`，服务端部署文档 `https://tsdaodao.com/dev/backend/deploy-compose.html`。
  - WuKongIM：WuKongIM（通讯端，长连接与消息投递）、WuKongIMAndroidSDK、WuKongIMiOSSDK、WuKongIMUniappSDK、WuKongIMJSSDK、WuKongIMFlutterSDK、WuKongIMReactNativeDemo（贡献者 wengqianshan，无 Gitee、无文档）。
  - Gitee 组织：`TangSengDaoDao`、`WuKongDev`。
- 换品牌阻碍：无，但若换品牌分叉，所有克隆源均为上游仓库。
- 安全：无。
- 矛盾 / 缺口：这里说 Server 是「业务端」，包括「朋友圈」，而 `guide/index.md` 的特性表里没提朋友圈（轻微缺口）。页面没有指出各客户端依赖的 WuKongIM SDK 版本，Web 页面徽章 `WukongIm 1.2.10` 无对应。

### `guide/legal.md`  [READ_LOCAL]
- 标题：法律声明
- 摘要：三条免责：软件用于合法用途；厂商无法实时监控客户行为；客户违法需自担责任，厂商停止技术支持和服务。
- 具体事实：无技术项。
- 换品牌阻碍：无。
- 安全：无（合规提示）。
- 缺口：页面对内容审核、隐私政策、用户协议要求无说明；换品牌上线要自备（推断）。

### `guide/question.md`  [READ_LOCAL]
- 标题：常见问题
- 摘要：占位，「非技术相关的一些问题到时候写在这里」。无条目。

### `guide/support.md`  [READ_LOCAL]
- 标题：问题解决途径
- 摘要：免费途径五项（看 FAQ -> 查 issue -> 提 issue -> 加微信群 -> 购买技术支持服务）。
- 具体事实：FAQ 链接：`/dev/backend/question`、`/dev/ios/question`、`/dev/android/question`（页面写作「Adnroid」，笔误）、`/dev/web/question`、`/guide/question`；GitHub issues 地址同 `guide/index.md`；微信群二维码 `tsdaodaowechat.jpg`；付费技术支持：`/price/index.html#技术支持服务`。
- 换品牌阻碍：无。
- 缺口：Android 与 guide 的 FAQ 都是空页，用户按页面指引去查是查不到的。目录里还有 `wukongwechat.png`（WuKongIM 微信群图），没有被本次 7 页引用（推断：另外文档在用）。

---

## 六、换品牌清单（Rebranding checklist，按页面原文要求）

### Android
1. 应用名：`app/src/main/res/values/strings.xml` 的 `app_name`（`build.md`）。
2. 包名：`app/build.gradle` 的 `namespace`，并全局 Replace in Files；`java` 目录结构需手动改（`build.md`）。页面未提 `applicationId`，见待核实。
3. Logo：`res/ic_logo.png`（`build.md`）。
4. 服务器地址：`TSApplication.kt` `initApi` 里 `WKApiConfig.initBaseURL(...)`（`config.md`）。
5. TURN/RTC：`TSApplication.kt` `getList()` 里 turn URL、用户名、密码（`config.md`）。
6. 签名：自建 keystore 并保管（`build.md`）。
7. 华为推送：`agconnect-services.json`（覆盖 `app` 下同名文件）、`wkpush` 的 `AndroidManifest.xml` `com.huawei.hms.client.appid`、`PushKeys.huaweiAPPID`（`structure.md`）。
8. 小米推送：`PushKeys.xiaoMiAppID`、`xiaoMiAppKey`。
9. OPPO 推送：`PushKeys.oppoAppKey`、`oppoAppSecret`。
10. VIVO 推送：`AndroidManifest.xml` 的 `com.vivo.push.api_key`、`com.vivo.push.app_id`。
11. 其他厂商推送：在 `wkpush` 按官方文档自行集成。
12. 收费模块：源码方式或 aar 方式引入后，`Application` 里调用 `init()`（`code_import.md`、`aar_import.md`）。

### iOS
1. 服务器地址：`AppDelegate.m` 的 `SERVER_IP` 与 `HTTPS_ON`（`config.md`）。
2. 开发者 team id：Podfile 的 `dev_team`（`package_notes.md`）。
3. 打包脚本修复：`Pods-{product}-frameworks.sh` 里 `readlink` 改 `readlink -f`（`package_notes.md`、`question.md`）。
4. 入口 target 内的启动页、项目 Logo（`structure.md` 只说「程序入口含启动页、Logo」，没给具体文件）。
5. 页面没有写应用名、bundle id、URL scheme、推送证书、第三方 SDK key 的修改位置（缺口）。

### Web / PC
1. 服务端地址：`apps/web/src/index.tsx` 的 `WKApp.apiClient.config.apiURL`，末尾 `/v1/`（`config.md`、`question.md`）。
2. 引入收费模块：`packages` 复制目录、`apps/web/package.json` 加依赖、`index.tsx` 注册（`moduleimport.md`）。
3. 页面没有写应用名、图标、Electron appId、签名、自动更新地址的修改位置（缺口）。

### guide / 文档站
1. 登录页文案「欢迎登录唐僧叨叨」、默认验证码 `123456`（`quickstart.md`）：上线前需处理（验证码关闭方式页面未给）。
2. 演示账号、下载域名、微信群二维码均属上游，不应沿用（推断）。

---

## 七、待对照源码核实的事实（Facts to verify against source）

1. Android 包名：除 `namespace` 外，`applicationId` 是否在 `app/build.gradle`，以及清单、推送 SDK 配置、`agconnect-services.json` 里是否也含包名。
2. Android `aar_import.md` 中 `wkgroupmanage` 与 `wkgroupmanager` 哪个才是正确的 Gradle 项目路径。
3. Android `init()` 与 `init(this)` 的真实签名，及 `getInstance()` 与 `instance` 的真实写法（`intro`、`module`、`code_import`、`aar_import` 四页不一致）。
4. Android `TSApplication.kt` 中 `initApi`、`initAll` 是否真实存在，TURN 凭据是否硬编码。
5. `PushKeys` 文件的真实路径与字段名（`huaweiAPPID`、`xiaoMiAppID`、`xiaoMiAppKey`、`oppoAppKey`、`oppoAppSecret`），以及上游仓库里是否已含真实 key。
6. Android 是否使用 FCM、地图、第三方登录 SDK（`wklogin` 提到「第三方授权登录」）及其 key；页面完全没写。
7. Android 的 JDK/Gradle/AGP 版本；`compileSdkVersion 33`、`minSdkVersion 23` 是否与当前源码一致。
8. iOS `AppDelegate.m` 中 `SERVER_IP`/`HTTPS_ON` 是否存在；是否需要 ATS 例外；Podfile 中 `dev_team` 变量；bundle id、URL scheme、APNs、扫码、地图相关配置所在位置。
9. iOS 的 `WKApp.shared.config` 对象有哪些配置项（页面说「所有配置项查看该对象」，但没列出）。
10. iOS 是否含 RTC/音视频模块，与 Android 的 `WKUIRTCApplication` 对应关系。
11. iOS `package_notes` 脚本修改是否会在 `pod install` 后被覆盖，是否应改为 Podfile `post_install`。
12. Web：`apiURL` 是否真需要 `/v1/` 前缀，而 Android/iOS 为何不需要；`WukongIm 1.2.10` 徽章指的是什么；Electron 配置（appId、productName、图标、自动更新）所在文件。
13. Web 目录名 `tsdaodaogroupmanager` 与包名 `@tsdaodao/groupmanager` 的映射。
14. `guide/quickstart.md` 所述万能验证码 `123456` 的开关配置，以及它与 `demo.md` 里「长按标题进入手机号登录」是否同一入口。
15. 官网所称「DH+流式加密」「私有二进制协议」「群人数无限制」「永久存储」是否由 WuKongIM 实际支持（属服务端/通讯端文档范围）。
16. 收费模块（群管理等）范围与许可条款，及其与「全部开源」说法的关系（价格页不在本次范围）。
17. `index.md` 中 `fetchReleaseTag` 脚本的作用（不在本次范围，在 `.vitepress/` 下）。
18. 提交 `41e11cb` 是否确为当前 `upstream/` 检出版本（该目录非 git 仓库，本地无法验证）。
