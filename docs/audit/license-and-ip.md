# license-and-ip（Phase 0 初版）
- Apache-2.0：Server、Android、iOS、WuKongIM（须保留 LICENSE/NOTICE、署名、改动声明；Apache-2.0 不授予商标权，品牌必须替换）。
- MIT：Docs。
- **无许可证声明：Manager、Web** → 默认保留所有权利，二开前须取得作者书面授权或重写。LICENSE_BLOCKED，待用户/法务决策。
- 收费模块：仅作功能需求基线；未取得源码授权，一律独立实现，不拷贝/反编译。
- 第三方 SDK/字体/图标/素材清单、依赖 SBOM：**未生成（需构建环境与磁盘）**。

## 扫描结果（2026-11-19）
- 上游自身：Server/Android/iOS/WuKongIM = Apache-2.0；Docs = MIT；**Manager(`tsdd-control@1.5.0`) 与 Web(`tsdd@1.5.0`) 的 package.json 均为 UNLICENSED，且无 LICENSE 文件**（R-04 证实）。各仓库均无 NOTICE 文件。
- Manager 生产依赖(license-checker，114 项)：MIT 98、BSD-3 9、Apache-2.0 3、ISC 1、0BSD 1、(MIT AND Zlib) 1(pako)、MIT* 1(vue-grid-layout)、**Custom 1：`@fancyapps/ui@5.0.22`（Fancybox，自带 LICENSE.md，已知商业使用需付费授权 [推断，须读取其 LICENSE.md 核实]）**。
- Web 生产依赖(65 项)：MIT 58、ISC 5、Apache-2.0 1、CC-BY-4.0 1(caniuse-lite，需署名)。
- 未覆盖：devDependencies、Go 模块(508+428)、Android(389)、iOS Pods(60)、MyLibs 预编译 aar（含 rlottie、各厂商推送 SDK）、字体/图标/图片素材的许可证——清单已生成，逐项许可证归类待 Phase 1 前补完。
- 行动项：① 联系上游或重写 Manager/Web 业务代码；② @fancyapps/ui 购买授权或替换；③ 重设计阶段不保留上游图标/图片素材。

- Redis：用户决策使用 7.x。许可证因版本而异（我的了解：7.2.x 及更早为 BSD-3；7.4+ 为 RSALv2/SSPLv1；8.x 增加 AGPLv3 选项）——**未在线核实**，锁定具体补丁版本时须核对并登记；自托管内部使用一般无障碍，但不得将 Redis 作为托管服务对外售卖。

## 品牌资产第三方来源
- **Poppins Black Italic**（SIL OFL 1.1，© The Poppins Project Authors）：Logo 中 V、G 字母轮廓由其字形生成。OFL 允许在商业设计（含标志）中使用、修改轮廓；须保留版权与许可声明；不得单独出售字体。已在 `tools/build_logo.py` 与 `docs/design/brand-guidelines.md` 注明。
- Logo 候选方案中还出现过 Outfit、Sora（均为 OFL，仅用于概念对比，未入选，其字形未进入发布资产）。
- 图标库（`brand/icons/ui`）为本项目原创绘制，无第三方来源。
