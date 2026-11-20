# ADR-0001 工具链锁定与 WuKongIM API 暴露策略（草案，待评审）
状态：Proposed｜日期：2026-11-19
## 决策
1. Go 工具链：WuKongIM 固定 go1.26.10（go1.27 编译失败，build-and-test-results.md）；Server 固定到一个经 CI 验证的版本，`GOTOOLCHAIN` 在 CI/发布脚本显式声明。
2. Node：Manager 用 pnpm 9 冻结安装；Web 用 yarn 1.22.x；禁止 lockfile 使用第三方镜像，重新生成并校验 integrity（R-12）。
3. WuKongIM 5001 仅监听回环/私网，由 Go 业务服务独占调用；Nginx 不得转发 5001；新增服务只能经业务服务间接发信。若需网络可达，前置带鉴权的内部网关（mTLS 或签名令牌）。
4. 官方 compose 仅作拓扑参考；原生 systemd 部署（总指令 §9）。
## 后果
增加网关/适配层工作量，但消除无鉴权发信面。须在 Phase 2 用负面测试证明 5001 外部不可达。
