# Apple Music 下载器 · 原生 Android 版

把 [zhaarey/apple-music-downloader](https://github.com/zhaarey/apple-music-downloader)（Go 命令行工具）
包成一个**原生 Android 应用**：装机即用，不需要 Termux、不需要 adb、不需要在手机上敲命令。

> 仅供你有权访问的内容使用，请遵守 Apple 服务条款与当地法律。本项目不包含任何绕过订阅的机制：
> 解码能力来自你自己运行的 wrapper-lite，账号与订阅由你自己提供。

---

## 1. 这个 App 做了什么

| 层 | 实现 |
|---|---|
| 界面 | 原生 Java，**纯代码构建视图**（无 XML 布局、无 AndroidX、无 Material 库），Material 3 视觉由 `Ui.java` 按 MD3 令牌手工实现：单页（服务 / 音质 / 链接 / 目录 / 日志） |
| 后台 | `DownloadService`（前台服务 + 通知），息屏、退到后台也能继续下载，可随时停止 |
| 下载核心 | 上游 Go CLI，交叉编译为 `arm64-v8a/libamdl.so`，随 APK 安装到 `nativeLibraryDir` 后作为子进程运行 |
| 输出 | 先落到 App 专属目录，成功后通过 MediaStore 导入系统音乐库 `Music/AppleMusic/…` |

流水线：

```
用户粘贴链接
   ↓
MainActivity 写 config.yaml（音质/保存路径/代理/DNS…）
   ↓
DownloadService.startForeground()  →  ProcessBuilder(nativeLibraryDir/libamdl.so …)
   ↓                                          ↓
通知栏进度  ←  逐行读取 stdout  ←  Go 核心：Apple API → wrapper-lite(/m3u8 /key /lyrics) → 分片下载解密 → 打标签
   ↓
完成后按需 Exporter → MediaStore（Music/AppleMusic/…）
```

### 为什么要有 `android_dns.go`

上游官方 Android 构建依赖 cgo（Bionic `getaddrinfo`），因为 Android 没有 `/etc/resolv.conf`，
纯 Go 解析器会直接失败（见 Go issue #10714）。本仓库**不引入 NDK**，而是新增
`android_dns.go`：启动时按 `AMDL_DNS` → `getprop net.dns1..4` → 内置公共 DNS 的顺序
探测可用服务器，并安装一个自带 Dial 的 `net.Resolver`。

已在本仓库验证：在把 `/etc/resolv.conf` 置空的环境里，`music.apple.com` 仍能被解析
（`go test -run TestDefaultResolverUsesConfiguredServer`，见下文“已验证项”）。

---

## 2. 前置条件（必须）

1. **wrapper-lite 解密服务**：上游要求先跑 [WorldObservationLog/wrapper](https://github.com/WorldObservationLog/wrapper/tree/lite)，
   提供 `/m3u8`、`/key`、`/lyrics`、`/license`、`/status` 等 HTTP 接口。
   - 同一局域网的另一台电脑/服务器跑最省事，App 里填 `http://<局域网IP>:12340`；
   - 想在手机本机跑，需要 Termux + `wrapper-lite-qemu`，填 `http://127.0.0.1:12340`（App 不会内置 wrapper）。
2. **Apple Music 订阅**（ALAC / Atmos / AAC / MV 均需要）。
3. **arm64 设备**（`arm64-v8a`）。32 位与 x86 模拟器不在支持范围。

---

## 3. 构建

### 3.1 先编译下载核心

```bash
cd android-native
./build-core.sh          # go build → app/src/main/jniLibs/arm64-v8a/libamdl.so
```

需要 Go 1.23+。产物是 ARM64 PIE，无任何 `NEEDED` 动态依赖（`readelf -d` 可验），
因此不依赖手机上的 libc 版本。

### 3.2 打包 APK

**方式 A：Android Studio / Gradle（推荐日常开发）**

```bash
cd android-native
./gradlew :app:assembleDebug     # 或直接用 Android Studio 打开该目录
```

Gradle 8.5.2 + AGP 8.5.2，`namespace com.amdl.nativeapp`，`minSdk 26`，`targetSdk 34`，
`packaging.jniLibs.useLegacyPackaging = true` 保证核心被解压到 `nativeLibraryDir`（可用 `exec`）。

**方式 B：无 Gradle、纯 SDK 工具链（本仓库实际采用，可在无网络/无 x86 环境下跑）**

```bash
./build-apk.sh    # aapt2 → javac → d8 → zipalign → apksigner
# 产物：android-native/out/app-release.apk（正式版 1.1.0，签名固定）
```

脚本把“注入 `package`”“Java 8 目标”“dex/so 不压缩并页对齐”“固定签名（见 3.4）”这些细节都处理好了；
可用环境变量覆盖工具路径：`ANDROID_JAR`、`NATIVE_BT`（aapt2/zipalign，需与本机架构一致）、
`JAVA_BT`（d8/apksigner，Java 程序任意架构均可）。

> 注意：aapt2 2.19（Debian 打包的 arm64 版本）读不了 API 30+ 的 `resources.arsc`，
> 所以脚本默认优先用 **API 29** 平台 jar 链接资源；用 Gradle 构建时则随便用新版 SDK。

---

## 3.3 上游更新了怎么办

这个仓库是由上游源码 + `android-native/` 覆盖层组成的，两个部分各自独立：

| 部分 | 内容 | 更新时 |
|---|---|---|
| 上游源码 | `cmd/`、`internal/`、`go.mod`、`config.yaml.example` … | 整个替换成新版 |
| 本项目覆盖层 | `android-native/`（Java 界面、构建脚本） | 原样保留 |
| 本项目 Go 补丁 | `android-native/go-patch/*.go`（DNS 引导） | 每次重新贴到上游根目录 |
| **本项目 Go 改动** | `android-native/go-overlay/`（日志包、多语言、分片修复、去超时…） | 每次自动叠加；叠加失败就中止 |

### 一条命令更新

```bash
# 跟随 main 分支
./android-native/update-upstream.sh

# 指定 tag / 分支
./android-native/update-upstream.sh v0.1.5

# 只验证能不能编过，不动工作区（推荐先跑这个）
./android-native/update-upstream.sh main --dry-run

# 只换源码，稍后自己编译
./android-native/update-upstream.sh main --no-build
```

脚本做的事：

1. 从 `codeload.github.com` 下载指定 ref 的源码包（分支和 tag 都会试）；
2. 解包并打印上游版本（`VERSION`）；
3. 把 `android-native/go-patch/*.go` 贴回上游根目录；
4. 叠加 `android-native/go-overlay/`（本项目对 Go 核心的全部改动：新增 `internal/logx`、`internal/i18n`，以及对 `internal/app|model|fairplay-rip|wrapper|config|media|widevine-rip` 的补丁）；
5. **先试编译** —— 叠加或编译失败就直接退出，工作区一个文件都不动；
6. 确认后：把 `android-native/` 暂存到临时目录 → 清掉旧的上游文件 → 铺开新源码（已含 overlay）→ 还原 `android-native/`；
7. 依次跑 `build-core.sh` 和 `build-apk.sh`，产物在 `android-native/out/app-release.apk`。

### 手动更新（不想用脚本）

```bash
# 1. 备份覆盖层
mv android-native /tmp/android-native-keep

# 2. 下载并解包新版，铺到仓库根目录
curl -L -o /tmp/up.tar.gz https://codeload.github.com/zhaarey/apple-music-downloader/tar.gz/refs/heads/main
mkdir -p /tmp/up && tar -xzf /tmp/up.tar.gz -C /tmp/up --strip-components=1
rm -rf cmd internal dist *.go go.mod go.sum VERSION   # 只删上游的东西
cp -a /tmp/up/. .

# 3. 还原覆盖层与补丁（Go 改动全在 go-overlay 里，必须叠加，否则日志/多语言/分片修复全丢）
mv /tmp/android-native-keep android-native
cp android-native/go-patch/*.go .
./android-native/go-overlay/apply.sh .

# 4. 编译
./android-native/build-core.sh && ./android-native/build-apk.sh
```

### 更新后要顺手检查的四处

上游改动不保证向后兼容，这四处是 App 与 CLI 之间的“接口”，一变就会静默失效：

| 位置 | 依赖什么 | 失效表现 |
|---|---|---|
| `AmdlRunner.parseMachine` | 汇总后的机器行 `##ok=13 total=14 warn=1 err=0`（**永不翻译**） | 计数显示不出来（会退回旧的英文正则） |
| `AmdlRunner.isPromptMarker` | 等输入前的 `##prompt` 行 | 选曲/选音质条弹不出来，只能靠英文后缀兜底 |
| `MainActivity.parseTrackTable` | `--select` 的 4 列曲目表格 | 选曲条弹不出来，或曲目列表为空 |
| `ConfigWriter` | `config.yaml` 的 `general/media/paths/metadata` 键名，以及 `general.ui-language` | 配置不生效（比如路径回落默认值、语言不跟随） |

补丁打不上时（上游改了同一段代码），`apply.sh` 会指出是哪个补丁并退出。重新生成：

```bash
cd /tmp && curl -fsSL -o up.tar.gz https://codeload.github.com/zhaarey/apple-music-downloader/tar.gz/refs/heads/main
mkdir -p base && tar -xzf up.tar.gz -C base --strip-components=1
cd <repo> && ./android-native/go-overlay/gen.sh /tmp/base   # 重新导出 files/ 与 patches/
```

另外 `--atmos`、`--aac`、`--save-m3u8-playlist`、`--select`、`--debug` 这几个 CLI 参数如果被改名，
也要跟着改 `MainActivity.startDownload`。

### 版本号

版本号只有一个源：`VERSION_NAME`。

```bash
VERSION_NAME=1.0.1 ./android-native/build-apk.sh     # versionCode 自动算成 10001
```

规则是 `major*10000 + minor*100 + patch`（1.0.0 → 10000、1.2.3 → 10203），
所以**改版本名就等于改版本号**，不会出现"忘了加 versionCode 导致装不上"。

发版也可以直接交给 CI：推一个 `v*` tag，GitHub Actions 会编译、校验签名指纹、
打出 APK 并发到 Release（见 `.github/workflows/release.yml`）：

```bash
git tag v1.0.1 && git push origin v1.0.1
```

CI 发布流程（`.github/workflows/release.yml`）已验证可用：

| 触发 | 结果 |
|---|---|
| 推 `v*` tag | 编译 + 校验签名 + 发 Release（APK 与 `.sha256` 两个附件） |
| Actions 页手动运行 | 填版本名；`release` 勾掉就只出 Artifact，不发 Release |

流水线做的事：装 Go/JDK17/SDK → `go build` + `go vet` + `go test` 门禁 →
`fetch-ffmpeg.sh` → `build-core.sh` → `build-shim.sh` → `build-apk.sh` →
**比对签名指纹**（不一致直接失败）→ `zipalign -c` → 上传 Artifact → 发版。
目标平台是 `platforms;android-29` + `build-tools;34.0.0`。

```bash
grep -n "VERSION_CODE" android-native/build-apk.sh   # 手动覆盖时才用
```

版本号同时出现在 `build-apk.sh`（无 Gradle 构建）和 `app/build.gradle`（Gradle 构建）里，两处要一起改。

`android-native/upstream-ref.txt` 记录当前对应的上游 ref 与版本，方便日后回看。

### 3.4 签名与覆盖安装（不卸载就能升级）

升级安装能否成功只看一件事：**新 APK 的签名证书和已安装的一不一样**。
不一样就会 `INSTALL_FAILED_UPDATE_INCOMPATIBLE`，只能先卸载（App 数据一起没）。

历史版本把 debug keystore 放在 `build/`（脚本目录），而 `build-apk.sh` 每次开头都 `rm -rf build`，
于是每次构建都重新生成一把钥匙——这正是“每次更新都得卸载”的原因。

现在：

- 密钥固定在 `android-native/signing/amdl-release.keystore`，由仓库跟踪，不再被 `build/` 清理波及；
- `build-apk.sh` 只在这个文件缺失时才生成新钥匙，并打印醒目警告；
- 构建完成后用 `apksigner verify --print-certs` 取出证书 SHA-256，与 `signing/fingerprint.txt`
  比对，**签名一变就报错**（防止以后又悄悄漂移）；
- `apksigner` 使用 `AMDL_KEYSTORE_PASSWORD` / `AMDL_KEY_PASSWORD` / `AMDL_KEY_ALIAS`
  （默认 `android` / `android` / `androiddebugkey`），不再写死口令；
- 想换自己的发布密钥：`AMDL_KEYSTORE=/path/release.jks AMDL_KEYSTORE_PASSWORD=... ./build-apk.sh`，
  换钥匙那一次必须卸载重装，之后又能连续覆盖升级。

```bash
adb install -r android-native/out/app-release.apk   # 直接升级，无需卸载
```

详见 `android-native/signing/README.md`。

## 4. 安装与使用

```bash
adb install -r android-native/out/app-release.apk
```

1. 打开“配置”，填写 **wrapper-lite 地址**（可点“测试连接”验证）和四条**下载路径**
   （ALAC / 全景声 / AAC / MV），默认是 `/storage/emulated/0/Music/AM/{ALAC,Atmos,AAC,MV}`；
2. 在同一个“配置”面板里点“权限申请”，开启“所有文件访问权限”，否则无法写入 `/storage` 下的真实路径；
3. 选音质（ALAC / 杜比全景声 / AAC）；
4. 添加链接：粘贴多行，或点 **导入列表** 选一个 `.txt`（每行一个链接，会自动去重），
   界面上会显示“共 N 个链接，将按顺序批量下载”；
5. 点“开始下载”。通知栏出现常驻通知，日志实时滚动；“停止”可随时中断。

### 4.1 下载目录与写入方式

App 只往“配置”里填写的路径写，没有隐藏的默认目录、也不导出系统音乐库。路径直接写绝对路径（不做 SAF 转换）：

| 状态 | 行为 |
|---|---|
| **已授权** | 核心直接写进配置里的路径，没有临时目录、没有二次复制；完成后自动触发媒体库扫描，本地播放器立刻可见 |
| 未授权 | 点“开始下载”会提示需要权限，并直接跳到系统授权页；授权后重试即可（Android 10 及以下只需普通存储权限，App 会自动申请） |

**怎么授权**：打开“配置”→ 点“下载路径”下方的“权限申请”，跳到系统设置
（`设置 → 应用 → 特殊权限 → 所有文件访问权限 → AM 下载器`）打开开关，返回 App 即可，
那一行文字会变成“已授权”。Android 10 及以下不需要这个特殊权限，App 会自动申请普通的存储权限。

为什么需要这个权限：核心是 Go 程序，写文件只认真实路径，而 SAF 给的是 `content://` URI。
所以这里不做目录选择器，而是让你直接填绝对路径，再申请“所有文件访问权限”把它交给核心。

四条路径可以随意改（比如 ALAC 放 `Music/AM/`、全景声单独放 `Music/AM-Atmos`），和 am.sh 里的
`paths.alac / atmos / aac / mv` 一一对应；专辑子目录由 CLI 按命名模板创建，同名文件覆盖。

### 4.2 选曲下载（专辑 / 歌单选单曲）

链接卡片里的 **“下载前先选曲目”** 开关打开后，每遇到一个专辑或歌单，CLI 会把曲目表打印到日志区，
底部同时出现一条输入条（不是弹窗——弹窗会挡住上面的曲目编号）：

- 在日志里看编号，然后在输入条里填 `1,3,5-8`，点“确定”；
- 或者点“全部下载”（等价 `all`）、“跳过此专辑”（该条目不下载任何曲目）；
- 输入会自动规范化：全角逗号 `，`、顿号 `、`、分号、空格都能识别，避免 CLI 报 `Invalid option`；
- 支持一次给多个链接，每个专辑/歌单各自询问一次。

实现方式：上游 `--select` 是终端交互，`select: ` 不带换行并阻塞等待 stdin。App 因此：

1. **按字符流**读 stdout（`readLine()` 永远等不到没有换行的提示，会死锁）；
2. **剥离 ANSI 颜色码**——CLI 的表格和提示都带颜色转义，不剥掉既难看又没法解析；
3. 解析表格时处理**长歌名换行**（编号列为空的续行会合并到上一首）；
4. 把选择写回子进程 stdin，选曲逻辑仍走上游原生实现。

若当时界面不在前台（无法询问），会记录“未连接界面，自动选择全部曲目”并继续，不会卡死。

### 4.3 批量下载

上游 CLI 本身就支持一次传多个链接（内部按 `Queue N of M` 逐个处理），所以批量=一次提交多个链接：

- 直接把一堆链接粘进输入框；
- 或点 **导入列表** 读取 `.txt` / 文本文件，自动提取其中的 `http(s)` 链接并去重，与已有链接合并。

### 4.4 查质量

`查质量` 按钮等价于 `amdl --debug <链接>`（am.sh 的菜单 4）：只打印可用音质，不下载、不需要目录。
用来在下载前确认某个地区商店有没有无损或全景声。

### 4.5 配置面板

配置面板的字段来自两个来源：

1. **App 自己的控件** —— 解密服务、四条下载路径、账号、音质、歌词、封面、命名等，
   与上游 `config.yaml` 的键一一对应；
2. **上游新增的配置项** —— APK 里打包了上游的 `config.yaml.example`，启动时解析它，
   凡是 App 还没有专用控件的键，会自动出现在“其它配置项”里（布尔值给开关，其余给输入框），
   并标明它在 YAML 里的位置，例如 `convert.format  # Target format: ...`。
   面板顶部会显示“上游配置共 53 项，以下 N 项还没有专用控件”。

所以上游加了新选项时，**不用等 App 出新版**也能改；下载时这些值会连同 App 的值一起写进
`config.yaml`，未改动的项保持上游默认值（注释也一并保留）。

生成配置的顺序是：上游模板 → App 已知键覆盖 → 用户改过的“其它配置项”覆盖 → 少数必须有固定值的键
（`exit-on-error: true`，以及 ffmpeg 相关的 `convert.*` 开关）。模板缺失或解析异常时，
会自动退回内置的手写版本，保证下载不会因为配置生成失败而中断。

### 4.6 转码（内置 ffmpeg）

APK 里带了一份 ffmpeg 3.3.2（arm64），不需要外挂或 Termux：

- **怎么内置的**：可执行文件命名为 `libffmpeg.so` 放进 `lib/arm64-v8a/`。Android 10+ 只允许执行
  `nativeLibraryDir` 里的文件（`filesDir` 是 noexec），而系统会把这个目录下的 `lib*.so` 从 APK 解出来，
  于是它就有了执行权限——下载核心 `libamdl.so` 用的是同一套机制；
- **怎么调用**：`config.yaml` 的 `convert.ffmpeg-path` 被写成 `<nativeLibraryDir>/libffmpeg.so`，
  上游用 `exec.LookPath` + `exec.Command` 直接调用，等于“本地调用”，不联网；
- **怎么开**：“配置 → 转码 → 下载完成后转码”，目标格式可选 `flac` / `mp3` / `opus` / `wav` / `copy`，
  还可以选“转码后保留原文件”。默认关闭，不影响原有下载流程；
- **兼容层 `libffmpegshim.so`**：上游给 flac/mp3 传的 `-disposition:v attached_pic` 是 ffmpeg 4.3+
  的写法，内置 3.3.2 不认识，会直接 `Conversion failed: exit status 1`。所以 `convert.ffmpeg-path`
  指向自己写的兼容层（`go-shim/main.go`）：它把封面参数改写成 `-vn` 先让转码成功，转完再把封面
  写回文件（flac 用 `METADATA_BLOCK_PICTURE`，mp3 用 ID3v2 `APIC`，纯 Go 实现），
  最后把 ffmpeg 的退出码原样返回。这样无论内置哪个 ffmpeg 版本行为都一致；
- **能力**：含 libmp3lame、libopus、libvorbis，FLAC 为内置编码器；m4a → flac 会保留内嵌封面与标签；
- **体积**：ffmpeg 未压缩 17 MB，APK 里压缩存储约 8 MB（安装后解压）；
- **来源与许可**：`Khang-NT/ffmpeg-binary-android` 的 arm64-v8a full 构建，只依赖 bionic 自带的
  `libc/libm/libdl/libz`。该构建带 `--enable-gpl --enable-nonfree`（含 libfdk-aac），
  自用无碍，但**对外分发前请自行确认许可**；
- **拿不到时怎么办**：`./android-native/fetch-ffmpeg.sh` 可重新下载（`--force` 覆盖），
  脚本会校验是 AArch64 再放进 `jniLibs`。

### 4.7 下载后 ALAC 检测（三层判定）

Apple 侧偶尔会给出编码损坏的 ALAC：能下载、能播放一部分，但解码到中途就报错；更常见的是
**下载被中途掐断**——文件能打开，却在某处突然结束。检测默认开启
（“配置 → 转码 → 下载后检测 ALAC 完整性”），在下载收尾后对**本次下载**产生的 `.m4a` 运行，
三层逐级加强：

| 层 | 做什么 | 能抓住什么 |
|---|---|---|
| 1. 容器结构 | 顺序扫顶层盒子，核对每个盒子声明的长度是否落在文件内 | `mdat` 声明 34 MB 实际只有 5 MB 的**截断文件**；缺 `moov` / 缺 `mdat`；尾部残留半截盒子头 |
| 2. 解码结果 | `ffmpeg -map 0:a:0 -f null -` 整首解码不落盘，看退出码与日志 | `Invalid data found…`、`error while decoding`、`get_bits`、`moov atom not found`、`non monotonically increasing dts` 等 |
| 3. 解码时长 | 对比解码器报告的 `time=` 与容器头的 `Duration:`（容差 1.5 s） | 不报错但**只解出一半就结束**的坏文件——只看报错是发现不了的 |

细节：

- 只查本次下载的文件（修改时间 ≥ 本次开始时间，留 2 分钟余量），大库不会每次全量扫；
- 非 ALAC（AAC / 全景声）自动跳过，不误报；
- 单首最多 3 分钟、单次最多 300 首，超出的下次再查；
- 结果写进实时日志：`[检测] 损坏：xxx.m4a（盒子 mdat 声明 35651584 字节，文件在 3670016 字节前就结束了）`，
  结尾一行汇总，并单独统计“下载不完整/被截断”的条数。

### 4.8 配置项一览

除上面自动生成的“其它配置项”，面板里这些分组是 App 的专用控件，与 am.sh 的 config 菜单一致：

| 分组 | 可改项 |
|---|---|
| 账号与网络 | 商店地区、歌词语言、代理、`media-user-token`、`authorization-token`、DNS、内存缓冲上限 |
| 音质 | ALAC 最高采样率、全景声码率上限、AAC 类型、M3U8 模式、ALAC 自动修复 |
| 歌词 | 类型（`lyrics` / `syllable-lyrics`）、格式（`lrc` / `ttml`）、附加（翻译/音译）、保存 `.lrc`、内嵌到标签 |
| 封面 | 内嵌、尺寸、格式、保存歌手头像 |
| 标签 | 排序标签、iTunes ID、歌单使用原曲信息 |
| 命名 | 专辑/歌单/歌曲/歌手目录模板、露骨/洁净/Master 标记、文件名字符上限 |
| MV | 音轨类型、分辨率上限 |
| 其它 | 按歌手建子目录、保存 M3U8、附加 CLI 参数 |

模板占位符与上游一致：`{AlbumName}`、`{PlaylistName}`、`{SongName}`、`{SongNumer}`、`{ArtistName}`、`{UrlArtistName}` 等。

### 4.9 界面语言（简体中文 / 日本語 / English）

App 界面和核心日志**共用同一套语言设置**，默认简体中文。切换位置：
「配置」面板最上方的「界面语言」。选完立刻生效（界面重建、日志文字保留），并写进 `config.yaml`：

```yaml
general:
  ui-language: zh      # zh / ja / en
```

- 核心侧：`internal/i18n` 里的键值目录（`app.*`、`model.*`、`media.*`、`net.*` 四组），
  所有日志都走 `logx.OKT/WarnT/ErrorT/...`；命令行也支持 `--ui-language ja`
  或环境变量 `AMDL_LANG=en` 临时覆盖；
- App 侧：`Strings.java` 的手写目录（`app.*`、`action.*`、`alac.*`、`account.*` …），
  不用 Android 资源系统，仍然是零 XML、零 AndroidX；
- 键漏翻译时**直接显示键名**而不是空白，`internal/i18n` 的测试还会检查三语是否齐全、
  占位符（`%s`/`%d`）数量与顺序是否一致，加新语种漏改会直接测试失败；
- 技术标识不翻译：ALAC / AAC / Atmos / M4U8 / MP4 / ffmpeg / lite-server / URL / 命令行参数。

给核心加一条新文案（三步）：

```go
// 1) internal/i18n/msg_app.go 里登记
register("已保存 %s", "%s を保存しました", "saved %s", "app.saved")
// 2) 调用处换成带 T 的版本
logx.OKT("app.saved", name)
```

给 App 加一条：在 `Strings.java` 对应分组里补一行 `put("key", new String[]{"中文", "日本語", "English"})`。

#### 关于“机器可读行”

界面语言一变，人读的那行汇总也跟着变，App 就没法用正则解析了。所以核心在人类可读的汇总之后，
固定再打印一行**永不翻译**的机器数据，App 只解析它、从不显示：

```
##ok=13 total=14 warn=1 err=0
```

同理，核心在等待输入前会打印 `##prompt`（语言无关），App 看到它就弹出曲目选择框；
可见的提示文字仍然跟着语言走。

### 4.10 日志格式（以及怎么加详细日志）

核心的所有输出都走 `internal/logx`：一处决定标记、缩进、空行和颜色。App 端收到后按标记上色
（✔ 绿 / ⚠ 琥珀 / ✖ 红 / ▸ 主色加粗 / · 次要灰），所以**不用看颜色也知道级别**：

```
⚠ lite-server /status unreachable: … connection refused

──────────── Queue 1/1 · Album ───────────────────────
▸ Kalafina · Seventh Heaven
  · artist folder: Kalafina
  · album folder: Seventh Heaven

───────── Track 1/14 · 01 · Overture ─────────────────
  · file: 01. Overture
16-bit / 44100 Hz
▸ downloading (attempt 1/2)
  Decrypting...  76%  (5.6/7.3 MB, 2.1 MB/s)
✔ Decrypted 7.3 MB in 3.4s
✔ 01. Overture.m4a · 7.3 MB
```

各级含义：

| 标记 | 含义 | 例子 |
|---|---|---|
| `▸` | 阶段开始（下载 / 解碎片 / 写标签） | `▸ downloading (attempt 1/2)` |
| `✔` | 一步成功 | `✔ Decrypted 7.3 MB in 3.4s` |
| `⚠` | 不影响继续的问题 | `⚠ no lyrics for this song` |
| `✖` | 这一首失败 | `✖ download failed: …` |
| `·` | 明细（缩进两级） | `· file: 01. Overture` |
| `────` | 队列 / 曲目分隔横幅 | `──── Track 3/14 · Love Comes Down ────` |

---

再加详细日志：命令行加 `--verbose`（或设环境变量 `AMDL_VERBOSE=1`），会额外打印媒体播放列表数量与
字节数、分片解密明细、标签写入耗时、续传次数等诊断信息；App 里可在“配置 → 其它 → 附加 CLI 参数”
填 `--verbose`。颜色默认只在终端上启用（`AMDL_COLOR=1/0` 可强制开关），管道输出保持纯文本，
所以日志文件里不会有转义码。

## 5. 与上游的差异 / 已知限制

| 项 | 说明 |
|---|---|
| 交互式功能 | `--select` 已原生支持（日志里出曲目表 + 底部输入条）；`--search`、`--all-album` 仍需终端逐条选择，未暴露（写进“附加参数”会卡住等输入） |
| ffmpeg 后处理 | 已内置 ffmpeg 3.3.2（arm64，解压到 `nativeLibraryDir` 执行），在“配置 → 转码”里开启即可；3.3.2 不认 `-disposition`，由兼容层改写参数并回写封面；动态封面仍关闭（`save-animated` / `emby-animated` 固定 false） |
| 实时进度 | 解密阶段的进度条用回车符重画，App 把 `\r` 帧单列一行显示（`Decrypting... 42% [3.2 MB/12.8 MB]`），其余日志逐行追加 |
| 退出码 | 上游即使全部失败也可能 `exit 0`，因此核心在人类可读的汇总行之后固定打印 `##ok=13 total=14 warn=1 err=0`，App 只解析这行机器数据（永不翻译） |
| 断流续传 | 核心对媒体文件支持 `Range` 续传：连接被掐断时按已下载的字节重新拉取，最多续 5 次；整条轨道失败会在清理半成品后自动重来一次 |
| 无超时 | 核心不设任何网络超时（`/key`、`/status`、`/m3u8`、媒体流、歌词），只靠“连接断开 → 续传”和 lite-server 的 3 次重试兜底；代价是真正挂死的连接需要手动停止 |
| 界面语言 | 「配置」面板最上方的「界面语言」可切 简体中文 / 日本語 / English，写入 `config.yaml` 的 `general.ui-language`，App 文案与核心日志同时改变；`--ui-language` / `AMDL_LANG` 可临时覆盖 |
| 分片计数 | 上游用 `len(m3u8.Segments)` 判断分片数量，而该切片是 m3u8 库的**1024 槽环形缓冲**，于是每首都以 `playlist/fragment count mismatch: playlist=1024 fragments=7` 失败；现按 `Count()` 取真实分片并按 `SeqId` 排序（支持滑动播放列表） |
| lite-server 重试 | `/key`、`/lyrics` 等接口遇到 `upstream error … EOF`、连接重置、5xx 时自动重试 3 次（500 ms/1 s 递增），不再因为第一次握手抖动丢掉整首歌 |
| 前台服务 | Android 14+ 对 `dataSync` 前台服务有时长限制，几千首的队列建议分批提交 |
| 共享存储 | 不使用 MediaStore 集合写入；需要 `MANAGE_EXTERNAL_STORAGE`（手动开启）才能写 `/storage` 下的路径，未授权时不允许开始下载。该权限上架 Google Play 会被拒，自装 APK 无碍 |
| 安全 | wrapper-lite 默认只监听 `127.0.0.1`；跨设备使用要 `--host 0.0.0.0` 并自行加防火墙/鉴权 |

---

## 6. 已验证项（本仓库构建时实测）

| 验证 | 命令 / 方式 | 结果 |
|---|---|---|
| Go 核心交叉编译 | `GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build` | 15 MB ARM64 PIE，解释器 `/system/bin/linker64` |
| 无 glibc 依赖 | `readelf -d dist/amdl_android_arm64` | 无 `NEEDED` 条目 |
| 无 `resolv.conf` 的 DNS | 空 `/etc/resolv.conf` + `go test -run TestDefaultResolverUsesConfiguredServer` | `music.apple.com -> [58.20.209.99 180.130.97.254]` |
| 不可达 DNS 的降级 | `AMDL_DNS=192.0.2.1 amdl --version` | 4 s 超时后继续启动，不挂死 |
| 生成的 config.yaml 合法 | 用同结构 YAML 跑上游 CLI（含全部新字段） | 无 YAML 报错，正常进入队列并请求 lite-server |
| APK 结构 | `unzip -l` / `aapt dump badging` | `com.amdl.nativeapp`，minSdk 26，targetSdk 35，权限正确 |
| dex 内容 | 解析 `classes.dex` | 全部类存在，无 `invoke-custom` / `java.util.stream` / `java.time` 引用 |
| 对齐 | `zipalign -c -p -v 4` | `Verification successful`，`libamdl.so` 位于 65536（页对齐） |
| 签名 | `apksigner verify --print-certs` | V3 签名有效，证书指纹与 `signing/fingerprint.txt` 一致（连编两次 + `rm -rf build` 后仍一致） |
| 分片计数 | `go test ./internal/fairplay-rip -run Parse` | 7 分片播放列表解析出 7 段（而不是 1024），字节区间求和正确，每段都带 key |
| 多语言目录 | `go test ./internal/i18n` | 三语键齐全、占位符数量与顺序一致、键命名合规、未知键回退为键名 |
| 三语日志 | `amdl --ui-language zh/ja/en "<url>"` | 同一条日志分别渲染为中文 / 日本語 / English，机器行 `##ok=…` 三语完全一致 |
| 1.0.0 产物 | `aapt dump badging out/app-release.apk` | `versionCode=100`、`versionName=1.0.0`，签名指纹不变，可直接覆盖安装 |
| 日志渲染 | `go test ./internal/logx` | 级别标记、空行折叠、`--verbose` 开关、横幅与吞吐格式化共 10 项 |
| 断流续传 | `go test ./internal/fairplay-rip` | 服务端中途掐断连接 → 自动 `Range` 续传，字节与原文完全一致；忽略 `Range` 的服务器也能续；预算耗尽时报出偏移量 |
| 删除半成品 | `go test ./internal/media/defrag` | `track contains no samples` 现在是 `ErrEmptyTrack`，调用方可判定后重试 |
| 大屏声明 | `aapt dump badging out/app-release.apk` | `supports-screens: small normal large xlarge`、`uses-feature-not-required: android.hardware.touchscreen`，可装平板 / 桌面模式 / 电视 |
| 权限 | `aapt dump badging` | INTERNET / ACCESS_NETWORK_STATE / FOREGROUND_SERVICE(_DATA_SYNC) / POST_NOTIFICATIONS / MANAGE_EXTERNAL_STORAGE，外加 `WRITE_EXTERNAL_STORAGE`（仅 API ≤29） |

**尚未验证**：真机运行（本环境无 adb/设备）。装机后建议按顺序确认：
`wrapper-lite` 可达 → 单曲下载 → 专辑下载 → 息屏后台 → 导出到音乐库。
若个别 ROM 拒绝从 `nativeLibraryDir` 执行，`AmdlRunner` 已自带 `/system/bin/sh -c exec …` 兜底路径。

---

## 7. 目录结构

```
android-native/
├── update-upstream.sh      # 上游更新：下载新源码 + 保留本覆盖层 + 重编译
├── go-patch/               # 贴到上游根目录的 Go 补丁（DNS 引导）
├── fetch-ffmpeg.sh         # 下载内置 ffmpeg（arm64，放进 jniLibs 以取得执行权）
├── build-shim.sh           # 编译 ffmpeg 兼容层 → libffmpegshim.so
├── go-shim/                # 兼容层源码（改写参数 + 回写封面，纯 Go）
├── build-core.sh                 # Go 核心 → jniLibs/arm64-v8a/libamdl.so
├── build-apk.sh                  # 不依赖 Gradle 的打包脚本（固定签名，见 signing/）
├── signing/                      # 签名密钥 + 指纹记录：换掉它就得卸载重装
├── settings.gradle / build.gradle / gradle.properties
└── app/
    ├── build.gradle
    └── src/main/
        ├── AndroidManifest.xml   # 服务、权限、extractNativeLibs、框架主题
        ├── java/com/amdl/nativeapp/
        │   ├── Ui.java               # Material 3 令牌与组件（颜色/圆角/涟漪/按钮/输入框/对话框）
        │   ├── MainActivity.java     # 纯代码构建的单页 UI、连接自检、崩溃兜底
        │   ├── DownloadService.java  # 前台服务 + 通知 + 结果判定
        │   ├── AmdlRunner.java       # 进程管理、日志流、汇总解析
        │   ├── ConfigWriter.java     # 生成 config.yaml（模板优先 + 手写兜底）
│   ├── ConfigTemplate.java   # 解析上游 config.yaml.example，回填补值
│   ├── ConfigKeys.java       # App 接管的配置键与取值
        │   ├── Exporter.java         # 媒体库扫描（直写后让播放器可见）
        │   ├── CrashLog.java         # 未捕获异常落盘 + 下次启动展示
        │   ├── Prefs.java            # 设置持久化
        │   └── AmdlApp.java          # Application（最早安装崩溃捕获）
        ├── res/drawable/ic_launcher.xml   # 唯一资源：桌面图标
        └── jniLibs/arm64-v8a/libamdl.so
```

上游 Go 侧新增文件：`android_dns.go`、`android_dns_test.go`（其余源码未改）。

---

## 8. 界面（Material 3）

整体按 MD3 信息层级排布：

- **顶部应用栏**：标题在左，右侧为“查质量”“配置”文字按钮；
- **中部可滚动表单**：只有“音质”和“链接”两张卡片；解密服务地址、四条下载路径、账号、命名等都收进右上角“配置”面板；
- **主操作**：单排“开始下载”（filled）+“停止”（outlined）；
- **底部日志**：带“实时日志”标题（右侧为运行状态），空态居中显示提示，下载时逐行刷新、
  解密进度只更新卡片底部一行，点按日志可复制/清空；选曲输入条在日志下方内嵌出现，不遮挡编号。
- **日志高度可调**：日志上方有一根抓手，上下拖动即可改变日志高度（88dp ~ 屏幕 55%），
  高度会被记住；单击抓手在“收起（88dp）”和“上次高度”之间切换。默认 150dp。
  （大屏分栏时日志直接占满右栏高度，不再需要抓手，见 8.1。）

### 8.1 大屏（平板 / 折叠屏 / 桌面窗口）

界面全部在代码里搭，所以大屏适配也在代码里做，没有 `-land` / `-sw600dp` 之类的资源目录：

| 场景 | 判定 | 布局 |
|---|---|---|
| 手机竖屏、被拉窄的多窗口 | 宽 < 600dp 或 高 < 400dp | 单列：应用栏 → 可滚动表单 → 操作 → 日志（抓手调高） |
| 平板、折叠屏展开、手机横屏、桌面窗口 | 宽 ≥ 600dp 且 高 ≥ 400dp | 两栏：左栏配置与操作（360~560dp），右栏状态 + 日志吃满高度，中间 0.5dp 分隔线 |
| 超宽屏 | 内容宽超过 1360dp 时封顶 | 整体居中、两侧留白，不把表单和日志拉成一行长句 |

- 分栏后日志右栏吃满高度，手机版那根拖动抓手自动隐藏；日志字号 11sp → 12sp；
- 选曲输入条跟着日志走，出现在**右栏日志正下方**，紧挨着它要读的曲目表；
- 对话框（配置面板 / 权限 / 日志菜单）宽度封顶 560dp 居中，正文高度封顶 720dp，
  横屏平板上标题与按钮不会被挤出屏幕；
- 尺寸变化由系统重建 Activity 承接：宽窄切换自动重选布局，链接、音质、日志、待输入的选曲
  都在重建后恢复（音质此前只在“开始下载”时落盘，现在 `onStop` 也会记住；
  核心停在选曲提示上时，重建后输入条会自动重新挂出来，不会卡住等输入）；
- `AndroidManifest.xml` 声明了 `resizeableActivity="true"`、
  `supports-screens`（含 `resizeable`）和 `android.hardware.touchscreen`
  （`required="false"`），因此可装到平板、ChromeOS 桌面模式和电视上，窗口可自由缩放、分屏。

App 不含 AndroidX，也没有 Material Components 库，所以 MD3 是手写的：

- **明暗双主题**：MD3 baseline light / dark 两套配色角色，`Ui.apply(dark)` 一处切换；
  默认跟随系统（系统切换夜间模式会自动重建界面），也可在“配置 → 外观”里强制浅色或深色；
  状态栏与导航栏同色，深色下自动换成浅色图标；
- **颜色**：两套角色都集中在 `Ui` 里（light：`primary` `#6750A4`、`surface` `#FEF7FF`；
  dark：`primary` `#D0BCFF`、`surface` `#141218`），涟漪颜色也随主题反转；
- **组件**：filled / tonal / outlined / text 四种按钮（40dp 高、20dp 全圆角、有界涟漪）、
  描边输入框（4dp 圆角，聚焦时描边由 1dp 灰变 2dp 主题色）、12dp 圆角卡片、MD3 开关（着色轨道/滑块）、
  28dp 圆角对话框（标题 + 可滚动内容 + 右对齐文字按钮）、自定义顶栏；
- **排版**：22sp 顶栏标题、16sp 卡片标题、14sp 正文、12sp 辅助说明，字距与行距按 MD3 规范微调；
- **构建**：全部用框架控件 + `GradientDrawable` / `RippleDrawable` 现画，仍然零依赖、零 XML。

---

## 9. 卡住 / 闪退排查

### 9.1 卡住（“下载到第一首就不动了”）

**1.0.0 起核心不再给任何网络请求设超时**（`/key`、`/status`、`/m3u8`、媒体流、歌词全部不设上限），
这是刻意为之：lite-server 的上游是 QEMU，冷启动或换区时首包可能很慢，超时反而会把本来能成的
请求掐死。

代价是“真卡住”时不会自己恢复，排查顺序：

1. 日志停在 `▸ 下载中（第 1/2 次）` 且没有任何 `Decrypting…` → 卡在 `/key` 或清单请求，
   看 lite-server 那边是不是卡住/没起；
2. 有 `Decrypting…` 但进度不动 → Apple CDN 那条连接没数据了。核心只在**连接被断开**时续传
   （会打 `⚠ stream interrupted … resuming`）；连接没断只是不发数据，就会一直等；
3. 日志停在最后一首之后、没有汇总行 → 卡在标签写入或封面下载。

要定位具体位置：加 `--verbose`（App 里“配置 → 其它 → 附加 CLI 参数”填 `--verbose`）。

### 9.2 闪退

界面不再依赖任何 XML 布局/自定义样式，启动路径只剩「框架主题 + 代码建视图」，
可崩溃面已大幅缩小。若仍然闪退，按下面两条拿证据：

1. **应用内自带**：崩溃堆栈会写入
   `Android/data/com.amdl.nativeapp/files/crash.txt`（用数据线在电脑上看，或文件管理器进
   `Android/data`）。下次打开应用时，堆栈会直接显示在日志区，点日志可「复制日志」。

2. **adb（最直接）**：
   ```bash
   adb logcat -c && adb shell am start -n com.amdl.nativeapp/.MainActivity
   adb logcat | grep -E "AndroidRuntime|amdl"
   ```
   看 `FATAL EXCEPTION` 后面第一段 `Caused by` 即可。

已排除的常见坑（本版本构建方式）：dex 内无 `invoke-custom`/`LambdaMetafactory`、
无 `java.util.stream`/`java.time` 引用、无自定义布局与样式资源、
`libamdl.so` 页对齐（65536）且 `extractNativeLibs=true`、v3 签名有效。
