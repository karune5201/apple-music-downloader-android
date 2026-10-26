#!/usr/bin/env bash
# 编译 ffmpeg 兼容层（go-shim）为 Android arm64 可执行文件。
#
# 上游转码时给 flac/mp3 传 -disposition:v attached_pic，这是 ffmpeg 4.3+ 的写法，
# 内置的 3.3.2 不认识，会直接退出 1。兼容层负责改写参数、补回封面。
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
out="$here/app/src/main/jniLibs/arm64-v8a/libffmpegshim.so"

GOOS=android GOARCH=arm64 CGO_ENABLED=0 \
  go build -C "$here/go-shim" -trimpath -ldflags='-s -w' -o "$out" .

chmod 755 "$out"
echo "==> $out"
ls -lh "$out"
