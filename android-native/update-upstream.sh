#!/usr/bin/env bash
# 拉取上游最新源码，保留本项目的 Android 部分，然后重新编译核心与 APK。
#
# 用法:
#   ./android-native/update-upstream.sh              # 跟随 main 分支
#   ./android-native/update-upstream.sh v0.1.5       # 指定 tag / 分支
#   ./android-native/update-upstream.sh main --dry-run   # 只下载并试编译，不动工作区
#   ./android-native/update-upstream.sh main --no-build  # 换源码但不编译
#
# 这个仓库没有 .git（最初是 tarball 解出来的），所以更新靠“重新下载 + 保留覆盖层”：
#   · android-native/            —— 本项目的 Android 外壳，更新时原样保留
#   · android-native/go-patch/   —— 加在上游根目录的 Go 文件（DNS 引导），每次更新重新贴回去
#   · android-native/go-overlay/ —— 本项目对 Go 核心的**全部改动**（新增包 + 补丁），
#                                   下载上游后自动叠加；叠加或试编译失败就中止，工作区不动
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/.." && pwd)"
patch_dir="$here/go-patch"

ref="${1:-main}"
shift || true
dry_run=0
do_build=1
for arg in "$@"; do
  case "$arg" in
    --dry-run) dry_run=1 ;;
    --no-build) do_build=0 ;;
    -*) echo "未知参数: $arg" >&2; exit 2 ;;
  esac
done

owner="zhaarey"
project="apple-music-downloader"

tmp="$(mktemp -d)"
cleanup() { rm -rf "$tmp"; }
trap cleanup EXIT

fetch() {
  local url="$1" dest="$2"
  echo "==> 下载 $url"
  curl -fsSL --retry 3 --connect-timeout 20 -o "$dest" "$url"
}

tarball="$tmp/upstream.tar.gz"
got=0
for candidate in "refs/heads/$ref" "refs/tags/$ref"; do
  if fetch "https://codeload.github.com/$owner/$project/tar.gz/$candidate" "$tarball" 2>/dev/null; then
    got=1
    echo "==> 命中 $candidate"
    break
  fi
done
if [ "$got" -eq 0 ]; then
  echo "无法下载 $ref（分支和 tag 都试过了）。检查网络或名称是否正确。" >&2
  exit 1
fi

echo "==> 解包"
mkdir -p "$tmp/src"
tar -xzf "$tarball" -C "$tmp/src" --strip-components=1

upstream_version="$(cat "$tmp/src/VERSION" 2>/dev/null || echo unknown)"
echo "==> 上游版本: $upstream_version"

if [ ! -d "$tmp/src/cmd" ] || [ ! -f "$tmp/src/go.mod" ]; then
  echo "解包结果不像上游源码（缺少 cmd/ 或 go.mod）。" >&2
  exit 1
fi

echo "==> 贴回本项目补丁: $(ls "$patch_dir")"
cp "$patch_dir"/*.go "$tmp/src/"

# Go 侧改动（日志包、多语言、分片修复、去超时…）在 overlay 里；先叠加再试编译，
# 失败就中止，工作区一个字节都不动。
echo "==> 叠加 go-overlay"
"$here/go-overlay/apply.sh" "$tmp/src"

echo "==> 试编译核心（android/arm64, CGO off）"
if ! ( cd "$tmp/src" && GOOS=android GOARCH=arm64 CGO_ENABLED=0 \
        go build -trimpath -ldflags='-s -w' -o "$tmp/libamdl.so" . ); then
  echo "编译失败：上游可能改了依赖或需要新的构建步骤。工作区没有改动。" >&2
  exit 1
fi
echo "==> 试编译通过: $(ls -lh "$tmp/libamdl.so" | awk '{print $5}')"

if [ "$dry_run" -eq 1 ]; then
  echo "==> --dry-run：到此为止，$repo 未做任何修改。"
  exit 0
fi

echo
echo "即将用上游 $upstream_version 覆盖 $repo"
echo "保留目录: android-native/（含 go-patch/）"
read -r -p "继续? [y/N] " answer
case "$answer" in
  y|Y) ;;
  *) echo "已取消。"; exit 1 ;;
esac

echo "==> 把 android-native/ 暂存到临时目录"
mv "$repo/android-native" "$tmp/android-native"

echo "==> 清理旧的上游文件（保留 android-native/）"
shopt -s dotglob nullglob
for entry in "$repo"/*; do
  case "$(basename "$entry")" in
    android-native) continue ;;
  esac
  rm -rf "$entry"
done
shopt -u dotglob nullglob

echo "==> 铺开新源码"
cp -a "$tmp/src/." "$repo/"

echo "==> 还原 android-native/"
mv "$tmp/android-native" "$repo/android-native"

echo "$ref ($upstream_version)" > "$repo/android-native/upstream-ref.txt"

# 配置模板就是上游的 config.example.yaml：刷新它，App 才能自动识别新增配置项
if [ -f "$repo/config.yaml.example" ]; then
  cp "$repo/config.yaml.example" "$repo/android-native/app/src/main/assets/config.example.yaml"
  keys="$(grep -cE '^[[:space:]]+[a-z-]+:' "$repo/config.yaml.example" || true)"
  echo "==> 已刷新配置模板（$keys 个配置键）"
fi

if [ "$do_build" -eq 0 ]; then
  echo "==> --no-build：源码已更新，未编译。"
  exit 0
fi

echo "==> 编译核心"
"$repo/android-native/build-core.sh"

echo "==> 编译 APK"
"$repo/android-native/build-apk.sh"

echo
echo "完成。上游: $ref ($upstream_version)"
echo "产物: $repo/android-native/out/app-release.apk"
echo
echo "建议再核对这四处（上游若改了输出格式，App 会解析不到）："
echo "  1. 机器行 ##ok=ok total warn err          —— AmdlRunner.parseMachine（不翻译）"
echo "  2. 等待输入前的 ##prompt 行               —— AmdlRunner.isPromptMarker"
echo "  3. --select 的曲目表格列数                —— MainActivity.parseTrackTable"
echo "  4. config.yaml 的键名（paths/media/metadata）—— ConfigWriter"
echo
echo "本次叠加的 Go 改动清单: $here/go-overlay/README.md"
