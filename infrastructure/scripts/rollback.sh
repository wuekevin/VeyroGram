#!/usr/bin/env bash
# Roll back to the previous release (or a named installed one):  rollback.sh [version]
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); . "$here/lib.sh"
target=${1:-}
if [[ -z $target ]]; then [[ -s $VG_PREFIX/.previous ]] || die "no previous release recorded"; target=$(cat "$VG_PREFIX/.previous"); fi
valid_version "$target" || die "invalid version '$target'"
cur=$(current_target)
[[ $cur != "$target" ]] || die "$target is already current"
switch_to "$target"; echo "$cur" >"$VG_PREFIX/.previous"
restart
wait_ready && log "rolled back to $target (was: $cur)" || die "rolled back to $target but readiness failed"
