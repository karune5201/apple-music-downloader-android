package com.amdl.nativeapp;

import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.Map;
import java.util.Set;

/**
 * 应用已经“认识”的 config.yaml 键，以及它们对应的取值来源。
 *
 * <p>这张表是 App 与上游配置之间的契约：表里有的键由界面控件负责，其余键交给
 * {@link ConfigTemplate} 用上游默认值填充，并在设置面板里自动生成控件。
 * 这样上游新增配置项时，不更新 App 也能看到并修改它。
 */
public final class ConfigKeys {

    /** 由“设置 → 其它配置项”里的开关控制。 */
    private static final Set<String> BOOL = new LinkedHashSet<String>();

    static {
        BOOL.add("metadata.lyrics.save-file");
        BOOL.add("metadata.lyrics.embed");
        BOOL.add("metadata.artwork.embed");
        BOOL.add("metadata.artwork.save-artist");
        BOOL.add("metadata.tags.sort-order");
        BOOL.add("metadata.tags.itunes-id");
        BOOL.add("metadata.tags.use-songinfo-for-playlist");
    }

    /**
     * 由 App 强制固定的键：手机上没有终端、也没有 ffmpeg，这些值必须固定，
     * 否则会卡在交互提示或转换失败上。
     */
    private static final Map<String, String> FORCED = new LinkedHashMap<String, String>();

    static {
        FORCED.put("general.exit-on-error", "true");
        FORCED.put("convert.check-bad-alac", "false");
        FORCED.put("convert.delete-bad-alac", "false");
        FORCED.put("metadata.artwork.save-animated", "false");
        FORCED.put("metadata.artwork.emby-animated", "false");
        FORCED.put("metadata.artwork.dl-for-playlist", "false");
        FORCED.put("convert.skip-if-source-matches", "true");
        // 有损转无损的告警在手机上没有意义（看不到 stdout），直接跳过
        FORCED.put("convert.skip-lossy-to-lossless", "true");
        FORCED.put("convert.warn-lossy-to-lossless", "false");
    }

    private ConfigKeys() {
    }

    static boolean isBoolean(String path) {
        return BOOL.contains(path);
    }

    /** ffmpeg 的绝对路径在运行时才确定（nativeLibraryDir），由 ConfigWriter 填入。 */
    static final String KEY_FFMPEG_PATH = "convert.ffmpeg-path";

    /** 已知键 → 由 App 决定的值；这些键不会再出现在“其它配置项”里。 */
    static Map<String, String> values(Prefs prefs) {
        Map<String, String> map = new LinkedHashMap<String, String>(64);
        map.put("general.lite-server", prefs.liteServer());
        // App 与 Go 核心共用的界面语言；核心也读这一项来决定自己的日志语言
        map.put("general.ui-language", prefs.uiLanguage());
        map.put("general.media-user-token", prefs.mediaUserToken());
        map.put("general.language", prefs.language());
        map.put("general.max-memory-limit", String.valueOf(prefs.maxMemory()));
        map.put("general.proxy", prefs.proxy());
        map.put("general.storefront", prefs.storefront());
        map.put("general.authorization-token", prefs.authorizationToken());

        map.put("media.get-m3u8-mode", prefs.m3u8Mode());
        map.put("media.alac-max", String.valueOf(prefs.alacMax()));
        map.put("media.atmos-max", String.valueOf(prefs.atmosMax()));
        map.put("media.aac-type", prefs.aacType());
        map.put("media.alac-fix", String.valueOf(prefs.alacFix()));
        map.put("media.mv.audio-type", prefs.mvAudioType());
        map.put("media.mv.max", String.valueOf(prefs.mvMax()));

        map.put("paths.alac", prefs.dirAlac());
        map.put("paths.atmos", prefs.dirAtmos());
        map.put("paths.aac", prefs.dirAac());
        map.put("paths.mv", prefs.dirMv());
        map.put("paths.album-folder", prefs.albumTemplate());
        map.put("paths.playlist-folder", prefs.playlistTemplate());
        map.put("paths.artist-folder", prefs.artistFolder() ? prefs.artistTemplate() : "");
        map.put("paths.song-file", prefs.songTemplate());
        map.put("paths.limit-max", String.valueOf(prefs.limitMax()));
        map.put("paths.explicit", prefs.tagExplicit());
        map.put("paths.clean", prefs.tagClean());
        map.put("paths.apple-master", prefs.tagMaster());

        map.put("metadata.lyrics.save-file", String.valueOf(prefs.lyricsSave()));
        map.put("metadata.lyrics.embed", String.valueOf(prefs.lyricsEmbed()));
        map.put("metadata.lyrics.type", prefs.lyricsType());
        map.put("metadata.lyrics.format", prefs.lyricsFormat());
        map.put("metadata.lyrics.extra", prefs.lyricsExtra());
        map.put("metadata.artwork.embed", String.valueOf(prefs.coverEmbed()));
        map.put("metadata.artwork.size", prefs.coverSize());
        map.put("metadata.artwork.format", prefs.coverFormat());
        map.put("metadata.artwork.save-artist", String.valueOf(prefs.coverSaveArtist()));
        map.put("metadata.tags.sort-order", String.valueOf(prefs.tagSortOrder()));
        map.put("metadata.tags.itunes-id", String.valueOf(prefs.tagItunesId()));
        map.put("metadata.tags.use-songinfo-for-playlist", String.valueOf(prefs.tagUseSonginfo()));

        map.put("convert.after-download", String.valueOf(prefs.convertEnabled()));
        map.put("convert.format", prefs.convertFormat());
        map.put("convert.keep-original", String.valueOf(prefs.convertKeepOriginal()));
        map.put("convert.with-metadata", "true");

        return map;
    }

    static Map<String, String> forced() {
        return FORCED;
    }

    /** App 是否已经接管这个键（含被强制固定的键）。 */
    static boolean isKnown(String path, Map<String, String> known) {
        return known.containsKey(path) || FORCED.containsKey(path);
    }
}
