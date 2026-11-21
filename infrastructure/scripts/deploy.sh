#!/usr/bin/env bash
# Deploy a versioned, checksum-verified binary with preflight and automatic rollback.
#   deploy.sh <binary> <version>        (expects <binary>.sha256 next to the binary)
# For a gradual rollout to a share of traffic first, use canary.sh.
# Env: VG_PREFIX VG_ENV_FILE VG_RESTART_CMD VG_READY_URL VG_READY_TIMEOUT VG_KEEP_RELEASES (see lib.sh)
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); . "$here/lib.sh"
bin=${1:?usage: deploy.sh <binary> <version>}; version=${2:?usage: deploy.sh <binary> <version>}
stage_release "$bin" "$version"
prev=$(current_target)
rc=0; activate_release "$version" || rc=$?
if (( rc == 0 )); then prune_releases "$version" "$prev" "$(canary_version)"; fi
exit "$rc"
