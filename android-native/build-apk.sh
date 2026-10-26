#!/usr/bin/env bash
# Builds a signed debug APK without Gradle, using the Android SDK build-tools
# directly (aapt2 -> javac -> d8 -> zipalign -> apksigner).
#
# Usage: ./build-apk.sh
#
# Everything can be overridden from the environment:
#   ANDROID_JAR, NATIVE_BT (aapt2/zipalign), JAVA_BT (d8/apksigner)
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
app="$here/app/src/main"
build="$here/build"
out="$here/out"

ANDROID_JAR="${ANDROID_JAR:-}"
if [[ -z "$ANDROID_JAR" ]]; then
  # aapt2 2.19 (the only native arm64 build available here) cannot read the
  # resources.arsc shipped in API 30+ platforms, so prefer an API 29 platform.
  for candidate in /opt/andsdk/platform29/android-10/android.jar \
                   /usr/lib/android-sdk/platforms/android-29/android.jar \
                   /opt/andsdk/android-36/android.jar; do
    if [[ -f "$candidate" ]]; then ANDROID_JAR="$candidate"; break; fi
  done
fi

# aapt2 and zipalign are native binaries, so they must match the host CPU.
NATIVE_BT="${NATIVE_BT:-/usr/lib/android-sdk/build-tools/debian}"
# d8 and apksigner are Java programs: any recent build-tools copy works.
JAVA_BT="${JAVA_BT:-/opt/andsdk/android-37.0}"

AAPT2="${AAPT2:-$NATIVE_BT/aapt2}"
ZIPALIGN="${ZIPALIGN:-$NATIVE_BT/zipalign}"
D8="${D8:-$JAVA_BT/d8}"
APKSIGNER="${APKSIGNER:-$JAVA_BT/apksigner}"

for f in "$ANDROID_JAR" "$AAPT2" "$ZIPALIGN" "$D8" "$APKSIGNER"; do
  [[ -e "$f" ]] || { echo "missing tool: $f" >&2; exit 1; }
done

SO="$app/jniLibs/arm64-v8a/libamdl.so"
[[ -f "$SO" ]] || { echo "missing core: $SO (run ./build-core.sh first)" >&2; exit 1; }
# ffmpeg 是转码用的可执行文件，同样放进 jniLibs：只有 nativeLibraryDir 里的文件
# 在 Android 10+ 上才允许执行（filesDir 等是 noexec）。
FFMPEG="$app/jniLibs/arm64-v8a/libffmpeg.so"
[[ -f "$FFMPEG" ]] || { echo "missing ffmpeg: $FFMPEG (run ./fetch-ffmpeg.sh)" >&2; exit 1; }
# 兼容层：改写上游不兼容的参数并补回封面，见 go-shim/main.go
SHIM="$app/jniLibs/arm64-v8a/libffmpegshim.so"
[[ -f "$SHIM" ]] || { echo "missing shim: $SHIM (run ./build-shim.sh)" >&2; exit 1; }

MIN_SDK=26
TARGET_SDK=35
# Bump both on every release: Android only offers an in-place update when the
# version code grows, and the signature stays identical (see signing/README.md).
# versionCode = major*10000 + minor*100 + patch，与 VERSION_NAME 一一对应：
# 1.0.0 -> 10000、1.0.1 -> 10001。发版只改 VERSION_NAME，code 不会忘记加。
VERSION_NAME="${VERSION_NAME:-1.0.0}"
VERSION_CODE="${VERSION_CODE:-$(awk -F. '{printf "%d", $1*10000 + $2*100 + $3}' <<< "${VERSION_NAME%%.*-*}" 2>/dev/null || echo 10000)}"

rm -rf "$build" && mkdir -p "$build/res" "$build/gen" "$build/classes" "$build/dex" "$build/stage/lib/arm64-v8a" "$out"

echo "==> aapt2 compile"
"$AAPT2" compile --dir "$app/res" -o "$build/res/resources.zip"

# The source manifest carries no package attribute (Gradle injects the
# namespace itself and rejects a stale package=). aapt2 still needs one, so a
# copy with the attribute is generated for this build.
MANIFEST="$build/AndroidManifest.xml"
sed '0,/^<manifest /s|^<manifest \(.*\)>$|<manifest \1 package="com.amdl.nativeapp">|' \
  "$app/AndroidManifest.xml" > "$MANIFEST"
grep -q 'package="com.amdl.nativeapp"' "$MANIFEST" || {
  echo "failed to inject package attribute into manifest" >&2; exit 1; }

echo "==> aapt2 link"
"$AAPT2" link \
  -o "$build/stage/base.apk" \
  -I "$ANDROID_JAR" \
  --manifest "$MANIFEST" \
  -R "$build/res/resources.zip" \
  -A "$app/assets" \
  --java "$build/gen" \
  --auto-add-overlay \
  --min-sdk-version "$MIN_SDK" \
  --target-sdk-version "$TARGET_SDK" \
  --version-code "$VERSION_CODE" \
  --version-name "$VERSION_NAME"

echo "==> javac"
# shellcheck disable=SC2046
# -bootclasspath makes android.jar the only API surface: anything from the host
# JDK that Android lacks becomes a compile error instead of a NoSuchMethodError
# at runtime. (This is why the sources avoid lambdas and streams.)
javac -encoding UTF-8 -source 8 -target 8 -nowarn -Xlint:-options \
  -bootclasspath "$ANDROID_JAR" \
  -d "$build/classes" \
  $(find "$app/java" "$build/gen" -name '*.java')

