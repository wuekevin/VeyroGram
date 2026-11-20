# ADR-0009 发送路径执行点
状态：Accepted（v1）｜日期：2026-11-19
证据：WuKongIM CHANGELOG 与 wukongim.toml.example `[webhook.before_send]` 提供可选同步 `msg.before_send`（可改写/拒绝，拒绝码 128–255）。客户端经 IM 网关直发，业务 HTTP 层看不到发送。
选项：A) before_send 同步 webhook + Redis 投影（选）；B) 全部发送改走业务 API（分叉上游协议，成本高）；C) 仅事后审计（无法阻止）。
后果：发送延迟预算与 fail-closed 策略需压测；投影滞后需监控；状态变更经 outbox 更新投影。
