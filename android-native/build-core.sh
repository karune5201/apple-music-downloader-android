#!/usr/bin/env bash
# Builds the Android arm64 download core from the upstream Go sources.
#
# The core is the original amdl CLI, cross-compiled for Android. CGO is
# disabled on purpose: android_dns.go installs a resolver that does not depend
# on /etc/resolv.conf, which is what a cgo build would otherwise provide.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/.." && pwd)"
out="$here/app/src/main/jniLibs/arm64-v8a/libamdl.so"

mkdir -p "$(dirname "$out")"
cd "$repo"

echo "==> go build (android/arm64, CGO off)"
GOOS=android GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags='-s -w' -o "$out" .

file "$out"
ls -lh "$out"
echo "==> core written to $out"
