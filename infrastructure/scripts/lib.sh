# shellcheck shell=bash
# Shared helpers for deploy.sh / rollback.sh (sourced, not executed).
VG_PREFIX=${VG_PREFIX:-/opt/veyrogram/api}
VG_ENV_FILE=${VG_ENV_FILE:-/etc/veyrogram/api.env}
VG_RESTART_CMD=${VG_RESTART_CMD:-systemctl restart veyrogram-api}
VG_READY_URL=${VG_READY_URL:-http://127.0.0.1:9101/readyz}
VG_READY_TIMEOUT=${VG_READY_TIMEOUT:-30}
VG_KEEP_RELEASES=${VG_KEEP_RELEASES:-5}

log() { printf '%s %s\n' "$(date -u +%FT%TZ)" "$*" >&2; }
die() { log "ERROR: $*"; exit 1; }

valid_version() { [[ $1 =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]]; }

# Run a binary's -check-config with the env file loaded into a CLEAN environment (no leakage from the caller's shell).
preflight() {
  local bin=$1 line k v; local -a vars=()
  [[ -r $VG_ENV_FILE ]] || die "env file $VG_ENV_FILE not readable"
  while IFS= read -r line || [[ -n $line ]]; do
    [[ -z ${line//[[:space:]]/} || $line == \#* ]] && continue
    k=${line%%=*}; v=${line#*=}
    v=${v%\"}; v=${v#\"}; v=${v%\'}; v=${v#\'}
    [[ $k =~ ^VG_[A-Z0-9_]+$ ]] || continue
    vars+=("$k=$v")
  done <"$VG_ENV_FILE"
  env -i PATH="$PATH" "${vars[@]}" "$bin" -check-config >/dev/null
}

current_target() { [[ -L $VG_PREFIX/current ]] && basename "$(readlink "$VG_PREFIX/current")" || true; }

# Point `current` at releases/<version>. Atomic where GNU mv -T exists, otherwise a short unlink+symlink window.
switch_to() {
  local version=$1 tmp=$VG_PREFIX/.current.new
  [[ -d $VG_PREFIX/releases/$version ]] || die "release $version not installed"
  ln -sfn "releases/$version" "$tmp"
  if mv --help 2>&1 | grep -q -- '-T'; then mv -Tf "$tmp" "$VG_PREFIX/current"; else rm -f "$VG_PREFIX/current"; mv "$tmp" "$VG_PREFIX/current"; fi
}

restart() { log "restarting: $VG_RESTART_CMD"; bash -c "$VG_RESTART_CMD"; }

wait_ready() {
  local i=0
  while (( i < VG_READY_TIMEOUT )); do
    if curl -fsS --max-time 2 "$VG_READY_URL" >/dev/null 2>&1; then return 0; fi
    sleep 1; i=$((i + 1))
  done
  return 1
}
