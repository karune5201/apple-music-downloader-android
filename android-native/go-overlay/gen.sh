#!/usr/bin/env bash
# 从“当前工作区 vs 纯净上游”重新生成 overlay（files/ 与 patches/）。
#
#   ./gen.sh /tmp/base/src        # 基线目录必须是同一上游版本的解包结果
#
# 用途：改完 Go 代码后刷新 overlay；或上游换版本、补丁冲突时在新版本上重做改动后
# 重新生成。生成完建议按 README 的“上游更新后怎么验证”跑一次 --dry-run。
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
base="${1:-}"
if [ -z "$base" ] || [ ! -f "$base/go.mod" ]; then
  echo "用法: $0 <纯净上游源码目录>" >&2
  exit 2
fi
base="$(cd "$base" && pwd)"

rm -rf "$here/files" "$here/patches"
mkdir -p "$here/files" "$here/patches"

new=0
pat=0
n=0
while IFS= read -r -d '' src; do
  rel="${src#"$repo/"}"
  case "$rel" in
    android-native/*) continue ;;
    android_dns.go|android_dns_test.go) continue ;;   # 由 update-upstream.sh 的 go-patch 负责
    *.orig|*.rej) continue ;;
  esac
  if [ ! -e "$base/$rel" ]; then
    mkdir -p "$here/files/$(dirname "$rel")"
    cp -a "$src" "$here/files/$rel"
    new=$((new + 1))
  elif ! cmp -s "$src" "$base/$rel"; then
    n=$((n + 1))
    name="$(printf '%02d-%s' "$n" "$(printf '%s' "$rel" | tr '/' '-')")"
    diff -u --label "a/$rel" --label "b/$rel" "$base/$rel" "$src" > "$here/patches/$name.patch" || true
    pat=$((pat + 1))
  fi
done < <(find "$repo" -name '*.go' -not -path '*/android-native/*' -print0)

# 非 .go 的少量文件（配置模板等）也要跟着 overlay 走
for rel in config.yaml.example .gitignore .github/workflows/release.yml; do
  if [ -f "$repo/$rel" ] && ! cmp -s "$repo/$rel" "$base/$rel"; then
    mkdir -p "$here/files/$(dirname "$rel")"
    cp -a "$repo/$rel" "$here/files/$rel"
    new=$((new + 1))
  fi
done

# 上游有、本仓库要删掉的文件
cat > "$here/remove.txt" <<'REMOVE'
# 叠加 overlay 时要删掉的上游文件（每行一个相对路径，空行与 # 注释忽略）。
# 上游自带的 GitHub Actions 对本仓库无用：
#   ci.yml     每次推 main 都跑上游的 CLI 测试，本仓库必然失败
#   docker.yml 打 tag 时往 GHCR 推一个容器镜像，与本项目无关
.github/workflows/ci.yml
.github/workflows/docker.yml
# 上游 CLI 的容器打包，Android 版用不到
Dockerfile
compose.yaml
docker-entrypoint.sh
.dockerignore
REMOVE

echo "新增文件: $new   补丁: $pat"
