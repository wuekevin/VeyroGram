# 测试与安全门禁（Story 1.24）

所有门禁都在 CI（`.github/workflows/ci.yml`）和本地（`bash tools/ci_local.sh`）同时运行。**要让它们真正阻断合并，须在 GitHub 分支保护里把 `ci` 的各作业设为必需检查**（见 MANUAL-CHECKLIST B 节；这个设置不在仓库文件里）。

| 门禁 | 做什么 | 怎么被阻断 / 怎么放行 | 工具 |
|---|---|---|---|
| SBOM | 为“构建实际链接的 Go 依赖（含测试依赖）”生成 CycloneDX 1.5，带 go.sum 哈希；CI 作为构件保留 90 天 | 不阻断，是证据 | `tools/supplychain.py sbom` |
| 漏洞扫描（SCA） | `govulncheck`：依赖 + Go 标准库，按可达性判断 | 代码**会调用**的漏洞阻断；仅被依赖但不可达的只提示 | CI `supply-chain` 作业 |
| 许可证 | 读每个依赖的许可证文本分类；白名单：MIT、BSD-2/3、Apache-2.0、ISC、MPL-2.0、Unlicense、0BSD | 无许可证（UNLICENSED）、无法识别、自定义/商业条款（Commons Clause、BUSL、SSPL、Elastic、非商用）、GPL/AGPL/LGPL 一律阻断。放行只能在 `tools/supplychain-waivers.yaml` 写：模块、理由、批准人、到期日；到期后自动失效 | `tools/supplychain.py licenses` |
| 官方源 | 禁止清单/锁文件/CI 出现第三方镜像（npmmirror、taobao、goproxy.cn 等） | 阻断 | `tools/supplychain.py mirrors` |
| 密钥扫描 | 扫所有受版本控制的文件：私钥块、云/VCS/聊天令牌、JWT、DSN 内口令、高熵“secret/password/token/key”赋值 | 阻断。放行只能在 `tools/secret-scan-allow.yaml` 写：文件、规则、该值的指纹、理由；一个条目只放行**一个文件里的一个值**，同文件后来新增的值仍会被抓 | `tools/secret_scan.py` |
| OpenAPI diff | 与基线分支比较，破坏性变更须升主版本或写 ADR | 阻断 | `tools/openapi_diff.py`（Story 1.2） |
| 上游兼容 | 新契约路径不得与上游旧路径相同；上游每张表有书面处置 | 阻断 | `tools/check_upstream_compat.py`（Story 1.23） |
| 路由必须鉴权 | 从源码发现全部路由；未包 `Auth` 且不在 `publicRoutes` 的路由失败；代码与 OpenAPI 双向一致，且“需要 bearer”与代码的鉴权一致 | 阻断 | `cmd/api/authzgate_test.go` |
| 生成式越权负面 | 对**每个**需鉴权路由自动发：无头、错误方案、空、乱码、他钥签名、过期、篡改的令牌；再测跨租户、非属主、被拉黑、登出后重放 | 阻断 | 同上（需 MySQL、Redis、WuKongIM） |

## 门禁的门禁（防止它们悄悄失效）
- `tests/ops/test_supplychain.py`：许可证分类（GPL、无许可证、商业条款必须被挡，MPL 里提到 GPL 的措辞不能误杀）、豁免过期与缺批准人、各类密钥必须被抓、允许清单只放行一个值。
- `tests/ops/test_upstream_compat.py`：旧路径混入、未处置的表、删除空清单声明，都必须失败。
- 路由门禁的变异测试（手工做过）：删掉某路由的 `Auth` 包装、新增未入契约的路由，都会失败。

## 核心 E2E 套件（自建，不以上游测试为基线）
上游测试不是基线（R-16）。核心套件就是 `services/api/cmd/api` 下用**真实 MySQL + Redis + 打补丁的 WuKongIM + API + worker 进程**跑的端到端测试：注册/登录/刷新（`TestAuthRegisterLoginRefreshOverRealHTTP`）、设备撤销与踢线（`TestRevocationKickReconnectAndNoticeOverRealWebSocket`）、单聊与发送门（`TestSingleChatOverRealIMWithBeforeSendGate`）、拉黑（`TestBlockListThroughRealIMAndAPI`）、离线同步（`TestSyncCatchUpUnreadDraftsAndIsolationOverRealIM`）、生成式越权（上表）、备份恢复与 IM 快照恢复、安装向导、管理员平面。`bash tools/ci_local.sh` 在设置了 `VG_TEST_*` 环境变量时全部运行；没设时这些测试**跳过并在输出里打 NOTE**，所以“本地绿”不等于“跑过了集成测试”。

## 仍然没做（诚实说明）
- **只有 Go 依赖被扫描。** 仓库里还没有 npm/Gradle/Pods 清单；这些端开工时必须把对应的 SBOM/许可证/镜像检查接进来（镜像检查已按文件名覆盖了 package.json、lockfile、Gradle、Podfile）。
- 许可证识别是对许可证**文本**的特征匹配，不是 SPDX 权威数据库；遇到双许可证、非常规措辞会判“无法识别”而阻断，需要人工看一眼并按豁免流程处理。
- 密钥扫描是正则 + 熵，不是 gitleaks 那样的完整规则库，也**不扫 git 历史**（只扫当前文件）。若曾经提交过密钥，要单独处理并轮换。
- “已移除成员”的负面测试**现在没有可测对象**（还没有群路由）；路由门禁会在出现群/频道路由时失败，逼着补上这个测试，而不是默默漏掉。
- 重放类：刷新令牌重放（`TestRefreshReuseAlsoKillsAccessTokensAndRotatesIM`）、短信验证码一次性、登出后令牌重放（上面的生成式测试）已覆盖；`Idempotency-Key` 重放在 Story 1.1 的中间件测试里，这里没有再生成一遍。
- 消息类型 schema 的契约测试要等 Epic 2 的内容类型注册表（Story 2.1）才有对象。
