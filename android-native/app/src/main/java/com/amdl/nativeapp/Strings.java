package com.amdl.nativeapp;

import java.util.HashMap;
import java.util.Locale;
import java.util.Map;

/**
 * 界面文案目录：简体中文 / 日本語 / English。
 *
 * <p>App 不使用 Android 资源系统（没有 XML、没有 AndroidX），所以文案放在这张
 * 手写的表里：键 → {zh, ja, en}。运行时语言由 {@link #setLang(String)} 决定，
 * 与 Go 核心共用配置项 {@code general.ui-language}，切换后界面和核心日志同时改变。
 *
 * <p>约定：
 * <ul>
 *   <li>未知键返回键本身，漏翻译会直接显示在界面上，不会变成空白；</li>
 *   <li>键按界面分组，用注释隔开；</li>
 *   <li>包名、路径、URL、编码名（ALAC/AAC/Atmos）等技术性文字不进目录。</li>
 * </ul>
 */
public final class Strings {

    /** 支持的语言标记。 */
    public static final String ZH = "zh";
    public static final String JA = "ja";
    public static final String EN = "en";

    private static final int ZH_INDEX = 0;
    private static final int JA_INDEX = 1;
    private static final int EN_INDEX = 2;

    private static final Map<String, String[]> CATALOG =
            new HashMap<String, String[]>(256);

    private static volatile String lang = ZH;
    private static volatile Locale formatLocale = Locale.SIMPLIFIED_CHINESE;

    private Strings() {
    }

    // ------------------------------------------------------------- 公共接口

    /**
     * 规范化语言标记并切换当前语言。
     *
     * <p>与 Go 侧 {@code i18n.Parse} 同样的规则：取 {@code -}/{@code _} 之前的部分，
     * cn→zh、jp→ja、us/gb→en，无法识别时回到 zh。
     */
    public static void setLang(String tag) {
        String normalized = normalise(tag);
        lang = normalized;
        formatLocale = localeOf(normalized);
    }

    /** 当前语言的规范化标记：{@code zh} / {@code ja} / {@code en}。 */
    public static String lang() {
        return lang;
    }

    /**
     * 取一条文案并格式化。
     *
     * <p>始终使用当前语言的 {@link Locale} 做 {@link String#format}，这样英文的
     * 小数点、中文和日文的宽字符标点都能自然呈现；未知键返回键本身。
     */
    public static String get(String key, Object... args) {
        String[] entry = CATALOG.get(key);
        if (entry == null) {
            return key;
        }
        String pattern = entry[indexOf(lang)];
        if (pattern == null || pattern.isEmpty()) {
            return key;
        }
        if (args == null || args.length == 0) {
            return pattern;
        }
        try {
            return String.format(formatLocale, pattern, args);
        } catch (RuntimeException e) {
            // 参数与占位符对不上时宁可露出原文，也不要抛异常打断渲染。
            return pattern;
        }
    }

    /** 某个键在所有支持的语言里是否与给定值相同（用于识别本地化的关键字）。 */
    static boolean matchesAny(String key, String value) {
        String[] entry = CATALOG.get(key);
        if (entry == null || value == null) {
            return false;
        }
        String trimmed = value.trim();
        for (int i = 0; i < entry.length; i++) {
            if (entry[i] != null && entry[i].equalsIgnoreCase(trimmed)) {
                return true;
            }
        }
        return false;
    }

    static String normalise(String tag) {
        String value = tag == null ? "" : tag.trim().toLowerCase(Locale.US);
        if (value.isEmpty()) {
            return ZH;
        }
        int cut = value.length();
        int dash = value.indexOf('-');
        int underscore = value.indexOf('_');
        if (dash >= 0) {
            cut = dash;
        }
        if (underscore >= 0 && underscore < cut) {
            cut = underscore;
        }
        String base = value.substring(0, cut);
        if ("zh".equals(base) || "cn".equals(base) || "chs".equals(base)
                || "cht".equals(base) || "tw".equals(base) || "hk".equals(base)) {
            return ZH;
        }
        if ("ja".equals(base) || "jp".equals(base)) {
            return JA;
        }
        if ("en".equals(base) || "us".equals(base) || "uk".equals(base)
                || "gb".equals(base)) {
            return EN;
        }
        return ZH;
    }

