# ADR-0007 音视频媒体层选型（待决，依赖 PRD OQ-6）
状态：Proposed｜日期：2026-11-19
| 候选 | 许可/形态 | 优点 | 风险 |
|---|---|---|---|
| LiveKit（自建，Go） | Apache-2.0 | WebRTC SFU、SDK 全端、Go 生态一致 | 需运维 TURN/多节点；版本与许可须在 Phase 4 前复核 |
| mediasoup | ISC | 成熟 SFU，Node 为主 | 引入 Node 运行时与信令自建 |
| Janus | GPL-3.0 | 插件丰富 | GPL 传染性风险 |
| 托管 RTC（云厂商） | 商用 | 省运维 | 成本与数据驻留，供应商绑定 |
| 官方 `tgocall`/RTC（闭源） | 闭源 | — | 不可复制，仅参考契约 |
TURN/STUN：coturn（BSD）自建或托管。判定准则：许可、全端 SDK 成熟度、弱网表现、成本、可自托管、与原生 systemd 部署兼容。版本与许可（尤其 LiveKit 当前版本）未在线核实 [UNVERIFIED]，决策前必须核实。
