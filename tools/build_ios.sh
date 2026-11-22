#!/usr/bin/env bash
# Unsigned iOS Simulator build (Story 1.9): pinned upstream SHA + VeyroGram patch queue, source policy gate, xcodebuild with signing off.
# Usage: tools/build_ios.sh <workdir>   (must not exist; macOS with Xcode 26 and the Metal Toolchain: xcodebuild -downloadComponent MetalToolchain)
set -euo pipefail
work=${1:?usage: build_ios.sh <workdir>}
root=$(cd "$(dirname "$0")/.." && pwd)
up="$root/upstream/TangSengDaoDaoiOS"
case "$work" in /*) ;; *) work="$PWD/$work";; esac
[ ! -e "$work" ] || { echo "$work already exists" >&2; exit 2; }
pin=$(tr -d '[:space:]' < "$root/apps/ios/PIN")
git -C "$up" cat-file -e "$pin^{commit}" 2>/dev/null || { echo "upstream ios does not contain pinned commit $pin" >&2; exit 1; }
xcrun metal --version >/dev/null 2>&1 || { echo "Metal Toolchain missing: xcodebuild -downloadComponent MetalToolchain" >&2; exit 2; }
git -C "$up" worktree add --detach -q "$work" "$pin"
for p in "$root"/apps/ios/patches/*.patch; do
  git -C "$work" apply --index "$p" || { echo "patch $(basename "$p") does not apply" >&2; git -C "$up" worktree remove --force "$work" >/dev/null 2>&1; exit 1; }
done
python3 -I "$root/tools/check_ios.py" "$work" || exit 1
cd "$work"
dd="$work/DerivedData"
xcodebuild -workspace TangSengDaoDaoiOS.xcworkspace -scheme WuKongChatiOS -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' \
  -derivedDataPath "$dd" CODE_SIGNING_ALLOWED=NO CODE_SIGN_IDENTITY="" build > "$work/build.log" 2>&1 || { grep 'error:' "$work/build.log" | head -20 >&2; exit 1; }
app=$(find "$dd/Build/Products" -maxdepth 2 -name '*.app' | head -1)
[ -n "$app" ] || { echo "no .app produced" >&2; exit 1; }
id=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$app/Info.plist")
[ "$id" = app.veyrogram.ios ] || { echo "bundle id is $id" >&2; exit 1; }
echo "ios simulator app built: $id ($(du -sk "$app" | cut -f1) KB)"
