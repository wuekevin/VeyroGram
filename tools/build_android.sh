#!/usr/bin/env bash
# Reproducible unsigned debug build of the Android client (Story 1.9): pinned upstream SHA + VeyroGram patch queue, source policy gate,
# Gradle dependency audit, debug APK + JVM unit tests, APK identity checks and a content hash. The upstream checkout is never modified.
# Usage: tools/build_android.sh <workdir>   (must not exist; needs JDK 17 [VG_JAVA_HOME|JAVA_HOME|java_home -v 17] and the Android SDK [ANDROID_HOME])
# Env: VG_SKIP_AUDIT=1 skips the network audit. Build under $HOME, not /tmp.
set -euo pipefail
work=${1:?usage: build_android.sh <workdir>}
root=$(cd "$(dirname "$0")/.." && pwd)
up="$root/upstream/TangSengDaoDaoAndroid"
case "$work" in /*) ;; *) work="$PWD/$work";; esac
[ ! -e "$work" ] || { echo "$work already exists" >&2; exit 2; }
pin=$(tr -d '[:space:]' < "$root/apps/android/PIN")
git -C "$up" cat-file -e "$pin^{commit}" 2>/dev/null || { echo "upstream android does not contain pinned commit $pin" >&2; exit 1; }
jh=${VG_JAVA_HOME:-}
[ -n "$jh" ] || { [ -n "${JAVA_HOME:-}" ] && "$JAVA_HOME/bin/java" -version 2>&1 | grep -q 'version "17' && jh=$JAVA_HOME; } || true
[ -n "$jh" ] || jh=$(/usr/libexec/java_home -v 17 2>/dev/null) || true
[ -n "$jh" ] || { [ -d /opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home ] && jh=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home; } || true
[ -n "$jh" ] || { echo "JDK 17 not found (set VG_JAVA_HOME)" >&2; exit 2; }
export JAVA_HOME=$jh
export ANDROID_HOME=${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$HOME/Library/Android/sdk}}
[ -d "$ANDROID_HOME/platforms" ] || { echo "Android SDK not found (set ANDROID_HOME)" >&2; exit 2; }
git -C "$up" worktree add --detach -q "$work" "$pin"
n=0
for p in "$root"/apps/android/patches/*.patch; do
  git -C "$work" apply --index "$p" || { echo "patch $(basename "$p") does not apply to $pin" >&2; git -C "$up" worktree remove --force "$work" >/dev/null 2>&1; exit 1; }
  n=$((n+1))
done
# Whole-file overlays and deletions are kept out of the patch so the removed vendor credentials never appear in this repo.
rm -f "$work/app/google-services.json"
[ ! -d "$root/apps/android/overlay" ] || cp -R "$root/apps/android/overlay/." "$work/"
echo "patched android at ${pin:0:8} with $n patch(es)"
python3 -I "$root/tools/check_android.py" "$work" || exit 1
# Optional FCM: the operator's own Firebase config (never the vendor's, never committed). Copied after the policy check, which forbids the file in the tree.
if [ -n "${VG_GOOGLE_SERVICES_JSON:-}" ]; then
  python3 -I - "$VG_GOOGLE_SERVICES_JSON" "${VG_APPLICATION_ID:-com.veyrogram.app}" <<'PY' || exit 1
import json, sys
d = json.load(open(sys.argv[1])); pkgs = [c["client_info"]["android_client_info"]["package_name"] for c in d.get("client", [])]
if sys.argv[2] not in pkgs: sys.exit(f"google-services.json has no client for {sys.argv[2]} (found {pkgs})")
if d.get("project_info", {}).get("project_id") == "tsdddemo": sys.exit("google-services.json is the upstream vendor project (tsdddemo)")
PY
  cp "$VG_GOOGLE_SERVICES_JSON" "$work/app/google-services.json"; echo "FCM enabled with operator google-services.json"
fi
cd "$work"
g="./gradlew --no-daemon -q"
if [ "${VG_SKIP_AUDIT:-0}" != 1 ]; then
  $g :app:dependencies --configuration debugRuntimeClasspath > "$work/deps.txt"
  python3 -I "$root/tools/audit_gradle.py" "$work/deps.txt"
fi
$g :app:assembleDebug testDebugUnitTest > "$work/build.log" 2>&1 || { tail -40 "$work/build.log" >&2; exit 1; }
apk="$work/app/build/outputs/apk/debug/app-debug.apk"
aapt=$(ls "$ANDROID_HOME"/build-tools/*/aapt2 | sort -V | tail -1)
badging=$("$aapt" dump badging "$apk")
case "$badging" in "package: name='com.veyrogram.app'"*) ;; *)  echo "APK package is not com.veyrogram.app" >&2; exit 1;; esac
case "$badging" in *"minSdkVersion:'23'"*) ;; *) echo "minSdk is not 23 (Android 6.0)" >&2; exit 1;; esac
tests=$(find . -path '*/test-results/testDebugUnitTest/TEST-*.xml' | wc -l | tr -d ' ')
fails=$(grep -l '<failure\|<error' $(find . -path '*/test-results/testDebugUnitTest/TEST-*.xml') 2>/dev/null | wc -l | tr -d ' ') || true
echo "unit test classes: $tests, with failures: $fails"
[ "$fails" = 0 ] || exit 1
x="$work/apk-extract"; mkdir -p "$x"; unzip -q "$apk" -d "$x"
if grep -rlaE "108677047|d14dac0803|tsdddemo|botgate|com\.test\.ts" "$x" >/dev/null 2>&1; then echo "vendor identity string found in the APK" >&2; exit 1; fi
echo "apk sha256 $(shasum -a 256 "$apk" | cut -d' ' -f1)"
# Optional signed release (R8-minified): set VG_KEYSTORE, VG_KEYSTORE_PASSWORD, VG_KEY_ALIAS, VG_KEY_PASSWORD (never commit them).
if [ -n "${VG_KEYSTORE:-}" ]; then
  $g :app:assembleRelease > "$work/release.log" 2>&1 || { tail -40 "$work/release.log" >&2; exit 1; }
  rel="$work/app/build/outputs/apk/release/app-release.apk"
  apksigner=$(ls "$ANDROID_HOME"/build-tools/*/apksigner | sort -V | tail -1)
  "$apksigner" verify "$rel" || { echo "release APK signature does not verify" >&2; exit 1; }
  echo "release apk (signed) sha256 $(shasum -a 256 "$rel" | cut -d' ' -f1)"
fi
