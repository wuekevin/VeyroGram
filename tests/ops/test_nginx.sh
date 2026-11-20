#!/usr/bin/env bash
# Real nginx: render the template, `nginx -t`, then prove the whitelist: /v1/ is forwarded (client XFF overwritten),
# /healthz /readyz /metrics and every other path are 404 and never reach the upstream.
set -uo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
command -v nginx >/dev/null || { echo "SKIP: nginx not installed"; exit 0; }
tmp=$(mktemp -d); pids=()
trap 'for p in "${pids[@]}"; do kill "$p" 2>/dev/null; done; [[ -f $tmp/nginx.pid ]] && kill "$(cat "$tmp/nginx.pid")" 2>/dev/null; rm -rf "$tmp"' EXIT
fail=0; pass=0
check() { if eval "$2"; then pass=$((pass+1)); echo "  ok   - $1"; else fail=$((fail+1)); echo "  FAIL - $1"; fi; }
port() { python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1])'; }
up=$(port); ng=$(port)

# upstream echo server that records every path it receives and echoes the X-Forwarded-For it saw
cat >"$tmp/up.py" <<PY
import http.server, sys
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        open("$tmp/upstream.log","a").write(self.path+"\n")
        self.send_response(200); self.send_header("Content-Type","text/plain"); self.end_headers()
        self.wfile.write(("xff="+(self.headers.get("X-Forwarded-For") or "")+" rid="+(self.headers.get("X-Request-Id") or "")).encode())
    def log_message(self,*a): pass
http.server.HTTPServer(("127.0.0.1",$up),H).serve_forever()
PY
python3 "$tmp/up.py" & pids+=($!)
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$tmp/k.pem" -out "$tmp/c.pem" -days 1 -subj "/CN=api.test" >/dev/null 2>&1
SERVER_NAME=api.test TLS_CERT=$tmp/c.pem TLS_KEY=$tmp/k.pem UPSTREAM=127.0.0.1:$up LISTEN=127.0.0.1:$ng \
  "$root/infrastructure/scripts/render-nginx.sh" "$root/infrastructure/nginx/veyrogram-api.conf.template" "$tmp/site.conf"
mkdir -p "$tmp/logs"
cat >"$tmp/nginx.conf" <<CONF
pid $tmp/nginx.pid; error_log $tmp/logs/error.log; daemon on;
events {}
http { access_log off; client_body_temp_path $tmp/b; proxy_temp_path $tmp/p; fastcgi_temp_path $tmp/f; uwsgi_temp_path $tmp/u; scgi_temp_path $tmp/s;
  include $tmp/site.conf; }
CONF
check "nginx -t accepts the rendered template" 'nginx -t -p "$tmp" -c "$tmp/nginx.conf" >"$tmp/t.out" 2>&1'
nginx -p "$tmp" -c "$tmp/nginx.conf" 2>"$tmp/start.err"; sleep 1
c() { curl -ks --max-time 3 "$@"; }
check "whitelisted /v1/ping is forwarded" '[[ "$(c -o /dev/null -w %{http_code} https://127.0.0.1:'$ng'/v1/ping)" == 200 ]]'
check "client-supplied X-Forwarded-For is overwritten, not appended" '[[ "$(c -H "X-Forwarded-For: 6.6.6.6" https://127.0.0.1:'$ng'/v1/ping)" == "xff=127.0.0.1 rid="* ]]'
check "a request id is added for tracing" '[[ "$(c https://127.0.0.1:'$ng'/v1/ping)" =~ rid=[0-9a-f]{32} ]]'
: >"$tmp/upstream.log"
for p in /healthz /readyz /metrics /debug/pprof/ / /admin/ /open/v1/x /message/send /user/token; do
  code=$(c -o /dev/null -w '%{http_code}' "https://127.0.0.1:$ng$p")
  check "$p is not served (404)" '[[ "$code" == 404 ]]'
done
code=$(c -o /dev/null -w '%{http_code}' "https://127.0.0.1:$ng/v1")
check "/v1 without slash is only redirected to /v1/ (301) or 404, never 200" '[[ "$code" == 301 || "$code" == 404 ]]'
check "none of those requests reached the upstream" '[[ ! -s "$tmp/upstream.log" ]]'
: >"$tmp/upstream.log"
c --path-as-is -o /dev/null "https://127.0.0.1:$ng/v1/../healthz"; c --path-as-is -o /dev/null "https://127.0.0.1:$ng/v1/%2e%2e/healthz"
check "path-normalisation tricks never deliver /healthz to the upstream as a clean path" '! grep -qx "/healthz" "$tmp/upstream.log"'
check "TLS 1.1 is refused" '! curl -ks --max-time 3 --tls-max 1.1 -o /dev/null https://127.0.0.1:'$ng'/v1/ping'
echo; echo "passed=$pass failed=$fail"; [[ $fail -eq 0 ]]
