package com.amdl.nativeapp;

import android.content.Context;
import android.content.SharedPreferences;
import android.content.res.Configuration;

import java.util.Map;

/**
 * All user settings, backed by {@link SharedPreferences}.
 *
 * <p>Keys mirror the upstream config.yaml names so {@link ConfigWriter} stays a
 * straight mapping. A small generic API ({@link #str}, {@link #bool}, {@link #num})
 * lets the settings screen bind fields by key instead of one method per option.
 */
public final class Prefs {

    public static final String QUALITY_ALAC = "alac";
    public static final String QUALITY_ATMOS = "atmos";
    public static final String QUALITY_AAC = "aac";

    private static final String FILE = "amdl_prefs";

    // Generic accessors are public so the settings dialog can bind by key.
    public static final String K_LITE_SERVER = "lite_server";
    public static final String K_QUALITY = "quality";
    public static final String K_STOREFRONT = "storefront";
    public static final String K_LANGUAGE = "language";
    public static final String K_PROXY = "proxy";
    public static final String K_MEDIA_TOKEN = "media_user_token";
    public static final String K_AUTH_TOKEN = "authorization_token";
    public static final String K_DNS = "dns";
    /** 每个音质各自的保存根目录，直接写绝对路径（核心只认真实路径）。 */
    public static final String K_DIR_ALAC = "dir_alac";
    public static final String K_DIR_ATMOS = "dir_atmos";
    public static final String K_DIR_AAC = "dir_aac";
    public static final String K_DIR_MV = "dir_mv";
    public static final String DEFAULT_MUSIC_ROOT = "/storage/emulated/0/Music/AM";
    public static final String K_ARTIST_FOLDER = "artist_folder";
    public static final String K_EXTRA_ARGS = "extra_args";
    public static final String K_LAST_URLS = "last_urls";
    public static final String K_MAX_MEMORY = "max_memory";
    public static final String K_M3U8_MODE = "m3u8_mode";
    public static final String K_LIMIT_MAX = "limit_max";
    public static final String K_ALAC_MAX = "alac_max";
    public static final String K_ATMOS_MAX = "atmos_max";
    public static final String K_AAC_TYPE = "aac_type";
    public static final String K_ALAC_FIX = "alac_fix";
    public static final String K_MV_AUDIO = "mv_audio";
    public static final String K_MV_MAX = "mv_max";
    public static final String K_LYRICS_SAVE = "lyrics_save";
    public static final String K_LYRICS_EMBED = "lyrics_embed";
    public static final String K_LYRICS_TYPE = "lyrics_type";
    public static final String K_LYRICS_FORMAT = "lyrics_format";
    public static final String K_LYRICS_EXTRA = "lyrics_extra";
    public static final String K_COVER_EMBED = "cover_embed";
    public static final String K_COVER_SIZE = "cover_size";
    public static final String K_COVER_FORMAT = "cover_format";
    public static final String K_COVER_ARTIST = "cover_save_artist";
    public static final String K_TAG_SORT = "tag_sort";
    public static final String K_TAG_ITUNES = "tag_itunes";
    public static final String K_TAG_SONGINFO = "tag_songinfo";
    public static final String K_ALBUM_TPL = "album_folder";
    public static final String K_PLAYLIST_TPL = "playlist_folder";
    public static final String K_SONG_TPL = "song_file";
    public static final String K_ARTIST_TPL = "artist_folder_tpl";
    public static final String K_TAG_EXPLICIT = "tag_explicit";
    public static final String K_TAG_CLEAN = "tag_clean";
    public static final String K_TAG_MASTER = "tag_master";
    public static final String K_SAVE_M3U8 = "save_m3u8";
    /** “其它配置项”的键前缀：后面跟上游 config.yaml 的点号路径。 */
    public static final String EXTRA_PREFIX = "cfg:";
    public static final String K_CONVERT_ENABLED = "convert_enabled";
    public static final String K_CONVERT_FORMAT = "convert_format";
    public static final String K_CONVERT_KEEP = "convert_keep_original";
    public static final String K_ALAC_CHECK = "alac_check";
    public static final String K_LOG_HEIGHT = "log_height_dp";
    public static final String K_SELECT_TRACKS = "select_tracks";
    public static final String K_THEME = "theme";
    /** 界面语言（zh/ja/en）：既决定 App 文案，也写进 config.yaml 的 general.ui-language。 */
    public static final String K_UI_LANGUAGE = "ui_language";

