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

canary_version() { [[ -s $VG_PREFIX/.canary ]] && awk '{print $1}' "$VG_PREFIX/.canary" || true; }

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

# ---- shared by deploy.sh and canary.sh ----
sha_of() { (sha256sum "$1" 2>/dev/null || shasum -a 256 "$1") | awk '{print $1}'; }

# Verify <binary>.sha256, install it as the immutable release <version>, and run the configuration preflight. Nothing is switched.
stage_release() {
  local bin=$1 version=$2 rel want got
  valid_version "$version" || die "invalid version string '$version'"
  [[ -f $bin ]] || die "binary $bin not found"
  [[ -f $bin.sha256 ]] || die "checksum file $bin.sha256 is required"
  want=$(awk '{print $1}' "$bin.sha256"); got=$(sha_of "$bin")
  [[ $want == "$got" ]] || die "checksum mismatch for $bin (refusing to deploy)"
  rel=$VG_PREFIX/releases/$version
  [[ ! -e $rel ]] || die "release $version already exists (versions are immutable)"
  mkdir -p "$VG_PREFIX/releases"
  install -d "$rel"; install -m 0755 "$bin" "$rel/veyrogram-api"; cp "$bin.sha256" "$rel/veyrogram-api.sha256"
  if ! preflight "$rel/veyrogram-api"; then rm -rf "$rel"; die "preflight (-check-config) failed; nothing was switched"; fi
}

# Make releases/<version> the live release: switch, restart, wait for readiness; on failure switch back to the previous release.
# Returns 0 on success, 1 on a failure that was rolled back, 2 when even the rollback is unhealthy (manual intervention).
activate_release() {
  local version=$1 prev
  prev=$(current_target)
  switch_to "$version"; echo "$prev" >"$VG_PREFIX/.previous"
  restart
  if wait_ready; then log "deployed $version (previous: ${prev:-none})"; return 0; fi
  log "readiness check failed within ${VG_READY_TIMEOUT}s"
  if [[ -n $prev && -d $VG_PREFIX/releases/$prev ]]; then
    log "rolling back to $prev"; switch_to "$prev"; restart
    if wait_ready; then log "ERROR: deploy of $version failed; rolled back to $prev (healthy)"; return 1; fi
    log "ERROR: deploy of $version failed AND rollback to $prev is not healthy — manual intervention required"; return 2
  fi
  log "ERROR: deploy of $version failed and there is no previous release to roll back to"; return 1
}

# Delete old releases beyond VG_KEEP_RELEASES, never the ones named as arguments (current, previous, canary...).
prune_releases() {
  local r keep=" $* "
  ls -1t "$VG_PREFIX/releases" | tail -n +$((VG_KEEP_RELEASES + 1)) | while read -r r; do
    if [[ -n $r && $keep != *" $r "* ]]; then rm -rf "${VG_PREFIX:?}/releases/$r"; fi
  done
}
