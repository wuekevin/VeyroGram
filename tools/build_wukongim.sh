#!/usr/bin/env bash
# Build the pinned WuKongIM for contract tests. Usage: tools/build_wukongim.sh <output-binary>
# Fails if upstream/WuKongIM is not at the pinned SHA, so contract tests can never silently run against a different revision.
set -euo pipefail
PIN=7b810e08
out=${1:?usage: build_wukongim.sh <output-binary>}
root=$(cd "$(dirname "$0")/.." && pwd)
head=$(git -C "$root/upstream/WuKongIM" rev-parse --short=8 HEAD)
[ "$head" = "$PIN" ] || { echo "upstream/WuKongIM is at $head, expected pinned $PIN" >&2; exit 1; }
cd "$root/upstream/WuKongIM"
GOTOOLCHAIN=go1.26.10 go build -o "$out" ./cmd/wukongim
echo "built WuKongIM $PIN -> $out"
