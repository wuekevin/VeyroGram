#!/usr/bin/env bash
# Reproducible build of the Web/Desktop and Manager clients (Story 1.10): pinned upstream SHA + VeyroGram patch queue, frozen install from the
# official registries, audit gate, build, and a content hash of the output. The upstream checkout is never modified (throwaway worktree).
# Usage: tools/build_clients.sh <web|manager> <workdir>    (workdir must not exist; the patched tree + build output are left there)
# Env: VG_RENDER_ELECTRON=<electron binary> renders the build in a hidden Electron window; VG_SKIP_AUDIT=1 skips the network audit step; VG_ELECTRON_SMOKE=1 downloads Electron and runs the runtime security smoke test (web only).
set -euo pipefail
which=${1:?usage: build_clients.sh <web|manager> <workdir>}
work=${2:?workdir}
root=$(cd "$(dirname "$0")/.." && pwd)
case "$which" in
  web) up="$root/upstream/TangSengDaoDaoWeb";;
  manager) up="$root/upstream/TangSengDaoDaoManager";;
  *) echo "unknown client: $which" >&2; exit 2;;
esac
case "$work" in /*) ;; *) work="$PWD/$work";; esac
[ ! -e "$work" ] || { echo "$work already exists" >&2; exit 2; }
pin=$(tr -d '[:space:]' < "$root/apps/$which/PIN")
head=$(git -C "$up" rev-parse HEAD)
git -C "$up" cat-file -e "$pin^{commit}" 2>/dev/null || { echo "upstream $which does not contain pinned commit $pin" >&2; exit 1; }
# Run a GUI program with a hard time limit, output in a file (Chromium helpers can outlive the app and hold a pipe open; a hung window must fail, not stall).
bounded() {  # bounded <seconds> <logfile> cmd...
  local secs=$1 log=$2; shift 2
  "$@" > "$log" 2>&1 & local pid=$!
  ( sleep "$secs"; kill -9 "$pid" 2>/dev/null ) & local killer=$!
  local rc=0; wait "$pid" || rc=$?
  kill "$killer" 2>/dev/null || true; wait "$killer" 2>/dev/null || true
  return $rc
}
cleanup() { git -C "$up" worktree remove --force "$work" >/dev/null 2>&1 || true; }
git -C "$up" worktree add --detach -q "$work" "$pin"
n=0
for p in "$root/apps/$which"/patches/*.patch; do
  git -C "$work" apply --index "$p" || { echo "patch $(basename "$p") does not apply to $pin" >&2; cleanup; exit 1; }
  n=$((n+1))
done
echo "patched $which at ${pin:0:8} with $n patch(es)"
python3 -I "$root/tools/check_client_locks.py" "$work" || exit 1
export npm_config_registry=https://registry.npmjs.org/ CI=false
cd "$work"
if [ "$which" = web ]; then
  yarn="npx --yes yarn@1.22.22"
  $yarn install --frozen-lockfile --ignore-scripts --ignore-engines --silent >/dev/null 2>&1
  [ "${VG_SKIP_AUDIT:-0}" = 1 ] || python3 -I "$root/tools/audit_clients.py" yarn "$work"
  python3 -I "$root/tools/npm_supply.py" licenses "$work"
  python3 -I "$root/tools/npm_supply.py" sbom "$work" -o "$work/web.cdx.json"
  (cd apps/web && $yarn build >/dev/null && $yarn compile:electron >/dev/null)
  python3 -I "$root/tools/check_electron_security.py" "$work/apps/web"
  out="$work/apps/web/build"
  if [ "${VG_ELECTRON_SMOKE:-0}" = 1 ]; then
    node node_modules/electron/install.js
    (cd apps/web && bounded 120 "$work/smoke.log" ../../node_modules/.bin/electron ${VG_ELECTRON_FLAGS:-} src-election/test/smoke.js) \
      || { echo "Electron smoke failed or timed out" >&2; grep -v 'ERROR:' "$work/smoke.log" | tail -40 >&2; exit 1; }
    grep -E '^(PASS|FAIL|all |[0-9]+ FAILED)' "$work/smoke.log"
  fi
else
  pnpm install --frozen-lockfile --ignore-scripts >/dev/null 2>&1
  [ "${VG_SKIP_AUDIT:-0}" = 1 ] || python3 -I "$root/tools/audit_clients.py" pnpm "$work"
  python3 -I "$root/tools/npm_supply.py" licenses "$work"
  python3 -I "$root/tools/npm_supply.py" sbom "$work" -o "$work/manager.cdx.json"
  pnpm build >/dev/null
  out="$work/dist"
fi
if [ -n "${VG_RENDER_ELECTRON:-}" ]; then  # render smoke: load the build in a hidden Electron, fail on app errors / empty page, keep a screenshot
  sel='#root'; [ "$which" = manager ] && sel='#app'
  # Output goes to a file, not a pipe: Chromium helper processes can outlive the app and hold a pipe open, which would hang `| grep` forever.
  rc=0; bounded 240 "$work/render-$which.log" "$VG_RENDER_ELECTRON" ${VG_ELECTRON_FLAGS:-} "$root/tools/render_smoke.js" "$out" "$work/render-$which.png" "$sel" || rc=$?
  grep -E '^(\{|ERROR|RENDER|NOTE|STEP)' "$work/render-$which.log" || true
  [ "$rc" = 0 ] || { echo "render smoke failed (exit $rc)" >&2; grep -v "dbus" "$work/render-$which.log" | tail -30 >&2; exit 1; }
fi
sum=$(cd "$out" && find . -type f ! -name '*.gz' -print0 | sort -z | xargs -0 shasum -a 256 | shasum -a 256 | cut -d' ' -f1)
echo "built $which: output $out sha256 $sum"
