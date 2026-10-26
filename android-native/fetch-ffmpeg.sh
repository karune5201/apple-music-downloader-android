#!/usr/bin/env bash
# 拉取 Android arm64 的 ffmpeg 可执行文件，放进 jniLibs。
#
# 为什么放进 jniLibs：Android 10+ 只允许执行 nativeLibraryDir 里的文件，
# 而 nativeLibraryDir 由系统从 APK 的 lib/<abi>/ 解出来。把可执行文件命名成
# lib*.so 就能被当作“原生库”解包并拿到可执行权限——下载核心 libamdl.so 用
# 的是同一套机制。
#
# 用法:
#   ./fetch-ffmpeg.sh            # 下载并校验
#   ./fetch-ffmpeg.sh --force    # 覆盖已有文件
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
dest="$here/app/src/main/jniLibs/arm64-v8a/libffmpeg.so"

# Khang-NT/ffmpeg-binary-android：面向 Android 的静态构建，只依赖 bionic
# 自带的 libc/libm/libdl/libz，不需要 Termux 的前缀库。
url="https://github.com/Khang-NT/ffmpeg-binary-android/releases/download/2018-07-31/arm64-v8a-full.tar.bz2"

force=0
for arg in "$@"; do
  case "$arg" in
    --force) force=1 ;;
    *) echo "未知参数: $arg" >&2; exit 2 ;;
  esac
done

if [ -f "$dest" ] && [ "$force" -eq 0 ]; then
  echo "已存在 $dest（用 --force 重新下载）"
  exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "==> 下载 ffmpeg (arm64-v8a)"
curl -fL --retry 5 --retry-all-errors -C - --connect-timeout 20 -m 600 \
  -o "$tmp/ffmpeg.tar.bz2" "$url"

echo "==> 解包"
# 精简容器里可能没有 bzip2，用 python 兜底
if command -v bzip2 >/dev/null 2>&1; then
  tar -xjf "$tmp/ffmpeg.tar.bz2" -C "$tmp"
else
  python3 - "$tmp" <<'PY'
import bz2, io, sys, tarfile
tmp = sys.argv[1]
data = bz2.decompress(open(f"{tmp}/ffmpeg.tar.bz2", "rb").read())
tarfile.open(fileobj=io.BytesIO(data)).extractall(tmp)
PY
fi

[ -f "$tmp/ffmpeg" ] || { echo "包里没有 ffmpeg" >&2; exit 1; }

echo "==> 校验 ELF"
if command -v readelf >/dev/null 2>&1; then
  readelf -h "$tmp/ffmpeg" | grep -q 'AArch64' || {
    echo "不是 AArch64 可执行文件" >&2; exit 1; }
fi

mkdir -p "$(dirname "$dest")"
cp "$tmp/ffmpeg" "$dest"
chmod 755 "$dest"

echo "==> 完成: $dest"
ls -lh "$dest"
echo
echo "版本自检（能在本机跑就说明是静态链接的 Android 构建）:"
"$dest" -version 2>/dev/null | head -2 || echo "（无法在本机执行，属正常）"
