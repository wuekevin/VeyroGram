#!/usr/bin/env bash
# Rehearsal of canary.sh with a REAL nginx in front and stub service processes behind it: the traffic split, the health gate, abort, promote, and that every failure
# path leaves traffic exactly where it was. The stubs stand in for the API binary (the real binary is covered by test_deploy.sh); this does NOT replace the
# drill on a clean Linux host with real systemd.
set -uo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd); scripts=$root/infrastructure/scripts
command -v nginx >/dev/null || { echo "SKIP: nginx not installed"; exit 0; }
tmp=$(mktemp -d); pids=()
cleanup() { for f in "$tmp"/pid.*; do [[ -f $f ]] && kill "$(cat "$f")" 2>/dev/null; done; [[ -f $tmp/nginx.pid ]] && kill "$(cat "$tmp/nginx.pid")" 2>/dev/null; rm -rf "$tmp"; }
trap cleanup EXIT
fail=0; pass=0
ok()   { pass=$((pass+1)); echo "  ok   - $1"; }
bad()  { fail=$((fail+1)); echo "  FAIL - $1"; }
check() { if eval "$2"; then ok "$1"; else bad "$1"; fi; }
port() { python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1])'; }
sha() { (sha256sum "$1" 2>/dev/null || shasum -a 256 "$1") | awk '{print $1}'; }
live=$(port); liveint=$(port); can=$(port); canint=$(port); ng=$(port)

# stub API: /v1/ping answers its version label; the internal listener answers /readyz and /metrics (counters read from a file the test controls)
cat >"$tmp/stub.py" <<PY
import http.server, sys, threading
label, pub, internal = sys.argv[1], int(sys.argv[2]), int(sys.argv[3])
class Pub(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200); self.end_headers(); self.wfile.write(label.encode())
    def log_message(self,*a): pass