    public static final String THEME_SYSTEM = "system";
    public static final String THEME_LIGHT = "light";
    public static final String THEME_DARK = "dark";

    private final SharedPreferences sp;

    public Prefs(Context context) {
        this.sp = context.getApplicationContext().getSharedPreferences(FILE, Context.MODE_PRIVATE);
    }

    // ------------------------------------------------------------ generic API

    public String str(String key, String fallback) {
        String value = sp.getString(key, fallback);
        if (value == null) {
            return fallback;
        }
        String trimmed = value.trim();
        return trimmed.isEmpty() ? fallback : trimmed;
    }

    /** 遍历全部键值，用于导出“其它配置项”。 */
    public Map<String, ?> all() {
        return sp.getAll();
    }

    public String extra(String path, String fallback) {
        String value = raw(EXTRA_PREFIX + path);
        return value == null || value.isEmpty() ? fallback : value;
    }

    public void setExtra(String path, String value) {
        setStr(EXTRA_PREFIX + path, value);
    }

    public String raw(String key) {
        return sp.getString(key, "");
    }

    public void setStr(String key, String value) {
        String v = value == null ? "" : value.trim();
        sp.edit().putString(key, v).apply();
    }

    public boolean bool(String key, boolean fallback) {
        return sp.getBoolean(key, fallback);
    }

    public void setBool(String key, boolean value) {
        sp.edit().putBoolean(key, value).apply();
    }

    public int num(String key, int fallback) {
        String value = sp.getString(key, "");
        if (value == null || value.trim().isEmpty()) {
            return fallback;
        }
        try {
            return Integer.parseInt(value.trim());
        } catch (NumberFormatException e) {
            return fallback;
        }
    }

    public void setNum(String key, int value) {
        sp.edit().putString(key, Integer.toString(value)).apply();
    }

    // ------------------------------------------------------------ named values

    public String liteServer() {
        return str(K_LITE_SERVER, "http://127.0.0.1:12340");
    }

    public void setLiteServer(String value) {
        setStr(K_LITE_SERVER, value);
    }

    public String quality() {
        return str(K_QUALITY, QUALITY_ALAC);
    }

    public void setQuality(String value) {
        setStr(K_QUALITY, value);
    }

    public String storefront() {
        return str(K_STOREFRONT, "us");
    }

    public String language() {
        return raw(K_LANGUAGE);
    }

    public String proxy() {
        return raw(K_PROXY);
    }

    public String mediaUserToken() {
        return raw(K_MEDIA_TOKEN);
    }

    public String authorizationToken() {
        return raw(K_AUTH_TOKEN);
    }

    public String dns() {
        return raw(K_DNS);
    }

    public String extraArgs() {
        return raw(K_EXTRA_ARGS);
    }

    public void setExtraArgs(String value) {
        setStr(K_EXTRA_ARGS, value);
    }

    public String dirAlac() {
        return str(K_DIR_ALAC, DEFAULT_MUSIC_ROOT + "/ALAC");
    }

    public String dirAtmos() {
        return str(K_DIR_ATMOS, DEFAULT_MUSIC_ROOT + "/Atmos");
    }

    public String dirAac() {
        return str(K_DIR_AAC, DEFAULT_MUSIC_ROOT + "/AAC");
    }

    public String dirMv() {
        return str(K_DIR_MV, DEFAULT_MUSIC_ROOT + "/MV");
    }

    /** 与当前音质对应的保存目录。 */
    public String dirForQuality(String quality) {
        if (QUALITY_ATMOS.equals(quality)) {
            return dirAtmos();
        }
        if (QUALITY_AAC.equals(quality)) {
            return dirAac();
        }
        return dirAlac();
    }

    public boolean artistFolder() {
        return bool(K_ARTIST_FOLDER, true);
    }

    public String lastUrls() {
        return raw(K_LAST_URLS);
    }

    public void setLastUrls(String value) {
        sp.edit().putString(K_LAST_URLS, value == null ? "" : value).apply();
    }

    public int alacMax() {
        return num(K_ALAC_MAX, 192000);
    }

    public int atmosMax() {
        return num(K_ATMOS_MAX, 2768);
    }

    public int mvMax() {
        return num(K_MV_MAX, 2160);
    }

    public int limitMax() {
        return num(K_LIMIT_MAX, 200);
    }

