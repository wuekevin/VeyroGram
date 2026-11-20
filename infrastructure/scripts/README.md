# infrastructure/scripts — native deployment (no Docker)

## One-time host setup (Linux, as root)
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

## Rehearsal tests
`tests/ops/test_deploy.sh` and `tests/ops/test_nginx.sh` run the scripts and a real nginx in a throwaway directory (macOS or Linux). They do **not** replace a drill on a clean Linux host with real systemd.
