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

## Develop
```
make test    # go test -race (toolchain pinned to go1.26.10)
make vet fmt
make build   # bin/veyrogram-api
```
Run locally: `VG_ENV=development go run ./cmd/api`, then `curl 127.0.0.1:9101/readyz`.
