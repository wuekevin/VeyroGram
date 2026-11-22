# Android 客户端（上游 TangSengDaoDaoAndroid + VeyroGram 补丁队列）

构建：`tools/build_android.sh <不存在的目录>`（需要 JDK 17、Android SDK；JDK 25 与 Gradle 8.13 不兼容）。
脚本在 PIN 处建一个一次性 worktree，打补丁，跑源码策略门禁、Gradle 依赖审计、`assembleDebug`、各模块 JVM 单测、APK 身份检查，最后输出 APK 的 sha256。上游检出本身从不被修改。

## 补丁 0001 做了什么
- 包名：`namespace` 固定为 `com.veyrogram.app`（R/ViewBinding 在该命名空间生成，不再依赖 `local.properties` 里的 `applicationId`，也不再有 `import com.test.ts.R` 这种硬编码）。`applicationId` 默认同值，可用 `-PvgApplicationId=` 覆盖。**临时值，等 PRD OQ-1 定最终包名。**
- 去掉上游厂商标识：`app/google-services.json`（含厂商 Firebase 项目 `tsdddemo`）、`PushKeys` 里的小米/华为/OPPO 凭据、`AndroidManifest` 里的华为/vivo 应用 ID 与密钥。没有运营方提供的 `google-services.json` 时 FCM 关闭，构建照常通过。
- 依赖安全：`fastjson 1.2.83`（RCE，1.x 无修复）换成 `fastjson 2.0.66`（同一 `com.alibaba.fastjson` API 的兼容层）；`jsoup` 1.21.2 → 1.24.1；强制 `guava 33.8.0-android`。为 fastjson2 把 **minSdk 24 提到 26**（Android 8.0+）。
- 构建：去掉 `jcenter()` 与 `mvnrepository.com` 仓库；Gradle wrapper 增加 `distributionSha256Sum`；给每个模块补 JUnit（上游的占位单测原本编译不过）；新增 3 个单测（身份、推送凭据为空、fastjson 兼容 API）。
- 应用名 `demo` → `VeyroGram`。

说明：删除 `google-services.json`、重写 `PushKeys.java` 与 `wkpush` 的 `AndroidManifest.xml` 不放进补丁（补丁会带出被删掉的厂商凭据旧值），改由 `build_android.sh` 删除文件并覆盖 `apps/android/overlay/`。

## 门禁
`tools/check_android.py`（源码策略）、`tools/audit_gradle.py`（OSV，高危/严重为 0，未知严重度按高危处理）、`tests/ops/test_mobile_gates.py`（负面测试）。

## 验证过的 / 没验证的
见 `docs/stories/1.9-*.md` 的 Dev Agent Record。

**启用 FCM：** 构建时设置 `VG_GOOGLE_SERVICES_JSON` 指向你自己的 `google-services.json`，脚本校验包名与非厂商项目后复制进 `app/`；未设置则 FCM 关闭。

**传输与备份加固（补丁 0002）：** 上游的 `SSLSocketClient` / `UnsafeOkHttpClient` 对所有证书和主机名都放行（可被中间人攻击），`BridgeWebViewClient` 遇到证书错误也继续加载，清单里 `allowBackup=true` 且允许明文 HTTP。0002 删除了前两者（改用系统默认校验）、WebView 遇证书错误改为取消、`allowBackup=false` 并用 `dataExtractionRules`/`fullBackupContent` 排除全部备份与设备迁移、主清单禁用明文（仅 `src/debug` 放开，因为默认 API 地址是 http://127.0.0.1:8080）。`check_android.py` 与 `build_android.sh`（检查合并后的 APK 清单）会拦回退。