echo "==> d8"
# shellcheck disable=SC2046
"$D8" --release --min-api "$MIN_SDK" --lib "$ANDROID_JAR" \
  --output "$build/dex" $(find "$build/classes" -name '*.class')

echo "==> package"
cp "$build/stage/base.apk" "$build/stage/app.apk"
cp "$SO" "$build/stage/lib/arm64-v8a/libamdl.so"
cp "$FFMPEG" "$build/stage/lib/arm64-v8a/libffmpeg.so"
cp "$SHIM" "$build/stage/lib/arm64-v8a/libffmpegshim.so"
cp "$build/dex/classes.dex" "$build/stage/classes.dex"
pushd "$build/stage" >/dev/null
# Store the dex and the download core uncompressed so zipalign can page-align
# them; ffmpeg is only extracted at install time, so it is compressed to keep
# the download small.
zip -q -X -0 app.apk classes.dex
zip -q -X -0 app.apk lib/arm64-v8a/libamdl.so
zip -q -X app.apk lib/arm64-v8a/libffmpeg.so
zip -q -X app.apk lib/arm64-v8a/libffmpegshim.so
popd >/dev/null

echo "==> zipalign"
"$ZIPALIGN" -f -p 4 "$build/stage/app.apk" "$build/stage/app-aligned.apk"

echo "==> sign"
# The signing identity must stay stable across rebuilds, otherwise Android
# refuses the update (INSTALL_FAILED_UPDATE_INCOMPATIBLE) and the user has to
# uninstall first. The key therefore lives in signing/ (tracked by git) and must
# never sit below build/, which this script wipes on every run.
#
# Overrides for CI or a private release key:
#   AMDL_KEYSTORE / AMDL_KEYSTORE_PASSWORD / AMDL_KEY_PASSWORD / AMDL_KEY_ALIAS
KS="${AMDL_KEYSTORE:-$here/signing/amdl-release.keystore}"
KS_PASS="${AMDL_KEYSTORE_PASSWORD:-android}"
KEY_PASS="${AMDL_KEY_PASSWORD:-$KS_PASS}"
KEY_ALIAS="${AMDL_KEY_ALIAS:-androiddebugkey}"
FINGERPRINT_FILE="$here/signing/fingerprint.txt"

if [[ ! -f "$KS" ]]; then
  mkdir -p "$(dirname "$KS")"
  cat >&2 <<WARN
!! $KS not found: generating a NEW signing key.
!! Every already installed build will have to be uninstalled before it can be
!! replaced. Restore the original keystore if you want in-place upgrades.
WARN
  keytool -genkeypair -v \
    -keystore "$KS" -storepass "$KS_PASS" -keypass "$KEY_PASS" \
    -alias "$KEY_ALIAS" -keyalg RSA -keysize 2048 -validity 10000 \
    -dname "CN=AMDL,O=AMDL,C=US" >/dev/null 2>&1
fi

"$APKSIGNER" sign \
  --ks "$KS" --ks-pass "pass:$KS_PASS" --key-pass "pass:$KEY_PASS" \
  --ks-key-alias "$KEY_ALIAS" \
  --min-sdk-version "$MIN_SDK" \
  --v4-signing-enabled true \
  --out "$out/app-release.apk" \
  "$build/stage/app-aligned.apk"

echo "==> verify"
"$APKSIGNER" verify --print-certs "$out/app-release.apk" | head -5

# Guard against the "new key every build" regression: compare with the recorded
# fingerprint and shout when the identity changed.
ACTUAL_SHA=$("$APKSIGNER" verify --print-certs "$out/app-release.apk" 2>/dev/null \
  | sed -n 's/^Signer #1 certificate SHA-256 digest: //p' | head -1)
if [[ -z "$ACTUAL_SHA" ]]; then
  ACTUAL_SHA=$("$APKSIGNER" verify --print-certs "$out/app-release.apk" 2>/dev/null \
    | sed -n 's/.*certificate SHA-256 digest: //p' | head -1)
fi
if [[ -f "$FINGERPRINT_FILE" ]]; then
  # Normalise "AA:BB:cc" / "aabbcc" before comparing: apksigner prints lowercase
  # without separators, the recorded file uses the keytool style.
  EXPECTED_SHA=$(tr -d '[:space:]:' < "$FINGERPRINT_FILE" | tr '[:upper:]' '[:lower:]')
  NORMAL_ACTUAL=$(printf '%s' "$ACTUAL_SHA" | tr -d '[:space:]:' | tr '[:upper:]' '[:lower:]')
  if [[ -n "$NORMAL_ACTUAL" && -n "$EXPECTED_SHA" && "$NORMAL_ACTUAL" != "$EXPECTED_SHA" ]]; then
    echo "!! signing certificate changed" >&2
    echo "!!   expected: $EXPECTED_SHA" >&2
    echo "!!   actual:   $ACTUAL_SHA" >&2
    echo "!! installed builds must be uninstalled once; update $FINGERPRINT_FILE if this is intended" >&2
  fi
else
  mkdir -p "$(dirname "$FINGERPRINT_FILE")"
  printf '%s\n' "$ACTUAL_SHA" > "$FINGERPRINT_FILE"
fi

ls -lh "$out/app-release.apk"
echo "==> done: $out/app-release.apk"
