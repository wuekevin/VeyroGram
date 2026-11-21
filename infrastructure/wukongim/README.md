# WuKongIM production hardening (Story 1.7, AD-2, R-21)

WuKongIM's API (5001), Manager (5301), cluster RPC (7001), Prometheus (9099), MQTT (1883) and pprof/debug surfaces have no
or weak authentication. They must never be reachable from the public network or from non-allowlisted hosts. Only the two gateway
listeners (client TCP/WS) are client-facing, and WS should be reached through Nginx.

## Render, validate, scan
```
export WK_DATA_DIR=/var/lib/veyrogram/wukongim WK_CLUSTER_ADDR=127.0.0.1:7001 WK_API_ADDR=127.0.0.1:5001 WK_MANAGER_ADDR=127.0.0.1:5301 \
       WK_GATEWAY_TCP=0.0.0.0:5100 WK_GATEWAY_WS=127.0.0.1:5200 WK_EXTERNAL_TCP=im.example.com:5100 WK_EXTERNAL_WS=wss://im.example.com/ws \
       WK_BEFORE_SEND_URL=http://127.0.0.1:9102/im/webhook/<path-secret>   # the IM appends ?event=msg.before_send; <path-secret> = VG_IM_WEBHOOK_SECRET   # + secrets from a root-only env file, never from the shell history
python3 -I tools/render_wukongim.py infrastructure/wukongim/wukongim.prod.toml.template /etc/veyrogram/wukongim.toml   # written 0600
python3 -I tools/wukongim_config_check.py /etc/veyrogram/wukongim.toml     # deploy gate: defaults/secrets/plugin/MQTT/bind addresses
python3 -I tools/exposure_scan.py --target <public-ip> --target <private-ip>   # run from OUTSIDE and from a same-subnet non-allowlisted host
```
The binary must be the patched build (`tools/build_wukongim.sh`), because `webhook.signing_secret` does not exist upstream; the same value goes into `VG_IM_WEBHOOK_SIGNING_SECRET` for the API.
Secrets (`WK_WEBHOOK_SIGNING_SECRET`, `WK_JOIN_TOKEN`, `WK_MANAGER_JWT_SECRET`, `WK_MANAGER_PASSWORD`, `WK_BENCH_TOKEN`) must be >= 32 chars (password >= 16) and generated fresh per environment.

## What is and is not proven
- Proven by tests (`tests/ops/test_exposure.py`): the checker rejects every upstream default and each mutation; the scanner flags a wildcard-bound
  listener and ignores loopback-only ones; a real WuKongIM started from the rendered template is unreachable via the host's non-loopback address,
  and a deliberately public API bind is caught by both the checker and the live scan.
- NOT proven here: reachability from the real Internet or from another host on the production subnet, firewall/security-group rules, and the
  gateway-port policy. The scan must be run from those vantage points at pre-production; a host firewall (default-deny inbound) remains a required second layer.
- The checker parses the single-line TOML subset used by the upstream example, not full TOML; the real-binary test guards against template drift.
