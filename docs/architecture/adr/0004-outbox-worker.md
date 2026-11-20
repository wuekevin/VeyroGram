# ADR-0004 DB Outbox + Worker，暂不引入消息中间件
状态：Accepted｜日期：2026-11-19
决策：事务内写 `outbox`，worker 用 `FOR UPDATE SKIP LOCKED` 领取，至少一次投递，消费者幂等，失败退避后入 `dead_letter`（AD-7）。
触发重评：压测显示 outbox 吞吐/延迟不达标，或需要跨主机扇出。
