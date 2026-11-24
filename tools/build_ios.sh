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
# The 2015-era prebuilt WebP.framework bundled in YYImage is replaced by the libwebp pod (see patch); the binary itself is deleted here
# so it never ships (a deletion of a binary would bloat the patch).
rm -rf "$work/Pods/YYImage/Vendor/WebP.framework"
python3 -I "$root/tools/check_ios.py" "$work" || exit 1
cd "$work"
# iOS 26 simulators are arm64-only, but the vendored libopencore-amr*/libvo-amrwbenc archives only carry a device arm64 slice, so the
# simulator link fails. For the simulator build ONLY, swap them for weak stubs of the same symbols (voice-message codecs are then
# unavailable in the simulator build); the device build below restores the real archives. Nothing from this ships.
amrdir="$work/Modules/WuKongIMiOSSDK/WuKongIMSDK/Classes/private/arm/lib"
stub=$(mktemp -d)
for a in libopencore-amrnb libopencore-amrwb libvo-amrwbenc; do
  lipo "$amrdir/$a.a" -thin arm64 -output "$stub/$a.thin.a"
  { echo "/* simulator-only stub of $a */"; nm -g -U "$stub/$a.thin.a" 2>/dev/null | awk '$2=="T"{sub(/^_/,"",$3); print "__attribute__((weak)) void *" $3 "(void) { return 0; }"}' | sort -u; } > "$stub/$a.c"
  xcrun --sdk iphonesimulator clang -arch arm64 -target arm64-apple-ios13.0-simulator -c "$stub/$a.c" -o "$stub/$a.o"
  rm -f "$amrdir/$a.a"; ar rcs "$amrdir/$a.a" "$stub/$a.o"
done
# Same for the prebuilt Bugly.framework (device-only slices): a do-nothing Bugly/BuglyConfig built from its own headers, simulator build only.
bug="$work/Modules/WuKongBase/WuKongBase/Bugly.framework"
cat > "$stub/bugly_stub.m" <<'EOF'
#import <Foundation/Foundation.h>
#import "Bugly.h"
@implementation BuglyConfig
@end
@implementation Bugly
+ (void)startWithAppId:(NSString *)appId config:(BuglyConfig *)config {}
+ (void)setUserIdentifier:(NSString *)userId {}
@end
EOF
xcrun --sdk iphonesimulator clang -arch arm64 -target arm64-apple-ios13.0-simulator -fobjc-arc -w -I "$bug/Headers" -c "$stub/bugly_stub.m" -o "$stub/bugly_stub.o"
rm -f "$bug/Bugly"; ar rcs "$bug/Bugly" "$stub/bugly_stub.o"
rm -rf "$stub"
# Unsigned simulator binaries carry no entitlements, so every Keychain call fails with -34018 and the session could not be saved. Embed the
# simulated entitlements Xcode would add when signing (simulator build only; the device build is signed later and gets them from signing).
ents="$work/vg-sim-entitlements.plist"
cat > "$ents" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>
<key>application-identifier</key><string>com.veyrogram.app</string>
<key>keychain-access-groups</key><array><string>com.veyrogram.app</string></array></dict></plist>
EOF
dd="$work/DerivedData"
xcodebuild -workspace TangSengDaoDaoiOS.xcworkspace -scheme WuKongChatiOS -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' ARCHS=arm64 EXCLUDED_ARCHS= ENABLE_DEBUG_DYLIB=NO OTHER_LDFLAGS='$(inherited) -Wl,-sectcreate,__TEXT,__entitlements,'"$ents" \
  -derivedDataPath "$dd" CODE_SIGNING_ALLOWED=NO CODE_SIGN_IDENTITY="" build > "$work/build.log" 2>&1 || { grep 'error:' "$work/build.log" | head -20 >&2; exit 1; }
app=$(find "$dd/Build/Products" -maxdepth 2 -name '*.app' | head -1)
[ -n "$app" ] || { echo "no .app produced" >&2; exit 1; }
id=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$app/Info.plist")
[ "$id" = com.veyrogram.app ] || { echo "bundle id is $id" >&2; exit 1; }
echo "ios simulator app built: $id ($(du -sk "$app" | cut -f1) KB)"
git -C "$work" checkout -- Modules/WuKongIMiOSSDK/WuKongIMSDK/Classes/private/arm/lib Modules/WuKongBase/WuKongBase/Bugly.framework   # restore the real AMR archives for the device build
# Unsigned device build (iphoneos) and an unsigned .ipa (a zip of Payload/<app>). Signing happens later in the release pipeline (checklist C14).
xcodebuild -workspace TangSengDaoDaoiOS.xcworkspace -scheme WuKongChatiOS -sdk iphoneos -destination 'generic/platform=iOS' \
  -derivedDataPath "$work/DerivedDataDevice" CODE_SIGNING_ALLOWED=NO CODE_SIGN_IDENTITY="" build > "$work/build-device.log" 2>&1 || { grep 'error:' "$work/build-device.log" | head -20 >&2; exit 1; }
dapp=$(find "$work/DerivedDataDevice/Build/Products" -maxdepth 2 -name '*.app' | head -1)
[ -n "$dapp" ] || { echo "no device .app produced" >&2; exit 1; }
did=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$dapp/Info.plist")
[ "$did" = com.veyrogram.app ] || { echo "device bundle id is $did" >&2; exit 1; }
mkdir -p "$work/ipa/Payload" && cp -R "$dapp" "$work/ipa/Payload/" && (cd "$work/ipa" && zip -qry "$work/VeyroGram-unsigned.ipa" Payload)
if codesign -dv "$dapp" >/dev/null 2>&1; then echo "device .app is unexpectedly signed" >&2; exit 1; fi
echo "unsigned device app: $did; ipa: $work/VeyroGram-unsigned.ipa ($(du -k "$work/VeyroGram-unsigned.ipa" | cut -f1) KB, sha256 $(shasum -a 256 "$work/VeyroGram-unsigned.ipa" | cut -d' ' -f1))"