    private static Locale localeOf(String normalized) {
        if (JA.equals(normalized)) {
            return Locale.JAPAN;
        }
        if (EN.equals(normalized)) {
            return Locale.US;
        }
        return Locale.SIMPLIFIED_CHINESE;
    }

    private static int indexOf(String normalized) {
        if (JA.equals(normalized)) {
            return JA_INDEX;
        }
        if (EN.equals(normalized)) {
            return EN_INDEX;
        }
        return ZH_INDEX;
    }

    private static void put(String key, String zh, String ja, String en) {
        CATALOG.put(key, new String[]{zh, ja, en});
    }

    // --------------------------------------------------------------- 文案表

    static {
        // ---------------------------------------------------------- 通用按钮
        put("common.ok", "确定", "決定", "OK");
        put("common.save", "保存", "保存", "Save");
        put("common.cancel", "取消", "キャンセル", "Cancel");
        put("common.close", "关闭", "閉じる", "Close");
        put("common.stop", "停止", "停止", "Stop");

        // ------------------------------------------------------------ 顶栏
        put("app.title", "AM 下载器", "AM ダウンローダー", "AM Downloader");
        put("app.subtitle", "Apple Music · ALAC / 全景声 / AAC",
                "Apple Music · ALAC / Atmos / AAC", "Apple Music · ALAC / Atmos / AAC");
        put("app.inspect", "查质量", "音質を確認", "Check quality");
        put("app.settings", "配置", "設定", "Settings");
        put("app.web", "Web", "Web", "Web");

        // ------------------------------------------------- 网页登录（取 token）
        put("web.title", "网页登录 Apple Music",
                "Apple Music にウェブでログイン", "Sign in to Apple Music on the web");
        put("web.reload", "刷新", "再読み込み", "Reload");
        put("web.saved", "已保存 media-user-token",
                "media-user-token を保存しました", "media-user-token saved");
        put("web.timeout", "还没检测到 media-user-token：请确认已经登录，并停留在 music.apple.com 页面",
                "media-user-token を検出できません：ログイン済みか確認し、music.apple.com のページを開いたままにしてください",
                "media-user-token not detected: make sure you are signed in and stay on the "
                        + "music.apple.com page");
        put("web.error", "页面加载失败：%s", "ページを読み込めませんでした：%s",
                "Page failed to load: %s");
        put("web.fetch", "Web 登录获取", "ウェブで取得", "Fetch via web");
        put("web.unsupported",
                "这台设备的 WebView 不可用：请用电脑上的开发者工具复制 token，粘贴到「配置」里",
                "この端末では WebView を使えません：PC の開発者ツールで token をコピーし、「設定」に貼り付けてください",
                "WebView is unavailable here: copy the token from DevTools on a computer "
                        + "and paste it into Settings");

        // ------------------------------------------------------------ 音质卡片
        put("quality.title", "音质", "音質", "Quality");
        put("quality.alac", "ALAC 无损（最高 192kHz）",
                "ALAC 可逆圧縮（最大 192kHz）", "ALAC lossless (up to 192kHz)");
        put("quality.atmos", "杜比全景声（E-AC-3）",
                "Dolby Atmos（E-AC-3）", "Dolby Atmos (E-AC-3)");
        put("quality.aac", "AAC 有损", "AAC 非可逆圧縮", "AAC lossy");

        // ------------------------------------------------------------ 链接卡片
        put("link.title", "链接", "リンク", "Links");
        put("link.hint", "每行一个，可批量粘贴或导入 TXT",
                "1 行に 1 件。まとめて貼り付け、または TXT を取り込めます",
                "One per line; paste a batch or import a TXT file");
        put("link.field", "下载链接（每行一个）",
                "ダウンロード URL（1 行に 1 件）", "Download links (one per line)");
        put("link.queue.empty", "尚未添加链接", "リンクがまだありません", "No links yet");
        put("link.queue.count", "共 %d 个链接，将按顺序批量下载",
                "リンク %d 件、順番にダウンロードします",
                "%d link(s); downloaded in order");
        put("link.import", "导入列表", "リストを取り込む", "Import list");
        put("link.select.label", "下载前先选曲目",
                "ダウンロード前に曲を選ぶ", "Choose tracks before download");
        put("link.select.on", "已开启：专辑/歌单会先列出曲目供你勾选",
                "オン：アルバム・プレイリストは先に曲一覧を表示します",
                "On: albums and playlists list their tracks first");
        put("link.select.off", "已关闭：整张专辑/整个歌单全部下载",
                "オフ：アルバム・プレイリストを丸ごとダウンロードします",
                "Off: whole albums and playlists are downloaded");

        // ------------------------------------------------------------ 主要操作
        put("action.start", "开始下载", "ダウンロード開始", "Start download");
        put("action.stop", "停止", "停止", "Stop");
        put("status.inspecting", "正在读取音质信息…",
                "音質情報を読み取り中…", "Reading audio quality…");
        put("status.started", "任务已启动 · 写入 %s",
                "タスクを開始しました · 保存先 %s", "Started · writing to %s");
        put("status.ready", "就绪", "待機中", "Ready");
        put("status.running", "下载中…", "ダウンロード中…", "Downloading…");

        // ------------------------------------------------------------- 日志
        put("log.title", "实时日志", "リアルタイムログ", "Live log");
        put("log.placeholder", "暂无日志\n开始下载后这里会实时显示进度",
                "ログはまだありません\nダウンロードを開始するとここに進行状況が表示されます",
                "No log yet\nProgress appears here once a download starts");
        put("log.crash.header", "=== 上次运行时崩溃，堆栈如下（点日志可复制）===",
                "=== 前回の実行はクラッシュしました。スタックは以下のとおりです（ログをタップでコピー）===",
                "=== Previous run crashed; stack trace below (tap the log to copy) ===");
        put("log.crash.footer", "=== 崩溃记录结束 ===",
                "=== クラッシュ記録ここまで ===", "=== End of crash record ===");
        put("log.check.tag", "检测", "検査", "check");
        put("log.binary.missing", "找不到内置下载核心 libamdl.so（本应用仅支持 arm64-v8a 设备）",
                "内蔵のダウンロードコア libamdl.so が見つかりません（このアプリは arm64-v8a のみ対応）",
                "Bundled download core libamdl.so is missing (this app is arm64-v8a only)");
        put("log.menu.title", "日志", "ログ", "Log");
        put("log.menu.copy", "复制日志", "ログをコピー", "Copy log");
        put("log.menu.clear", "清空日志", "ログを消去", "Clear log");
        put("log.menu.clearCrash", "清除崩溃记录", "クラッシュ記録を削除", "Clear crash record");
        put("log.copied", "日志已复制", "ログをコピーしました", "Log copied");
        put("log.crash.cleared", "崩溃记录已清除",
                "クラッシュ記録を削除しました", "Crash record cleared");

        // ------------------------------------------------------------ 选曲输入
        put("prompt.hint.empty",
                "等待选择：查看上方日志里的曲目编号，输入后点确定（如 1,3,5-8 或 all）",
                "選択待ち：上のログで曲番号を確認し、入力して決定してください（例 1,3,5-8 または all）",
                "Waiting for a selection: check the track numbers in the log above, type "
                        + "them and press OK (e.g. 1,3,5-8 or all)");
        put("prompt.hint.count",
                "共 %d 首：在上方日志查看编号，输入后点确定（如 1,3,5-8 或 all）",
                "全 %d 曲：上のログで番号を確認し、入力して決定してください（例 1,3,5-8 または all）",
                "%d track(s): check the numbers in the log above, type them and press OK "
                        + "(e.g. 1,3,5-8 or all)");
        put("prompt.field.hint", "例如 1,3,5-8 或 all",
                "例 1,3,5-8 または all", "e.g. 1,3,5-8 or all");
        put("prompt.all", "全部下载", "すべて", "Download all");
        put("prompt.skip", "跳过此专辑", "このアルバムをスキップ", "Skip this album");
        put("prompt.err.empty", "请输入曲目编号，例如 1,3,5-8；全部下载请点“全部下载”",
                "曲番号を入力してください（例 1,3,5-8）。すべての場合は「すべて」を押してください",
                "Type the track numbers, e.g. 1,3,5-8; press “Download all” to take everything");
        put("prompt.err.format", "格式不对：只能输入数字、逗号与连字符，例如 1,3,5-8",
                "形式が正しくありません：数字・カンマ・ハイフンのみ使えます（例 1,3,5-8）",
                "Bad format: only digits, commas and hyphens, e.g. 1,3,5-8");
        put("prompt.err.dead", "下载进程已结束，选择未能提交",
                "ダウンロードは既に終了しているため、選択を送信できませんでした",
                "The download already ended; the selection was not sent");
        put("prompt.sent.skip", "→ 已跳过该条目", "→ この項目をスキップしました",
                "→ Item skipped");
        put("prompt.sent.all", "→ 已提交选择：%s", "→ 選択を送信しました：%s",
                "→ Selection sent: %s");
        put("select.all", "全部", "すべて", "all");

        // -------------------------------------------------------------- 结果
        put("result.cancelled", "已停止", "停止しました", "Stopped");
        put("result.summary", "完成 %d/%d · 警告 %d · 失败 %d",
                "完了 %d/%d · 警告 %d · 失敗 %d",
                "Completed %d/%d · %d warning(s) · %d failed");
        put("result.done", "完成 %d/%d，失败 %d", "完了 %d/%d、失敗 %d",
                "Completed %d/%d, %d failed");
        put("result.failed", "任务失败：错误 %d 条，退出码 %d",
                "失敗しました：エラー %d 件、終了コード %d",
                "Task failed: %d error(s), exit code %d");
        put("result.empty",
                "没有产出文件（退出码 %d）：请检查 lite-server 是否可达、账号是否有效、链接地区是否与商店地区一致",
                "ファイルが作成されませんでした（終了コード %d）：lite-server に到達できるか、"
                        + "アカウントが有効か、リンクの地域とストアの地域が一致しているかを確認してください",
                "No files were produced (exit code %d): check that lite-server is reachable, "
                        + "the account is valid, and the link region matches the storefront");

        // ------------------------------------------------------------ 设置面板
        put("settings.title", "配置", "設定", "Settings");
        put("settings.language.title", "界面语言", "表示言語", "Language");
        put("settings.language.changed", "界面语言已切换为 %s",
                "表示言語を %s に切り替えました", "Interface language switched to %s");
        put("settings.saved", "配置已保存", "設定を保存しました", "Settings saved");
        put("settings.theme.changed", "已切换主题", "テーマを変更しました", "Theme changed");
        put("lang.name.zh", "简体中文", "简体中文", "简体中文");
        put("lang.name.ja", "日本語", "日本語", "日本語");
        put("lang.name.en", "English", "English", "English");

        put("settings.section.appearance", "外观", "外観", "Appearance");
        put("theme.system", "跟随系统", "システムに従う", "Follow system");
        put("theme.light", "浅色", "ライト", "Light");
        put("theme.dark", "深色", "ダーク", "Dark");

        put("settings.section.server", "解密服务", "復号サービス", "Decryption service");
        put("server.lite.label", "wrapper-lite 地址",
                "wrapper-lite のアドレス", "wrapper-lite address");
        put("server.test", "测试连接", "接続テスト", "Test");
        put("server.testing", "检测中…", "確認中…", "Checking…");
        put("server.connected", "已连接 · %s", "接続しました · %s", "Connected · %s");
        put("server.failed", "连接失败：%s", "接続に失敗しました：%s",
                "Connection failed: %s");
        put("server.empty", "请先填写 wrapper-lite 服务地址",
                "wrapper-lite のアドレスを入力してください",
                "Enter the wrapper-lite address first");

        put("settings.section.paths", "下载路径", "保存先", "Download folders");
        put("dir.alac", "ALAC / 无损", "ALAC / 可逆圧縮", "ALAC / lossless");
        put("dir.atmos", "杜比全景声", "Dolby Atmos", "Dolby Atmos");
        put("perm.granted", "已授权“所有文件访问权限”",
                "「すべてのファイルへのアクセス」は許可済みです",
                "All-files access granted");
        put("perm.denied", "未授权：Android 11+ 需要该权限才能写入上面的路径",
                "未許可：Android 11 以降はこの権限がないと上記のパスに書き込めません",
                "Not granted: Android 11+ needs this to write the folders above");
        put("perm.request", "权限申请", "権限を要求", "Grant access");

        put("settings.section.account", "账号与网络", "アカウントとネットワーク",
                "Account & network");
        put("account.storefront", "商店地区", "ストアの地域", "Storefront");
        put("account.lyricslang", "歌词语言（可留空）", "歌詞の言語（空欄可）",
                "Lyrics language (optional)");
        put("account.proxy", "代理（可留空）", "プロキシ（空欄可）", "Proxy (optional)");
        put("account.mtoken", "media-user-token（电台需要）",
                "media-user-token（ラジオに必要）", "media-user-token (radio only)");
        put("account.atoken", "authorization-token（可留空）",
                "authorization-token（空欄可）", "authorization-token (optional)");
        put("account.dns", "DNS 服务器（留空自动探测）",
                "DNS サーバー（空欄で自動検出）", "DNS server (blank = auto)");
        put("account.maxmem", "内存缓冲上限（MB）", "メモリ上限（MB）", "Memory limit (MB)");

        put("settings.section.quality", "音质", "音質", "Audio quality");
        put("quality.alacmax", "ALAC 最高采样率", "ALAC 最大サンプルレート",
                "ALAC max sample rate");
        put("quality.atmosmax", "全景声最高码率", "Atmos 最大ビットレート",
                "Atmos max bitrate");
        put("quality.aactype", "AAC 类型", "AAC タイプ", "AAC type");
        put("quality.m3u8mode", "M3U8 模式（hires/all）", "M3U8 モード（hires/all）",
                "M3U8 mode (hires/all)");

        put("settings.section.lyrics", "歌词", "歌詞", "Lyrics");
        put("lyrics.type", "歌词类型", "歌詞タイプ", "Lyrics type");
        put("lyrics.format", "歌词格式（lrc/ttml）", "歌詞フォーマット（lrc/ttml）",
                "Lyrics format (lrc/ttml)");
        put("lyrics.extra", "歌词附加（可留空）", "歌詞の追加項目（空欄可）",
                "Lyrics extras (optional)");

        put("settings.section.artwork", "封面与标签", "アートワークとタグ", "Artwork & tags");
        put("artwork.size", "封面尺寸", "アートワークのサイズ", "Artwork size");
        put("artwork.format", "封面格式（jpg/png）", "アートワークの形式（jpg/png）",
                "Artwork format (jpg/png)");

        put("settings.section.naming", "命名", "ファイル名", "Naming");
        put("naming.album", "专辑目录", "アルバムのフォルダー", "Album folder");
        put("naming.playlist", "歌单目录", "プレイリストのフォルダー", "Playlist folder");
        put("naming.song", "歌曲文件", "曲のファイル名", "Song file name");
        put("naming.artist", "歌手子目录", "アーティストのサブフォルダー",
                "Artist subfolder");
        put("naming.explicit", "露骨标记（留空关闭）",
                "露骨コンテンツのマーク（空欄で無効）", "Explicit tag (blank = off)");
        put("naming.clean", "洁净标记", "クリーンのマーク", "Clean tag");
        put("naming.master", "Master 标记", "Master マーク", "Master tag");
        put("naming.limit", "文件名字符上限", "ファイル名の文字数上限",
                "Max filename characters");

        put("settings.section.mv", "MV 与其它", "MV など", "MV & misc");
        put("mv.audio", "MV 音轨（atmos/ac3/aac）", "MV の音声（atmos/ac3/aac）",
                "MV audio (atmos/ac3/aac)");
        put("mv.max", "MV 最高分辨率", "MV の最大解像度", "MV max resolution");
        put("mv.extraargs", "附加 CLI 参数", "追加の CLI 引数", "Extra CLI arguments");

        put("settings.sw.alacfix", "自动修复损坏的 ALAC 包",
                "壊れた ALAC を自動修復", "Auto-repair broken ALAC");
        put("settings.sw.lyricssave", "保存 .lrc 歌词文件",
                ".lrc 歌詞ファイルを保存", "Save .lrc lyrics file");
        put("settings.sw.lyricsembed", "歌词写入音频标签",
                "歌詞をタグに書き込む", "Embed lyrics in tags");
        put("settings.sw.coverembed", "内嵌封面", "アートワークを埋め込む",
                "Embed artwork");
        put("settings.sw.coverartist", "保存歌手头像封面",
                "アーティスト画像を保存", "Save artist artwork");
        put("settings.sw.tagsort", "写入排序标签", "並べ替えタグを書き込む",
                "Write sort tags");
        put("settings.sw.itunesid", "写入 iTunes ID", "iTunes ID を書き込む",
                "Write iTunes ID");
        put("settings.sw.songinfo", "歌单使用原曲信息",
                "プレイリストは原曲情報を使う", "Use original track info in playlists");
        put("settings.sw.artistfolder", "按歌手建子目录",
                "アーティストごとにサブフォルダー", "Subfolder per artist");
        put("settings.sw.m3u8", "保存 M3U8 播放列表",
                "M3U8 プレイリストを保存", "Save M3U8 playlist");

        put("settings.section.convert", "转码（内置 ffmpeg）",
                "変換（内蔵 ffmpeg）", "Transcoding (bundled ffmpeg)");
        put("convert.missing", "内置 ffmpeg 缺失，转码不可用",
                "内蔵 ffmpeg が見つからないため、変換は使えません",
                "Bundled ffmpeg is missing; transcoding is unavailable");
        put("convert.note", "已内置 ffmpeg 3.3.2（arm64），下载完成后在本机转码，不需要外挂",
                "ffmpeg 3.3.2（arm64）を内蔵しています。ダウンロード後に端末内で変換します",
                "ffmpeg 3.3.2 (arm64) is bundled; conversion runs on the device after download");
        put("convert.enable", "下载完成后转码", "ダウンロード後に変換",
                "Transcode after download");
        put("convert.format", "目标格式（flac / mp3 / opus / wav / copy）",
                "出力形式（flac / mp3 / opus / wav / copy）",
                "Target format (flac / mp3 / opus / wav / copy)");
        put("convert.keep", "转码后保留原文件", "変換後に元ファイルを残す",
                "Keep originals after transcoding");
        put("alaccheck.enable", "下载后检测 ALAC 完整性",
                "ダウンロード後に ALAC を検査", "Verify ALAC after download");
        put("alaccheck.note",
                "整首解码丢到 /dev/null，命中解码器报错即判为损坏（同 alac.sh）",
                "全曲を /dev/null にデコードし、デコーダーのエラーで破損と判定します（alac.sh と同じ）",
                "Decodes each track to /dev/null; a decoder error means corruption (as alac.sh)");

        put("settings.section.extra", "其它配置项", "その他の設定項目",
                "Other config keys");
        put("settings.extra.all", "上游配置共 %d 项，已全部由上面的设置覆盖",
                "上流の設定は全 %d 項で、すべて上記の設定で上書きしています",
                "All %d upstream keys are covered by the settings above");
        put("settings.extra.some",
                "上游配置共 %d 项，以下 %d 项还没有专用控件，可直接改",
                "上流の設定は全 %d 項です。次の %d 項には専用のコントロールがないため、直接編集できます",
                "%d upstream keys; %d of them have no dedicated control yet — edit them here");

        // -------------------------------------------------------- 对话框与提示
        put("dialog.perm.title", "需要文件访问权限",
                "ファイルへのアクセス権限が必要です", "File access permission needed");
        put("dialog.perm.body",
                "下载要直接写进“配置”里填写的目录（例如 %s），\n"
                        + "Android 11 及以上需要“所有文件访问权限”。\n"
                        + "Android 10 及以下只需普通的存储权限。",
                "ダウンロードは「設定」で指定したフォルダー（例 %s）に直接書き込みます。\n"
                        + "Android 11 以降では「すべてのファイルへのアクセス」権限が必要です。\n"
                        + "Android 10 以前は通常のストレージ権限だけで足ります。",
                "Downloads are written straight into the folder set in Settings (e.g. %s).\n"
                        + "Android 11 and later require the “All files access” permission.\n"
                        + "Android 10 and earlier only need normal storage permission.");
        put("dialog.perm.grant", "去授权", "許可する", "Grant");
        put("fatal.title", "启动失败", "起動に失敗しました", "Start-up failed");

        put("err.lite.empty", "请先在“配置”里填写解密服务地址",
                "まず「設定」で復号サービスのアドレスを入力してください",
                "Fill in the decryption service address in Settings first");
        put("err.urls.empty", "请先粘贴至少一个 Apple Music 链接",
                "Apple Music のリンクを 1 件以上貼り付けてください",
                "Paste at least one Apple Music link first");
        put("err.url.invalid", "链接看起来不是 Apple Music 地址：%s",
                "Apple Music の URL ではないようです：%s",
                "This does not look like an Apple Music link: %s");
        put("err.binary.missing", "找不到内置下载核心（本应用仅支持 arm64-v8a）",
                "内蔵のダウンロードコアが見つかりません（このアプリは arm64-v8a のみ対応）",
                "Bundled download core not found (this app is arm64-v8a only)");
        put("err.path.absolute", "下载路径需要是绝对路径（以 / 开头）：%s",
                "保存先は絶対パス（/ で始まる）で指定してください：%s",
                "The download folder must be an absolute path (starting with /): %s");
        put("err.dir.create", "无法创建目录：%s", "フォルダーを作成できません：%s",
                "Cannot create folder: %s");
        put("err.dir.empty", "请先在“配置”里填写下载目录",
                "まず「設定」で保存先を入力してください",
                "Set the download folder in Settings first");
        put("err.config.write", "写入配置失败：%s", "設定の書き込みに失敗しました：%s",
                "Cannot write the config file: %s");
        put("err.perm.guide",
                "请手动到 系统设置 → 应用 → 特殊权限 开启“所有文件访问权限”",
                "端末の設定 → アプリ → 特別な権限 で「すべてのファイルへのアクセス」を有効にしてください",
                "Enable “All files access” manually in System settings → Apps → Special access");
        put("err.pick.unsupported", "系统不支持选择文件：%s",
                "この端末ではファイル選択を開けません：%s",
                "This device cannot open a file picker: %s");
        put("err.read.file", "读取文件失败：%s", "ファイルの読み込みに失敗しました：%s",
                "Cannot read the file: %s");
        put("err.import.empty", "文件里没有找到链接（一行一个即可）",
                "ファイル内にリンクが見つかりません（1 行に 1 件で記載してください）",
                "No links found in the file (one per line is enough)");
        put("import.done", "导入 %d 个链接，当前共 %d 个",
                "リンクを %d 件取り込みました。合計 %d 件です",
                "Imported %d link(s); %d in total");

        // -------------------------------------------------- 前台服务与通知
        put("service.preparing", "准备中…", "準備中…", "Preparing…");
        put("service.result.done", "下载完成", "ダウンロードが完了しました",
                "Download finished");
        put("service.result.empty",
                "没有产出文件（退出码 %d），请检查 lite-server 与账号",
                "ファイルが作成されませんでした（終了コード %d）。lite-server とアカウントを確認してください",
                "No files were produced (exit code %d); check lite-server and the account");
        put("notify.title", "Apple Music 下载中", "Apple Music をダウンロード中",
                "Downloading from Apple Music");
        put("notify.stop", "停止", "停止", "Stop");
        put("channel.name", "下载任务", "ダウンロード", "Downloads");

        // ------------------------------------------------------- ALAC 完整性检测
        put("alac.start", "开始检查 ALAC 完整性…",
                "ALAC の整合性…",
                "Checking ALAC integrity…");
        put("alac.none", "没有需要检查的新文件",
                "検査する新規ファイルはありません", "No new files to check");
        put("alac.ok", "ALAC 检测完成：%d 首，全部正常",
                "ALAC 検査完了：%d 曲、すべて正常です",
                "ALAC check done: %d file(s), all healthy");
        put("alac.ok.notalac", "（另有 %d 首非 ALAC，已跳过）",
                "（ほかに非 ALAC の %d 曲をスキップしました）",
                " (%d non-ALAC file(s) skipped)");
        put("alac.bad", "ALAC 检测完成：%d 首，发现 %d 首损坏",
                "ALAC 検査完了：%d 曲中 %d 曲が破損しています",
                "ALAC check done: %d file(s), %d corrupted");
        put("alac.bad.truncated", "（其中 %d 首是下载不完整/被截断）",
                "（うち %d 曲はダウンロード不完全・切り詰め）",
                " (%d incomplete or truncated)");
        put("alac.bad.skipped", "，%d 首无法检查", "、%d 曲は検査できませんでした",
                ", %d could not be checked");
        put("alac.bad.file", "损坏文件：%s", "破損ファイル：%s", "Corrupted file: %s");
        put("alac.ffmpeg.missing", "内置 ffmpeg 缺失，跳过 ALAC 检测",
                "内蔵 ffmpeg がないため、ALAC 検査をスキップします",
                "Bundled ffmpeg is missing; skipping the ALAC check");
        put("alac.limit", "已达单次上限 %d 首，剩余文件下次再查",
                "1 回の上限 %d 曲に達しました。残りは次回検査します",
                "Reached the per-run limit of %d file(s); the rest is checked next time");
        put("alac.broken", "损坏：%s（文件不完整：%s）",
                "破損：%s（ファイルが不完全：%s）",
                "Corrupted: %s (incomplete file: %s)");
        put("alac.undecodable", "无法解码检查：%s", "デコード検査ができません：%s",
                "Cannot decode for checking: %s");
        put("alac.broken.reason", "损坏：%s（%s）", "破損：%s（%s）", "Corrupted: %s (%s)");
        put("alac.exitcode", "解码器退出码 %d", "デコーダーの終了コード %d",
                "Decoder exit code %d");
        put("alac.exitcode.msg", "解码器退出码 %d：%s", "デコーダーの終了コード %d：%s",
                "Decoder exit code %d: %s");
        put("alac.shortdecode", "只解出 %s / 全长 %s，音频在中途结束",
                "%s しかデコードできず（全体 %s）、途中で終わっています",
                "Decoded only %s of %s; the audio ends early");
        put("alac.struct.bytes", "文件只有 %d 字节",
                "ファイルは %d バイトしかありません", "File is only %d bytes");
        put("alac.struct.tail", "结尾残留 %d 字节，盒子头不完整",
                "末尾に %d バイト残り、ボックスのヘッダーが不完全です",
                "%d trailing byte(s) left; box header incomplete");
        put("alac.struct.box64", "盒子 %s 使用 64 位长度但头部不完整",
                "ボックス %s は 64 ビット長ですがヘッダーが不完全です",
                "Box %s uses a 64-bit size but its header is incomplete");
        put("alac.struct.size", "盒子 %s 长度非法（%d 字节）",
                "ボックス %s の長さが不正です（%d バイト）",
                "Box %s has an invalid size (%d bytes)");
        put("alac.struct.overflow", "盒子 %s 声明 %d 字节，文件在 %d 字节前就结束了",
                "ボックス %s は %d バイトと宣言していますが、ファイルは %d バイト手前で終わっています",
                "Box %s declares %d bytes but the file ends %d byte(s) earlier");
        put("alac.struct.toomany", "盒子数量异常（超过 10 万），文件可能被重复写入",
                "ボックスの数が異常です（10 万超）。ファイルが二重に書き込まれた可能性があります",
                "Abnormal box count (over 100k); the file may have been written twice");
        put("alac.struct.moov", "缺少 moov（元数据缺失，通常是下载未完成）",
                "moov がありません（メタデータ欠落。通常はダウンロード未完了）",
                "Missing moov (metadata absent; usually an unfinished download)");
        put("alac.struct.mdat", "缺少 mdat（没有任何音频数据）",
                "mdat がありません（音声データがありません）",
                "Missing mdat (no audio data at all)");
        put("alac.struct.unparsable", "结构无法解析：%s",
                "構造を解析できません：%s", "Cannot parse the structure: %s");

        // -------------------------------------------------- 下载核心启动（runner）
        put("runner.binary.missing", "找不到内置下载核心：%s",
                "内蔵のダウンロードコアが見つかりません：%s",
                "Bundled download core not found: %s");
        put("runner.dir.create", "无法创建输出目录：%s",
                "出力フォルダーを作成できません：%s",
                "Cannot create the output folder: %s");
        put("runner.exec.fail", "直接执行失败：%s，尝试兼容方式…",
                "直接実行に失敗しました：%s。互換モードを試します…",
                "Direct exec failed: %s; trying a compatible path…");
        put("runner.exec.failed", "无法启动下载核心，已停止。",
                "ダウンロードコアを起動できないため停止しました",
                "Could not start the download core; stopped");
        put("runner.stopping", "正在停止…", "停止しています…", "Stopping…");
        put("runner.read.fail", "读取输出失败：%s", "出力の読み取りに失敗しました：%s",
                "Cannot read the process output: %s");
        put("runner.no.ui", "未连接界面，自动选择全部曲目",
                "画面が接続されていないため、すべての曲を自動選択しました",
                "No UI attached; selecting every track automatically");
    }
}
