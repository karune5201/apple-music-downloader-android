#!/usr/bin/env bash
# 把本项目的 Go 侧改动叠加到（刚下载的）上游源码上。
#
#   ./apply.sh <上游源码根目录>
#
# 两种叠加方式：
#   files/   整文件覆盖——本项目**新增**的文件（internal/logx、internal/i18n、
#            各种新测试）。上游没有同名文件，直接复制，不会冲突。
#   patches/ 补丁——对上游**既有**文件的改动。用 patch(1) 打，fuzz 收紧到 0，
#            上游改了同一段就立刻失败，而不是硬塞进去。
#
# 任何一步失败都返回非零，调用方应当中止并不改动工作区。
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
target="${1:-}"
if [ -z "$target" ]; then
  echo "用法: $0 <上游源码根目录>" >&2
  exit 2
fi
if [ ! -f "$target/go.mod" ]; then
  echo "目标不像 Go 模块（缺 go.mod）: $target" >&2
  exit 2
fi
target="$(cd "$target" && pwd)"

echo "==> overlay -> $target"

# 2) 既有文件的补丁：先全部预检，全通过再真打（预检不过就退出，此时未改动任何文件）。
#    优先用 git apply（上下文严格匹配、不写 .orig），没有 git 时退回 patch(1)。
if [ -d "$here/patches" ] && compgen -G "$here/patches/*.patch" > /dev/null; then
  if command -v git > /dev/null 2>&1; then
    apply_check() { git apply --check -p1 --unsafe-paths --directory=. "$1"; }
    apply_real()  { git apply -p1 --unsafe-paths --directory=. "$1"; }
  elif command -v patch > /dev/null 2>&1; then
    apply_check() { patch -p1 --dry-run --forward --fuzz=0 -s < "$1"; }
    apply_real()  { patch -p1 --forward --fuzz=0 -s < "$1"; }
  else
    echo "需要 git 或 patch 才能叠加补丁，两者都没找到。" >&2
    exit 1
  fi

  for p in "$here"/patches/*.patch; do
    if ! ( cd "$target" && apply_check "$p" ); then
      echo "补丁预检失败: $(basename "$p")" >&2
      echo "上游多半改了同一段代码，需要人工重新生成这个补丁（见 README）。" >&2
      exit 1
    fi
  done
  for p in "$here"/patches/*.patch; do
    ( cd "$target" && apply_real "$p" )
    echo "    已打补丁: $(basename "$p")"
  done
fi

# 3) 新增文件：最后铺。
#    顺序刻意如此：补丁预检失败时一个字节都还没动，调用方可以放心重试。
if [ -d "$here/files" ]; then
  count=0
  while IFS= read -r -d '' src; do
    rel="${src#"$here/files/"}"
    mkdir -p "$target/$(dirname "$rel")"
    cp -a "$src" "$target/$rel"
    count=$((count + 1))
  done < <(find "$here/files" -type f -print0)
  echo "    新增/覆盖文件: $count"
fi

echo "==> overlay 完成"
