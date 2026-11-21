# infrastructure/scripts — native deployment (no Docker)

## Install (Story 1.21): `install.sh`
```
sudo infrastructure/scripts/install.sh <development|test|staging|production>
```
Creates the service account and directories, renders `/etc/veyrogram/api.env` from `infrastructure/config/<env>.env.template` **on the host** (every `@@GEN:…@@` becomes a fresh random value that is never printed; `@@FILL:…@@` marks what only you can supply, and the service refuses to start while any remain), installs the systemd units, the journald size limits and the logrotate rule. It never overwrites an existing `api.env` and never touches Nginx. `VG_INSTALL_ROOT=/some/dir` runs it as a rehearsal without accounts or ownership changes (what `tests/ops/test_install.sh` does).
Then, with the binary: `veyrogram-api install preflight -install -fresh` (lists blockers with fixes; exit 3 if any), and `veyrogram-api install run -username <admin> -password-stdin` (configuration check → password policy → preflight → already-installed check → migrations → first administrator; refuses with exit 4 once the database says it is initialised, whatever files exist). The four environments never share secrets, database, Redis instance or (on one host) ports.

## One-time host setup (Linux, as root) — what install.sh automates
```
useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin veyrogram
install -d -o root -g veyrogram -m 0750 /etc/veyrogram
install -d -o veyrogram -g veyrogram -m 0750 /opt/veyrogram/api
install -m 0640 -o root -g veyrogram /dev/null /etc/veyrogram/api.env   # then fill VG_* values (see services/api/README.md); never commit it
cp infrastructure/systemd/veyrogram-api.service /etc/systemd/system/ && systemctl daemon-reload && systemctl enable veyrogram-api
```
Nginx (managed by 宝塔): `SERVER_NAME=… TLS_CERT=… TLS_KEY=… UPSTREAM=127.0.0.1:8080 infrastructure/scripts/render-nginx.sh infrastructure/nginx/veyrogram-api.conf.template <site.conf>`.
Only `/v1/` is forwarded; every other path, including health/metrics, returns 404.

## Release / rollback
```
make -C services/api linux                      # builds bin/veyrogram-api-linux-amd64
sha256sum services/api/bin/veyrogram-api-linux-amd64 > services/api/bin/veyrogram-api-linux-amd64.sha256
infrastructure/scripts/deploy.sh services/api/bin/veyrogram-api-linux-amd64 v2026.11.20-1   # verify checksum → preflight → switch → restart → wait ready → auto-rollback on failure
infrastructure/scripts/rollback.sh                 # back to the previous release (or: rollback.sh <version>)
```
Releases are immutable under `/opt/veyrogram/api/releases/<version>`; `current` is a symlink; the last 5 are kept.

## Gradual release (canary) and health probe
```
canary.sh start <binary> <version> <percent 1-50>   # stage + start a second instance + route that share of /v1 to it (any failure aborts, traffic unchanged)
canary.sh status | check | abort | promote           # check = server-error rate over its request count; promote re-checks, switches live, auto-rolls back
veyrogram-api probe [-textfile FILE]                 # api readiness (incl. MySQL), worker, IM; exit 1 if any is down; veyrogram-probe.timer runs it every minute
```
The canary shares the MySQL with the live release, so the schema must work with both versions at once (expand first, contract in a later release). Only `/v1` HTTP requests are split; IM connections are not.

## Rehearsal tests
`tests/ops/test_deploy.sh`, `test_install.sh`, `test_canary.sh` and `test_nginx.sh` run the scripts and a real nginx in a throwaway directory (macOS or Linux). They do **not** replace a drill on a clean Linux host with real systemd.
