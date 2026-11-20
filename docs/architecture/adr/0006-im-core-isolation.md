# ADR-0006 WuKongIM 隔离与适配
状态：Accepted（延伸 audit/adr/0001）｜日期：2026-11-19
证据：源码 WuKongIM@7b810e08 中 `/message/send`、`/user/token` 注册于 `internal/access/api`，仅挂 debug/维护/epoch 中间件与开放 CORS（server.go:372,421-423）；默认监听 127.0.0.1（wukongim.toml.example:80）。
决策：见 AD-2。所有对 IM API 的调用经 `imgw` 适配器；WuKongIM 固定 tag 并用 go1.26.10 构建；适配层有对固定版本的契约测试；`wukongim.toml` 的 manager 默认账号/jwt_secret、`[bench] api_token` 必须在生产覆盖。
