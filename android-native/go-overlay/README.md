# go-overlay —— 本项目对 Go 核心的全部改动

`update-upstream.sh` 每次都会**整棵 Go 源码换成上游**，只保留 `android-native/`。
所以本项目对 Go 侧的所有改动都必须住在这里，才能在上游更新后自动叠加回来。

## 目录

| 路径 | 用途 |
|---|---|
| `files/` | 上游**没有**的文件（新包、新测试），直接铺进源码树 |
| `patches/` | 对上游**既有**文件的改动，unified diff，`patch -p1 --fuzz=0` 打 |
| `apply.sh` | 叠加入口：`./apply.sh <上游源码根目录>`（优先 `git apply`，没有 git 时退回 `patch`） |
| `gen.sh` | 重新生成 `files/` 与 `patches/`：`./gen.sh <纯净上游目录>` |

当前规模：**16 个新增文件 + 29 个补丁**，覆盖 204 条三语文案（app 166 / net 18 / media 11 / model 9）。

`apply.sh` 先对所有补丁做 `--dry-run` 预检，全部通过才真正落盘；
`update-upstream.sh` 在**临时目录**里就完成“叠加 + 试编译”，失败即中止，
工作区一个字节都不会动。

## 当前改动清单

### 新增包

| 包 | 作用 |
|---|---|
| `internal/logx` | 全部终端输出的唯一出口。级别靠行首标记（▸ ✔ ⚠ ✖ ·），`--verbose` 控制 `Debug`，`SetBeforeLine` 让进度条在写日志前清掉自己；`Bytes/Rate/Duration/Since/KV` 统一数字格式 |
| `internal/i18n` | 中日英三语目录。键为 `<区>.<事件>`，区取 `app`/`model`/`media`/`net`；`register(zh, ja, en, keys...)` 登记，`T(key, args...)` 取文案，缺键原样返回键名 |

### 行为修复（不只是日志）

| 位置 | 问题 | 修法 |
|---|---|---|
| `internal/fairplay-rip` | `grafov/m3u8` 的 `Segments` 是 1024 槽**环形缓冲**，代码拿 `len()` 当分片数，于是每首都以 `playlist/fragment count mismatch: playlist=1024 fragments=7` 失败 | 改用 `Count()` 取真实分片，过滤 nil 并按 `SeqId` 排序（滑动播放列表也正确） |
| `internal/fairplay-rip` | 媒体流空闲超时会掐掉本来能成的请求 | 删除全部网络超时；只在**连接被掐断**时按 `Range` 续传（最多 5 次） |
| `internal/wrapper` | lite-server 首次握手抖动（`upstream error … EOF`）就丢掉整首 | 3 次重试 + 500 ms 递增退避，只对瞬时错误重试；同样不设超时 |
| `internal/app` | 汇总行是人读的多语言文本，App 无法正则解析 | 人类可读汇总之后固定再打一行机器数据 `##ok=… total=… warn=… err=…`（永不翻译） |
| `internal/app` `internal/model` | 等待 stdin 的提示被翻译后 App 认不出来 | 提示前打印语言无关的 `##prompt` 行 |
| `internal/config` | 无法选择语言 | 新增 `general.ui-language`（`zh`/`ja`/`en`，默认 `zh`），`--ui-language` 与 `AMDL_LANG` 可临时覆盖 |

### 日志改造范围

`internal/app`、`internal/model`、`internal/fairplay-rip`、`internal/wrapper`、
`internal/media/{alacfix,lyrics}`、`internal/widevine-rip` 的用户可见输出全部走 `logx` +
`i18n`。**不翻译**：文件名、路径、URL、codec/格式名（ALAC/AAC/Atmos/M4U8/MP4/ffmpeg）、
命令行参数、`Range`、字节偏移，以及 Go 返回的 `error` 文本（以 `%v` 参数传入）。

## 加一条新文案

```go
// internal/i18n/msg_<区>.go
register("已保存 %s", "%s を保存しました", "saved %s", "app.saved")
```

```go
logx.OKT("app.saved", path)   // 输出：✔ 已保存 /path/x.m4a
```

测试会检查三语齐全、`%` 占位符数量与顺序一致、键名合规、无重复键。

## 上游更新后怎么验证

```bash
./android-native/update-upstream.sh main --dry-run   # 下载 + 叠加 + 试编译，不动工作区
./android-native/update-upstream.sh main             # 真更新 + 出包
```

补丁打不上（上游改了同一段代码）时 `apply.sh` 直接失败并指出是哪个补丁，
这时需要在新上游上人工重做那处改动，再重新生成补丁：

```bash
cd /tmp && curl -fsSL -o up.tar.gz https://codeload.github.com/zhaarey/apple-music-downloader/tar.gz/refs/tags/<tag>
mkdir -p base && tar -xzf up.tar.gz -C base --strip-components=1
diff -u base/internal/app/run.go  <repo>/internal/app/run.go > android-native/go-overlay/patches/xx-run.patch
```
