# architecture-as-is（Phase 0，2026-11-19）

标签：[FOUND_IN_SOURCE] 源码已证实；[DOCUMENTED] 仅文档；[推断] 未验证。

## 组件
- **WuKongIM**（Go，`go 1.26.0`/toolchain go1.26.10，commit 7b810e08）：长连接、消息投递与存储。端口：API 5001(默认 127.0.0.1)、TCP 5100、WS 5200、Manager 5301 [FOUND_IN_SOURCE wukongim.toml.example]。
- **TangSengDaoDaoServer**（Go，commit fc81513d）：业务（user/group/message/file/report/robot/webhook/workplace/search/statistics/openapi…），244 条静态扫描到的路由注册；依赖 TangSengDaoDaoServerLib（伪版本 9410aea0201d）。MySQL + Redis [FOUND_IN_SOURCE：测试 DSN/配置默认]；对象存储 MinIO/OSS 等 [DOCUMENTED]。
- **Manager**：Vue 3 + Element Plus + Pinia，静态站点，调用 Server 管理接口 [FOUND_IN_SOURCE package.json]。
- **Web**：React 17 monorepo + Electron 26；**Android**：Kotlin/Java 多模块(wkbase/wklogin/wkpush/wkscan/wkuikit)+MyLibs aar；**iOS**：ObjC/Swift + CocoaPods(Pods 入库)。
- 衔接：Server ↔ WuKongIM 通过 HTTP API(5001) 与 Webhook；文档对 Webhook 传输写法不一致（gRPC 6979 vs HTTP `?event=`）[DOCUMENTED 矛盾，待源码核实]。

## 信任边界与已知问题
1. 客户端 → Nginx → Server(REST)，客户端 → WuKongIM(TCP/WS)。WuKongIM API 5001 无鉴权，**必须仅被业务服务访问**（R-21）。
2. 测试/配置默认值含万能验证码与弱口令（R-02/03/20）。
3. 客户端含写死 TURN 凭据与推送 key（R-24）。
4. 上游测试基座失效（R-16），Server 无可信回归网。

## 未验证
WuKongIM↔Server 版本兼容矩阵、消息 payload 类型注册表、数据库表结构与迁移、各端 WebSocket/TCP 协议版本。→ Phase 2 前须补 `DATA-MODEL`/协议核对。
