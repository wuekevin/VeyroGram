#!/usr/bin/env bash
# Build the pinned WuKongIM plus VeyroGram's patch queue (patches/wukongim/*.patch). Usage: tools/build_wukongim.sh <output-binary>
# Fails if upstream/WuKongIM is not at the pinned SHA, and if any patch does not apply cleanly. The upstream checkout is never modified:
# the build happens in a throwaway detached worktree.
set -euo pipefail
PIN=7b810e08
out=${1:?usage: build_wukongim.sh <output-binary>}
root=$(cd "$(dirname "$0")/.." && pwd)
case "$out" in /*) ;; *) out="$PWD/$out" ;; esac
head=$(git -C "$root/upstream/WuKongIM" rev-parse --short=8 HEAD)
[ "$head" = "$PIN" ] || { echo "upstream/WuKongIM is at $head, expected pinned $PIN" >&2; exit 1; }
tmp=$(mktemp -d)
trap 'git -C "$root/upstream/WuKongIM" worktree remove --force "$tmp/src" >/dev/null 2>&1 || true; rm -rf "$tmp"' EXIT
git -C "$root/upstream/WuKongIM" worktree add --detach -q "$tmp/src" "$PIN"
n=0
for p in "$root"/patches/wukongim/*.patch; do
  [ -e "$p" ] || continue
  git -C "$tmp/src" apply --index "$p" || { echo "patch $(basename "$p") does not apply to $PIN" >&2; exit 1; }
  n=$((n+1))
done
(cd "$tmp/src" && GOTOOLCHAIN=${WK_GOTOOLCHAIN:-go1.26.11} go build -o "$out" ./cmd/wukongim)
echo "built WuKongIM $PIN + $n patch(es) -> $out"