class Int(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/readyz": body = b"ok"
        elif self.path == "/metrics":
            try: body = open("$tmp/metrics."+label).read().encode()
            except Exception: body = b""
        else: self.send_response(404); self.end_headers(); return
        self.send_response(200); self.end_headers(); self.wfile.write(body)
    def log_message(self,*a): pass
threading.Thread(target=http.server.ThreadingHTTPServer(("127.0.0.1",pub),Pub).serve_forever,daemon=True).start()
http.server.ThreadingHTTPServer(("127.0.0.1",internal),Int).serve_forever()
PY
mkbin() { # <label> <mode: good|hang|badcfg>
  local b=$tmp/api-$1
  cat >"$b" <<B
#!/usr/bin/env bash
[[ "\$1" == "-check-config" ]] && { [[ "$2" == badcfg ]] && { echo "VG_X: invalid" >&2; exit 78; }; exit 0; }
[[ "\$1" == "-version" ]] && { echo "$1"; exit 0; }
[[ "$2" == hang ]] && sleep 60
exec python3 "$tmp/stub.py" "$1" "\${VG_HTTP_ADDR##*:}" "\${VG_INTERNAL_ADDR##*:}"
B
  chmod +x "$b"; echo "$(sha "$b")  api-$1" >"$b.sha256"
}
metrics() { printf '%s\n' "vg_http_requests_total{method=\"GET\",class=\"2xx\"} $2" "vg_http_requests_total{method=\"GET\",class=\"5xx\"} $3" >"$tmp/metrics.$1"; }

export VG_PREFIX=$tmp/prefix VG_ENV_FILE=$tmp/api.env VG_READY_URL=http://127.0.0.1:$liveint/readyz VG_READY_TIMEOUT=6
export VG_CANARY_READY_URL=http://127.0.0.1:$canint/readyz VG_CANARY_METRICS_URL=http://127.0.0.1:$canint/metrics VG_CANARY_UPSTREAM=127.0.0.1:$can
export VG_CANARY_NGINX_FILE=$tmp/nginx/canary-api.conf
printf 'VG_ENV=development\nVG_HTTP_ADDR=127.0.0.1:%s\nVG_INTERNAL_ADDR=127.0.0.1:%s\n' "$live" "$liveint" >"$VG_ENV_FILE"
printf 'VG_HTTP_ADDR=127.0.0.1:%s\nVG_INTERNAL_ADDR=127.0.0.1:%s\n' "$can" "$canint" >"$tmp/canary.env"
runsvc() { # <name> <release link> <extra env file or ''>
  [[ -f "$tmp/pid.$1" ]] && kill "$(cat "$tmp/pid.$1")" 2>/dev/null; sleep 0.3
  ( set -a; . "$VG_ENV_FILE"; [[ -n $3 ]] && . "$3"; set +a; nohup "$VG_PREFIX/$2/veyrogram-api" >"$tmp/$1.log" 2>&1 & echo $! >"$tmp/pid.$1" )
}
cat >"$tmp/live.sh" <<S
#!/usr/bin/env bash
$(declare -f runsvc); tmp=$tmp; VG_PREFIX=$VG_PREFIX; VG_ENV_FILE=$VG_ENV_FILE; runsvc live current ""
S
cat >"$tmp/canary-start.sh" <<S
#!/usr/bin/env bash
$(declare -f runsvc); tmp=$tmp; VG_PREFIX=$VG_PREFIX; VG_ENV_FILE=$VG_ENV_FILE; runsvc canary canary $tmp/canary.env
S
cat >"$tmp/canary-stop.sh" <<S
#!/usr/bin/env bash
[[ -f "$tmp/pid.canary" ]] && { kill "\$(cat "$tmp/pid.canary")" 2>/dev/null; rm -f "$tmp/pid.canary"; }; exit 0
S
chmod +x "$tmp"/*.sh; export VG_RESTART_CMD=$tmp/live.sh VG_CANARY_START_CMD=$tmp/canary-start.sh VG_CANARY_STOP_CMD=$tmp/canary-stop.sh

# real nginx from the real template
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$tmp/k.pem" -out "$tmp/c.pem" -days 1 -subj "/CN=api.test" >/dev/null 2>&1
mkdir -p "$tmp/nginx" "$tmp/logs"
SERVER_NAME=api.test TLS_CERT=$tmp/c.pem TLS_KEY=$tmp/k.pem UPSTREAM=127.0.0.1:$live LISTEN=127.0.0.1:$ng CANARY_INCLUDE="$tmp/nginx/canary-*.conf" \
  "$scripts/render-nginx.sh" "$root/infrastructure/nginx/veyrogram-api.conf.template" "$tmp/site.conf"
cat >"$tmp/nginx.conf" <<CONF
pid $tmp/nginx.pid; error_log $tmp/logs/error.log; daemon on;
events {}
http { access_log off; client_body_temp_path $tmp/b; proxy_temp_path $tmp/p; fastcgi_temp_path $tmp/f; uwsgi_temp_path $tmp/u; scgi_temp_path $tmp/s;
  include $tmp/site.conf; }
CONF
export VG_NGINX_TEST_CMD="nginx -t -p $tmp -c $tmp/nginx.conf" VG_NGINX_RELOAD_CMD="nginx -p $tmp -c $tmp/nginx.conf -s reload"
check "nginx accepts the template with NO canary file present (empty wildcard include)" 'nginx -t -p "$tmp" -c "$tmp/nginx.conf" >/dev/null 2>&1'
nginx -p "$tmp" -c "$tmp/nginx.conf" 2>/dev/null; sleep 1

# N requests through nginx -> "v1=.. v2=.."
spread() { local n=$1 i out=""; for ((i=0;i<n;i++)); do out+="$(curl -ks --max-time 3 https://127.0.0.1:$ng/v1/ping) "; done; echo "$out" | tr ' ' '\n' | grep -v '^$' | sort | uniq -c | awk '{printf "%s=%s ", $2, $1}'; }
share() { local s; s=$(spread "$1"); local a b; a=$(grep -o "$2=[0-9]*" <<<"$s" | cut -d= -f2); echo "${a:-0}"; }

echo "building stub releases"
mkbin v1 good; mkbin v2 good; mkbin vhang hang; mkbin vbad badcfg; metrics v2 0 0
"$scripts/deploy.sh" "$tmp/api-v1" v1 2>"$tmp/d1.err"
check "live release v1 is deployed and serves everything" '[[ "$(spread 20)" == "v1=20 " ]]'

echo "case 1: argument and state checks"
for p in 0 51 abc; do "$scripts/canary.sh" start "$tmp/api-v2" v2 "$p" 2>/dev/null; check "percent '$p' is refused" '[[ $? -ne 0 ]]'; done
check "refusals leave no state and no nginx file" '[[ ! -e $VG_PREFIX/.canary && ! -e $VG_CANARY_NGINX_FILE && ! -e $VG_PREFIX/releases/v2 ]]'
"$scripts/canary.sh" abort 2>/dev/null; check "abort with no canary is refused" '[[ $? -ne 0 ]]'

echo "case 2: start a 20% canary"
"$scripts/canary.sh" start "$tmp/api-v2" v2 20 2>"$tmp/c2.err"; rc=$?
check "start succeeds" '[[ $rc -eq 0 ]]'
n=$(share 400 v2)
check "about 20% of 400 requests reach the canary (got $n)" '(( n >= 72 && n <= 88 ))'
check "the live instance still answers the rest" '[[ $(share 100 v1) -ge 70 ]]'
"$scripts/canary.sh" start "$tmp/api-v2" v3 10 2>/dev/null; check "a second canary cannot start while one runs" '[[ $? -ne 0 ]]'
st=$("$scripts/canary.sh" status 2>&1); check "status names the canary and its share" 'grep -q "canary: v2 at ~20%" <<<"$st" && grep -q "canary instance: ready" <<<"$st"'

echo "case 3: the health gate"
metrics v2 10 0;  "$scripts/canary.sh" check 2>"$tmp/k1.err"; check "too few requests -> not healthy yet" '[[ $? -ne 0 ]] && grep -q "keep watching" "$tmp/k1.err"'
metrics v2 190 10; "$scripts/canary.sh" check 2>"$tmp/k2.err"; check "5% server errors -> refused" '[[ $? -ne 0 ]] && grep -q "do not promote" "$tmp/k2.err"'
"$scripts/canary.sh" promote 2>/dev/null; check "promote refuses an unhealthy canary" '[[ $? -ne 0 ]]'
check "...and nothing changed: still v1 live, canary still routed" '[[ "$(readlink "$VG_PREFIX/current")" == releases/v1 && -f $VG_CANARY_NGINX_FILE ]]'
metrics v2 600 0; check "no errors over 600 requests -> healthy" '"$scripts/canary.sh" check 2>/dev/null'

echo "case 4: abort"
"$scripts/canary.sh" abort 2>/dev/null; check "abort succeeds" '[[ $? -eq 0 ]]'
check "all traffic is back on v1" '[[ "$(spread 60)" == "v1=60 " ]]'
check "canary file, state and instance are gone" '[[ ! -e $VG_CANARY_NGINX_FILE && ! -e $VG_PREFIX/.canary && ! -e $tmp/pid.canary ]]'
check "live release was never touched" '[[ "$(readlink "$VG_PREFIX/current")" == releases/v1 ]]'

echo "case 5: failure paths leave traffic untouched"
rm -rf "$VG_PREFIX/releases/v2"
"$scripts/canary.sh" start "$tmp/api-vbad" vbad 10 2>"$tmp/e1.err"; check "failing config preflight refuses the canary" '[[ $? -ne 0 && ! -e $VG_PREFIX/.canary && ! -e $VG_PREFIX/releases/vbad ]]'
"$scripts/canary.sh" start "$tmp/api-vhang" vhang 10 2>"$tmp/e2.err"; check "a canary that never becomes ready is aborted" '[[ $? -ne 0 ]] && grep -q "not ready" "$tmp/e2.err"'
check "...traffic is unchanged and nothing is left behind" '[[ "$(spread 40)" == "v1=40 " && ! -e $VG_CANARY_NGINX_FILE && ! -e $VG_PREFIX/.canary ]]'
rm -rf "$VG_PREFIX/releases/vhang"; mkbin v2 good
VG_NGINX_TEST_CMD=false "$scripts/canary.sh" start "$tmp/api-v2" v2 10 2>"$tmp/e3.err"; check "nginx rejecting the config aborts the canary" '[[ $? -ne 0 ]] && grep -q "nginx rejected" "$tmp/e3.err"'
check "...traffic unchanged, no file, instance stopped" '[[ "$(spread 40)" == "v1=40 " && ! -e $VG_CANARY_NGINX_FILE && ! -e $tmp/pid.canary ]]'
cp "$tmp/api-v2" "$tmp/api-evil"; echo "0000000000000000000000000000000000000000000000000000000000000000  x" >"$tmp/api-evil.sha256"
"$scripts/canary.sh" start "$tmp/api-evil" vevil 10 2>/dev/null; check "a checksum mismatch is refused before anything starts" '[[ $? -ne 0 && ! -e $VG_PREFIX/releases/vevil ]]'

echo "case 6: promote"
rm -rf "$VG_PREFIX/releases/v2"
"$scripts/canary.sh" start "$tmp/api-v2" v2 10 2>/dev/null; check "start 10% canary" '[[ $? -eq 0 ]]'
metrics v2 500 2   # 0.4% errors: under the 1% limit
"$scripts/canary.sh" promote 2>"$tmp/p.err"; rc=$?
check "promote succeeds" '[[ $rc -eq 0 ]]'
check "v2 is now the live release and everything answers v2" '[[ "$(readlink "$VG_PREFIX/current")" == releases/v2 && "$(spread 40)" == "v2=40 " ]]'
check "the canary is gone (file, state, link, instance)" '[[ ! -e $VG_CANARY_NGINX_FILE && ! -e $VG_PREFIX/.canary && ! -e $VG_PREFIX/canary && ! -e $tmp/pid.canary ]]'
"$scripts/rollback.sh" 2>/dev/null; check "rollback.sh returns to v1 after a promote" '[[ "$(spread 20)" == "v1=20 " ]]'

echo; echo "canary rehearsal: $pass passed, $fail failed"; exit $fail
