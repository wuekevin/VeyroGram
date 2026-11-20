# risk-register（Phase 0 初版）

| ID | 级别 | 状态 | 风险 | 证据 |
|---|---|---|---|---|
| R-01 | 中(环境，已缓解) | 已证实 | 执行期间磁盘曾降至 237MB 并中断一次构建；后恢复至 47GB。后续构建继续使用磁盘看门狗 | df 输出 |
| R-02 | 高 | 已证实 | 官方示例默认短信验证码 123456 | upstream/TangSengDaoDaoServer/configs/tsdd.yaml:40；docker/tsdd/configs/tsdd.yaml:41；Docs deploy-compose.md:199、btpanle.md:231 |
| R-03 | 高 | 已证实 | 官方部署文档默认管理员密码示例 admin1234567，用户名 superAdmin | Docs deploy-compose.md:205、btpanle.md:237 |
| R-04 | 中(用户已接受风险) | 已证实 | Manager、Web 仓库无 LICENSE 文件，二开/再分发权限不明 | 仓库根目录列表；GitHub API license=None |
| R-05 | 高 | 待核验 | Web 端 Electron 26.0.0、React 17 严重过时，可能含已知漏洞 | Web/apps/web/package.json；SCA 未运行 |
| R-06 | 中 | 已证实 | iOS 仓库提交 Pods/Vendor 二进制，来源与许可需逐项审计 | 目录存在，337MB |
| R-07 | 中 | 已证实 | 测试覆盖极低（Server 23 个 test 文件，Manager 0，Android 8） | find 统计 |
| R-08 | 中 | 已证实 | WuKongIM docker-compose 含默认 Grafana 密码 Aa12345678、默认 MCP token | WuKongIM/docker-compose.yml:121,162（仅作参考拓扑，不得照搬） |
| R-09 | 中 | 已证实 | WuKongIM CHANGELOG：channel promise 激活后不支持回滚 | WuKongIM/CHANGELOG.md:45 |
| R-10 | 低 | 已决策 | 与 VeyroGrama(SereinGram 系)同名但为两个隔离产品（用户确认）；发布前仅做包名/域名/商标核验清单 | 用户指示 |
| R-11 | 中 | 已证实 | 定价页链接异常（标签→#文件模块，客服→#旗舰模块）、单位"w"未定义；需求矩阵以明细行为准 | 抓取结果 |
| R-12 | 高 | 已证实 | Manager/Web 锁文件引用第三方镜像(npmmirror/taobao)：Manager 2 处、Web 1788 处，破坏供应链可信；pnpm 12 拒绝 Manager lockfile | build-and-test-results.md |
| R-13 | 高 | 已证实 | npm 依赖漏洞：Manager 含 6 critical/72 high，Web 含 123 high（未做可达性分析） | pnpm/yarn audit |
| R-14 | 中 | 已证实 | WuKongIM 在 Go 1.27 编译失败，必须锁 Go 1.26.10 | build-and-test-results.md |
| R-15 | 中 | 已证实 | 构建阻断：Android 无 JDK；Server 测试需隔离 MySQL/Redis | build-and-test-results.md |
| R-16 | 中 | 已证实 | 上游 Server 单测与依赖库不同步，重复注册路由 panic，不能作回归基线 | build-and-test-results.md |
| R-17 | 高 | 已证实 | iOS Pods(AFNetworking 等)陈旧，Xcode 26.6 无法编译 | build-and-test-results.md |
| R-18 | 中 | 已证实 | govulncheck：Server 41 项命中(含 14 项标准库)，WuKongIM 2 项；未做逐项可达性复核 | build-and-test-results.md |
| R-19 | 低 | 已证实 | Android 默认包名 com.test.ts 硬编码进源码导入；换品牌包名须批量重构 | app/src/main/java/com/test/demo2/*.kt |
| R-20 | 高 | 文档证实(待源码核实) | 官方 .env 示例硬编码口令：MySQL root Aa1234567、MinIO minio/Aa1234567、WK_JWT_SECRET=wk_secret_123 | docs-review/backend-api.md |
| R-21 | 严重 | 已证实(源码) | WuKongIM 业务 API(如 POST /message/send、/user/token)无鉴权中间件，仅有 debug/维护/epoch 中间件，并全局开放 CORS(internal/access/api/server.go:372,421-423；message_send.go:54；user_legacy.go:24)。默认仅绑定 127.0.0.1(wukongim.toml.example:80)，一旦绑定到内网/公网或被反代暴露即可伪造发信；[bench] api_token 默认为空 | WuKongIM@7b810e08；运行时未做渗透验证 |
| R-22 | 中 | 文档证实 | 示例 compose 默认对宿主机发布 adminer 8306、MinIO 控制台 9001、监控 5300，宝塔页还发布 3306；与 port.md 矛盾 | backend-api.md |
| R-23 | 中 | 文档证实 | 全部示例明文 http；多处 curl|bash（gitee main）与镜像 latest，供应链不可复现 | backend-api.md |
| R-24 | 高 | 文档证实 | Android TURN 地址/用户名/密码写死在 TSApplication.kt；推送 key(华为/小米/OPPO/VIVO)明文入源码；仅覆盖国内厂商，FCM/APNs 无说明 | clients-guide.md |
| R-25 | 中 | 文档证实 | Manager 手册：管理员可看手机号/设备/聊天记录并冒用群主发消息，手册未提审计日志与角色权限矩阵；dev 默认指向第三方域名 api.botgate.cn，运行时可由 window.TSDD_CONFIG 覆盖 APP_URL | manager-price.md |
| R-26 | 低 | 文档证实 | 文档缺口：api.md/module.md/k8s.md/config-nginx.md 等 5+ 页空，iOS 页无 bundle id/URL scheme/APNs/Xcode 版本，Web 无 Electron appId/图标说明 | docs-review/* |
| R-27 | 低 | 文档证实 | 价格页：币种未写，客服(开发中)计入总价，链接锚点错误；功能矩阵以明细行为准 | manager-price.md |
| R-28 | 高 | 已证实(依赖存在)/授权条款待核实 | Manager 依赖 @fancyapps/ui@5.0.22，License 为 Custom（Fancybox 商业条款，推断），商用须购买或替换 | license-checker；node_modules/@fancyapps/ui/LICENSE.md |
| R-29 | 低 | 已证实 | 上游活跃度：Server 仅 1 个 release(v1.0.5，2023-11)；Web v1.0.0(2023-07)；Android/iOS 无 release；Server 40/Web 22/WuKongIM 101 个 open issue+PR；WuKongIM 为 v3.0.0-beta.27，仍是 beta，需锁版本 | GitHub API 2026-11-19 |
| R-30 | 中 | 已证实 | 官方 Swagger 暴露 24 份规范共约 383 个操作，含 hotline(55)、organization(46)、community、web3、tgocall 等超出价格页的模块，需求矩阵应追加这些扩展 | docs/audit/apidocs-botgate-index.md |
| R-31 | 中(用户已接受风险) | 已证实 | MySQL 8.0 已于 2026-04 EOL（最后 8.0.46），无官方安全补丁；用户决定继续使用 8.0 | ADR-0002；Oracle 8.0 release notes |
