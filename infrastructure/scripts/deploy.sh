#!/usr/bin/env bash
# Deploy a versioned, checksum-verified binary with preflight and automatic rollback.
#   deploy.sh <binary> <version>        (expects <binary>.sha256 next to the binary)
# Env: VG_PREFIX VG_ENV_FILE VG_RESTART_CMD VG_READY_URL VG_READY_TIMEOUT VG_KEEP_RELEASES (see lib.sh)
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); . "$here/lib.sh"
bin=${1:?usage: deploy.sh <binary> <version>}; version=${2:?usage: deploy.sh <binary> <version>}
valid_version "$version" || die "invalid version string '$version'"
[[ -f $bin ]] || die "binary $bin not found"
[[ -f $bin.sha256 ]] || die "checksum file $bin.sha256 is required"

want=$(awk '{print $1}' "$bin.sha256"); got=$( (sha256sum "$bin" 2>/dev/null || shasum -a 256 "$bin") | awk '{print $1}')
[[ $want == "$got" ]] || die "checksum mismatch for $bin (refusing to deploy)"

rel=$VG_PREFIX/releases/$version
[[ ! -e $rel ]] || die "release $version already exists (versions are immutable)"
mkdir -p "$VG_PREFIX/releases"
install -d "$rel"; install -m 0755 "$bin" "$rel/veyrogram-api"; cp "$bin.sha256" "$rel/veyrogram-api.sha256"

if ! preflight "$rel/veyrogram-api"; then rm -rf "$rel"; die "preflight (-check-config) failed; nothing was switched"; fi

prev=$(current_target)
switch_to "$version"; echo "$prev" >"$VG_PREFIX/.previous"
restart
if wait_ready; then
  log "deployed $version (previous: ${prev:-none})"
  # prune old releases, never the current or previous one
  ls -1t "$VG_PREFIX/releases" | tail -n +$((VG_KEEP_RELEASES + 1)) | while read -r r; do
    if [[ -n $r && $r != "$version" && $r != "$prev" ]]; then rm -rf "${VG_PREFIX:?}/releases/$r"; fi
  done
  exit 0
fi

log "readiness check failed within ${VG_READY_TIMEOUT}s"
if [[ -n $prev && -d $VG_PREFIX/releases/$prev ]]; then
  log "rolling back to $prev"; switch_to "$prev"; restart
  wait_ready && die "deploy of $version failed; rolled back to $prev (healthy)" || die "deploy of $version failed AND rollback to $prev is not healthy — manual intervention required"
fi
die "deploy of $version failed and there is no previous release to roll back to"
