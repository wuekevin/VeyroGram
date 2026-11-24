#!/usr/bin/env bash
# Builds the iOS IM SDK probe (WuKongIMSDK from the source pinned by apps/ios/PIN) for the simulator, installs it on a simulator and
# runs the interop tests against the pinned, patched WuKongIM (Story 1.11 AC2). Needs Xcode, CocoaPods, network for `pod install`
# (CocoaAsyncSocket, FMDB/SQLCipher, 25519 from the CocoaPods trunk), VG_TEST_WUKONGIM_BIN, node >= 22 + `npm ci --ignore-scripts` in tests/imsdk/web.
# The simulator build swaps the vendored device-only AMR codec archives for weak stubs (voice codecs are not under test; same as tools/build_ios.sh).
# Env: VG_IOS_SIM = simulator UDID (default: the booted one, else the first available iPhone).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
up="$root/upstream/TangSengDaoDaoiOS"; pin=$(tr -d '[:space:]' < "$root/apps/ios/PIN")
git -C "$up" cat-file -e "$pin^{commit}" 2>/dev/null || { echo "upstream iOS does not contain pinned commit $pin" >&2; exit 1; }
cd "$here"; rm -rf build; mkdir -p build/sdk
git -C "$up" archive "$pin" Modules/WuKongIMiOSSDK | tar -x -C build/sdk
amrdir="build/sdk/Modules/WuKongIMiOSSDK/WuKongIMSDK/Classes/private/arm/lib"; stub=$(mktemp -d)
for a in libopencore-amrnb libopencore-amrwb libvo-amrwbenc; do
  lipo "$amrdir/$a.a" -thin arm64 -output "$stub/$a.thin.a"
  { echo "/* simulator-only stub of $a */"; nm -g -U "$stub/$a.thin.a" 2>/dev/null | awk '$2=="T"{sub(/^_/,"",$3); print "__attribute__((weak)) void *" $3 "(void) { return 0; }"}' | sort -u; } > "$stub/$a.c"
  xcrun --sdk iphonesimulator clang -arch arm64 -target arm64-apple-ios13.0-simulator -c "$stub/$a.c" -o "$stub/$a.o"
  rm -f "$amrdir/$a.a"; ar rcs "$amrdir/$a.a" "$stub/$a.o"
done
rm -rf "$stub"
pod install --silent
uuid='[0-9A-F]\{8\}-[0-9A-F-]\{27\}'
sim=${VG_IOS_SIM:-$(xcrun simctl list devices booted | grep -o "$uuid" | head -1 || true)}
[ -n "$sim" ] || sim=$(xcrun simctl list devices available | grep -m1 'iPhone' | grep -o "$uuid" || true)
[ -n "$sim" ] || { echo "no iOS simulator available" >&2; exit 2; }
xcrun simctl boot "$sim" 2>/dev/null || true; xcrun simctl bootstatus "$sim" -b >/dev/null
xcodebuild -workspace Probe.xcworkspace -scheme Probe -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' -derivedDataPath build/dd \
  ARCHS=arm64 EXCLUDED_ARCHS= ONLY_ACTIVE_ARCH=YES CODE_SIGNING_ALLOWED=NO build -quiet
xcrun simctl install "$sim" build/dd/Build/Products/Debug-iphonesimulator/Probe.app
cd "$root/services/api"
export GOTOOLCHAIN=${GOTOOLCHAIN:-go1.26.11} VG_REQUIRE_IMSDK_IOS=1 VG_IOS_SIM="$sim"
exec go test -count=1 -run 'IOS' -v ./internal/platform/testim/
