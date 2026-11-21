# services/api — VeyroGram business API

Modular monolith, hexagonal per module (`docs/architecture/ARCHITECTURE.md` AD-1). This is the composition root and public HTTP edge.

## Layout
```
cmd/api/                  main + startup (exit 78 on invalid config)
internal/platform/        cross-cutting building blocks (config, health, httpx)
internal/<module>/        domain modules (added by later stories): domain/ app/ ports/ adapters/
```
Modules call each other only through their `ports` packages; the import-direction check lands with Story 1.2.

## Configuration (all `VG_*`, see `internal/platform/config`)
`VG_ENV` = development | test | staging | production. **Unset means production.** In staging/production the service refuses to start with: a fixed SMS code, any configured admin password,
a signing key shorter than 32 bytes or a known default, wildcard CORS, debug mode, a `root` database user, or a health listener that is not loopback/private.

| Variable | Default | Notes |
|---|---|---|
| `VG_HTTP_ADDR` | `127.0.0.1:8080` | public API listener behind Nginx |
| `VG_INTERNAL_ADDR` | `127.0.0.1:9101` | `/healthz`, `/readyz` only; never routed by Nginx |
| `VG_TOKEN_SIGNING_KEY` | — | required in staging/production |
| `VG_CORS_ALLOWED_ORIGINS` | — | comma-separated exact origins |
| `VG_MYSQL_DSN` | — | used from Story 1.3 |
| `VG_AUTH_SMS_LOGIN` | `false` | feature flag `auth.sms_login`: `/v1/auth/*` exists only when `true` (Story 1.12) |
| `VG_SMS_PROVIDER` | — | `smsbao` (短信宝, recommended) or `http` (generic JSON gateway) in staging/production; `log` prints codes and is refused outside development/test |
| `VG_SMSBAO_USER` / `VG_SMSBAO_KEY` | — | 短信宝 account and ApiKey (secret) for provider `smsbao`; `+86` numbers use the domestic API, all others the international API |
| `VG_SMS_HTTP_URL` / `VG_SMS_HTTP_TOKEN` | — | gateway for the `http` provider (https required); posts `{"to":"+E164","message":"..."}` with a bearer token |
| `VG_TOKEN_KEYS` | — | `kid:base64url(32-byte seed)[,kid2:...]`; the first key signs 15-minute access tokens (EdDSA), the others only verify (rotation). Generate: `openssl rand 32 \| base64 \| tr '+/' '-_' \| tr -d =` |
| `VG_PHONE_HASH_KEY` | — | >= 32 bytes; HMAC key for the phone blind index and code hashes. **Changing it orphans every account**: treat it like a database key, back it up, never rotate casually |
| `VG_REDIS_ADDR` / `VG_REDIS_PASSWORD` | — | Redis 7.x (loopback/private) for the access-token deny-list; required in staging/production when auth is on; without it revocation takes effect only at token expiry (<= 15 min) |
| `VG_MANAGER_ENABLED` | `false` | Turns on the admin plane (`/v1/manager/*`) on its own listener. Needs `VG_MYSQL_DSN` and `VG_ADMIN_TOTP_KEY` |
| `VG_MANAGER_ADDR` | `127.0.0.1:9103` | Admin listener; loopback/private in staging/production, published through Nginx on its own hostname (`infrastructure/nginx/veyrogram-manager.conf.template`). Must differ from the other listeners |
| `VG_MANAGER_ALLOW` | — | Optional comma-separated CIDRs/IPs allowed to reach the admin plane at all (login included); everything else gets 403 |
| `VG_MANAGER_PUBLIC_RATE` | `20` | Login / second-factor requests per minute per address on the admin plane |
| `VG_ADMIN_TOTP_KEY` | — | >= 32 random characters; encrypts administrators' TOTP secrets at rest (AES-256-GCM). Losing it means every administrator must be re-enrolled |
| `VG_IM_API_PEER_URLS` | — | Comma-separated API URLs of the **other** nodes of a multi-node WuKongIM cluster (loopback/private only). `device_quit` only closes connections owned by the node that receives it (verified on a real 3-node cluster), so revocation is sent to every node and fails (and is retried) while any node is unreachable. Leave empty for the supported single node |
| `VG_MESSAGE_RATE_LIMIT` / `VG_MESSAGE_RATE_WINDOW_SECONDS` | `30` / `10` | Per-sender send limit enforced by the send-policy gate (before_send hook and `/v1/messages`), counted in Redis; `0` disables. A Redis outage skips the rule (fail-open) and is counted in `vg_message_gate_total{rule="ratelimit",result="skipped"}` |
| `VG_IM_TOKEN_KEY` | — | >= 32 bytes, different from `VG_PHONE_HASH_KEY`; derives WuKongIM connection tokens (rotated by revocations, never stored) |
| `VG_TRUSTED_PROXIES` | — | CIDRs/IPs whose `X-Forwarded-For` is honoured (the Nginx host); empty = trust nobody (AD-16) |
| `VG_WORKER_INTERNAL_ADDR` | `127.0.0.1:9103` | The worker's own internal listener. **Must differ from `VG_MANAGER_ADDR`** when both run on one host (the production template uses `9104` for this reason) |
| `VG_INSTALL_LOCK` | — | Path the install wizard writes a human-readable "installed at" note to. Informational only: nothing reads it to decide anything; the installed state is in the database |
| `VG_ENV_FILE` | `/etc/veyrogram/api.env` | Where `install preflight -install` looks for the environment file (mode and owner are checked) |
| `VG_PROBE_CANARY_URL` | — | Optional readiness URL of a running canary; adds a `canary` target to `veyrogram-api probe` |

## Operational commands (Story 1.21)
| Command | What it does |
|---|---|
| `install preflight [-install] [-fresh] [-json] [-ports a,b]` | Checks the host and the servers and lists blockers with fixes; exit 3 if any blocks. Reads the same `VG_*` variables as the service. MySQL must be 8.0 (MariaDB is refused), Redis 7+; in staging/production Redis must require a password and use `noeviction` (the token deny-list lives there), MySQL must be in strict mode, the env file must not be world-readable |
| `install run -username U [-password-stdin] [-fresh]` | Wizard: configuration → password policy → preflight → already-installed check → migrations → first administrator. Nothing is written before the first three pass. Exit 4 when the database says the installation is initialised (deleting `install.lock` changes nothing) |
| `probe [-textfile FILE]` | api readiness (includes MySQL), worker, IM; exit 1 if any is down |

A value in the environment that still contains an install marker (`@@FILL:…@@`, `@@GEN:…@@`) is a configuration error: the service does not start on a file nobody finished filling in.

## Develop
```
make test    # go test -race (toolchain pinned to go1.26.10)
make vet fmt
make build   # bin/veyrogram-api
```
Run locally: `VG_ENV=development go run ./cmd/api`, then `curl 127.0.0.1:9101/readyz`.
