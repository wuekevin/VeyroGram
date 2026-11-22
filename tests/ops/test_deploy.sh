#!/usr/bin/env bash
# Rehearsal of deploy.sh / rollback.sh in a throwaway prefix with the REAL binary and a stub "service manager".
# This does NOT replace the drill on a clean Linux host with real systemd (tracked separately in Story 1.2).
set -uo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd); scripts=$root/infrastructure/scripts
tmp=$(mktemp -d); trap 'kill_svc; rm -rf "$tmp"' EXIT
fail=0; pass=0
ok()   { pass=$((pass+1)); echo "  ok   - $1"; }
bad()  { fail=$((fail+1)); echo "  FAIL - $1"; }
check() { if eval "$2"; then ok "$1"; else bad "$1"; fi; }

port() { python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1])'; }
pub=$(port); internal=$(port)
export VG_PREFIX=$tmp/prefix VG_ENV_FILE=$tmp/api.env VG_READY_URL=http://127.0.0.1:$internal/readyz VG_READY_TIMEOUT=6
printf 'VG_ENV=development\nVG_HTTP_ADDR=127.0.0.1:%s\nVG_INTERNAL_ADDR=127.0.0.1:%s\n' "$pub" "$internal" >"$VG_ENV_FILE"

# stub service manager: stop the old process, start $VG_PREFIX/current/veyrogram-api with the env file
cat >"$tmp/svc.sh" <<SVC
#!/usr/bin/env bash
[[ -f "$tmp/pid" ]] && kill "\$(cat "$tmp/pid")" 2>/dev/null; sleep 0.3
set -a; . "$VG_ENV_FILE"; set +a
nohup "$VG_PREFIX/current/veyrogram-api" >"$tmp/svc.log" 2>&1 &
echo \$! >"$tmp/pid"
SVC
chmod +x "$tmp/svc.sh"; export VG_RESTART_CMD="$tmp/svc.sh"
kill_svc() { [[ -f "$tmp/pid" ]] && kill "$(cat "$tmp/pid")" 2>/dev/null; return 0; }
version_running() { curl -fsS --max-time 2 "$VG_READY_URL" >/dev/null 2>&1 && "$VG_PREFIX/current/veyrogram-api" -version; }
sha() { (sha256sum "$1" 2>/dev/null || shasum -a 256 "$1") | awk '{print $1}'; }

echo "building real binary v1/v2"
(cd "$root/services/api" && GOTOOLCHAIN=${GOTOOLCHAIN:-go1.26.11} go build -o "$tmp/api-v1" -ldflags "-X main.version=v1" ./cmd/api && GOTOOLCHAIN=${GOTOOLCHAIN:-go1.26.11} go build -o "$tmp/api-v2" -ldflags "-X main.version=v2" ./cmd/api) || { echo "build failed"; exit 1; }
for v in v1 v2; do echo "$(sha "$tmp/api-$v")  api-$v" >"$tmp/api-$v.sha256"; done

echo "case 1: first deploy"
"$scripts/deploy.sh" "$tmp/api-v1" v1 2>"$tmp/out1"; check "deploy v1 succeeds" '[[ $? -eq 0 || -L $VG_PREFIX/current ]]'
check "current -> v1 and service ready" '[[ "$(version_running)" == v1 ]]'

echo "case 2: checksum mismatch is refused before anything changes"
cp "$tmp/api-v2" "$tmp/api-evil"; echo "0000000000000000000000000000000000000000000000000000000000000000  x" >"$tmp/api-evil.sha256"
"$scripts/deploy.sh" "$tmp/api-evil" v-evil 2>"$tmp/out2"; rc=$?
check "mismatch exits non-zero" '[[ $rc -ne 0 ]]'
check "current still v1 and no release dir for v-evil" '[[ "$(readlink "$VG_PREFIX/current")" == releases/v1 && ! -e "$VG_PREFIX/releases/v-evil" ]]'
"$scripts/deploy.sh" "$tmp/api-v2" "../../etc" 2>/dev/null; check "path-traversal version string rejected" '[[ $? -ne 0 ]]'

echo "case 3: preflight failure (-check-config) stops before the switch"
cat >"$tmp/api-badcfg" <<'B'
#!/usr/bin/env bash
[[ "$1" == "-check-config" ]] && { echo "VG_X: invalid" >&2; exit 78; }
exit 0
B
chmod +x "$tmp/api-badcfg"; echo "$(sha "$tmp/api-badcfg")  x" >"$tmp/api-badcfg.sha256"
"$scripts/deploy.sh" "$tmp/api-badcfg" v-badcfg 2>"$tmp/out3"; rc=$?
check "preflight failure exits non-zero" '[[ $rc -ne 0 ]]'
check "current still v1, failed release removed" '[[ "$(readlink "$VG_PREFIX/current")" == releases/v1 && ! -e "$VG_PREFIX/releases/v-badcfg" ]]'

echo "case 4: new version never becomes ready -> automatic rollback"
cat >"$tmp/api-hang" <<'B'
#!/usr/bin/env bash
[[ "$1" == "-check-config" ]] && exit 0
[[ "$1" == "-version" ]] && { echo hang; exit 0; }
sleep 60
B
chmod +x "$tmp/api-hang"; echo "$(sha "$tmp/api-hang")  x" >"$tmp/api-hang.sha256"
"$scripts/deploy.sh" "$tmp/api-hang" v-hang 2>"$tmp/out4"; rc=$?
check "failed readiness exits non-zero" '[[ $rc -ne 0 ]]'
check "automatically rolled back to v1 and healthy again" '[[ "$(readlink "$VG_PREFIX/current")" == releases/v1 && "$(version_running)" == v1 ]]'
check "log says it rolled back" 'grep -q "rolled back to v1" "$tmp/out4"'

echo "case 5: good upgrade then manual rollback"
"$scripts/deploy.sh" "$tmp/api-v2" v2 2>"$tmp/out5"; check "deploy v2 succeeds and runs" '[[ "$(version_running)" == v2 ]]'
"$scripts/rollback.sh" 2>"$tmp/out6"; check "rollback.sh returns to v1" '[[ "$(version_running)" == v1 ]]'
"$scripts/rollback.sh" v1 2>/dev/null; check "rolling back to the already-current version is refused" '[[ $? -ne 0 ]]'
"$scripts/deploy.sh" "$tmp/api-v2" v2 2>/dev/null; check "release versions are immutable (redeploy v2 refused)" '[[ $? -ne 0 ]]'

echo; echo "passed=$pass failed=$fail"; [[ $fail -eq 0 ]]
