#!/usr/bin/env bash
# Builds and installs the Android IM SDK probe (WuKongIMAndroidSDK, the version the Android client ships) on the ONE connected
# device/emulator, then runs the interop tests against the pinned, patched WuKongIM (Story 1.11 AC2).
# Needs: JDK 17, Android SDK (ANDROID_HOME), exactly one adb device online, VG_TEST_WUKONGIM_BIN (tools/build_wukongim.sh),
# node >= 22 and `npm ci --ignore-scripts` in tests/imsdk/web (the tests also cross to the Web SDK).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
export ANDROID_HOME=${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$HOME/Library/Android/sdk}}
jh=${VG_JAVA_HOME:-}; [ -n "$jh" ] || jh=$(/usr/libexec/java_home -v 17 2>/dev/null) || true
[ -n "$jh" ] || [ ! -d /opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home ] || jh=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home
[ -n "$jh" ] || { echo "JDK 17 not found (set VG_JAVA_HOME)" >&2; exit 2; }
export JAVA_HOME=$jh
adb="$ANDROID_HOME/platform-tools/adb"
[ "$("$adb" devices | tail -n +2 | grep -c 'device$')" = 1 ] || { echo "need exactly one adb device online" >&2; exit 2; }
(cd "$here" && ./gradlew --no-daemon -q :app:assembleDebug :app:assembleDebugAndroidTest)
"$adb" install -r -t "$here/app/build/outputs/apk/debug/app-debug.apk" >/dev/null
"$adb" install -r -t "$here/app/build/outputs/apk/androidTest/debug/app-debug-androidTest.apk" >/dev/null
cd "$root/services/api"
export GOTOOLCHAIN=${GOTOOLCHAIN:-go1.26.11} VG_REQUIRE_IMSDK_ANDROID=1
exec go test -count=1 -run 'Android' -v ./internal/platform/testim/