    public int maxMemory() {
        return num(K_MAX_MEMORY, 256);
    }

    public String m3u8Mode() {
        return str(K_M3U8_MODE, "hires");
    }

    public String aacType() {
        return str(K_AAC_TYPE, "aac-lc");
    }

    public boolean alacFix() {
        return bool(K_ALAC_FIX, true);
    }

    public String mvAudioType() {
        return str(K_MV_AUDIO, "atmos");
    }

    public boolean lyricsSave() {
        return bool(K_LYRICS_SAVE, true);
    }

    public boolean lyricsEmbed() {
        return bool(K_LYRICS_EMBED, true);
    }

    public String lyricsType() {
        return str(K_LYRICS_TYPE, "lyrics");
    }

    public String lyricsFormat() {
        return str(K_LYRICS_FORMAT, "lrc");
    }

    public String lyricsExtra() {
        return raw(K_LYRICS_EXTRA);
    }

    public boolean coverEmbed() {
        return bool(K_COVER_EMBED, true);
    }

    public String coverSize() {
        return str(K_COVER_SIZE, "3000x3000");
    }

    public String coverFormat() {
        return str(K_COVER_FORMAT, "jpg");
    }

    public boolean coverSaveArtist() {
        return bool(K_COVER_ARTIST, false);
    }

    public boolean tagSortOrder() {
        return bool(K_TAG_SORT, true);
    }

    public boolean tagItunesId() {
        return bool(K_TAG_ITUNES, true);
    }

    public boolean tagUseSonginfo() {
        return bool(K_TAG_SONGINFO, false);
    }

    public String albumTemplate() {
        return str(K_ALBUM_TPL, "{AlbumName}");
    }

    public String playlistTemplate() {
        return str(K_PLAYLIST_TPL, "{PlaylistName}");
    }

    public String songTemplate() {
        return str(K_SONG_TPL, "{SongNumer}. {SongName}");
    }

    public String artistTemplate() {
        return str(K_ARTIST_TPL, "{UrlArtistName}");
    }

    public String tagExplicit() {
        return raw(K_TAG_EXPLICIT);
    }

    public String tagClean() {
        return raw(K_TAG_CLEAN);
    }

    public String tagMaster() {
        return raw(K_TAG_MASTER);
    }

    public boolean saveM3u8() {
        return bool(K_SAVE_M3U8, false);
    }

    /** Height of the log card in dp; the user drags the handle to change it. */
    public int logHeightDp() {
        return num(K_LOG_HEIGHT, 150);
    }

    public void setLogHeightDp(int dp) {
        setNum(K_LOG_HEIGHT, dp);
    }

    /** 下载完成后用内置 ffmpeg 转码。 */
    public boolean convertEnabled() {
        return bool(K_CONVERT_ENABLED, false);
    }

    /** flac / mp3 / opus / wav / copy。 */
    public String convertFormat() {
        return str(K_CONVERT_FORMAT, "flac");
    }

    public boolean convertKeepOriginal() {
        return bool(K_CONVERT_KEEP, false);
    }

    /** 下载完成后用内置 ffmpeg 逐首解码，检查 ALAC 编码是否损坏。 */
    public boolean alacCheck() {
        return bool(K_ALAC_CHECK, true);
    }

    public boolean selectTracks() {
        return bool(K_SELECT_TRACKS, false);
    }

    public void setSelectTracks(boolean value) {
        setBool(K_SELECT_TRACKS, value);
    }

    public String themeMode() {
        return str(K_THEME, THEME_SYSTEM);
    }

    public void setThemeMode(String value) {
        setStr(K_THEME, value);
    }

    /** 明暗的最终结果：用户强制就听用户的，否则跟随系统。 */
    public boolean darkTheme(Context context) {
        String mode = themeMode();
        if (THEME_DARK.equals(mode)) {
            return true;
        }
        if (THEME_LIGHT.equals(mode)) {
            return false;
        }
        int uiMode = context.getResources().getConfiguration().uiMode
                & Configuration.UI_MODE_NIGHT_MASK;
        return uiMode == Configuration.UI_MODE_NIGHT_YES;
    }

    /** 界面语言：返回规范化后的 zh/ja/en，永远不为空。 */
    public String uiLanguage() {
        return Strings.normalise(raw(K_UI_LANGUAGE));
    }

    public void setUiLanguage(String tag) {
        setStr(K_UI_LANGUAGE, Strings.normalise(tag));
    }
}
