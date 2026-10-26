package com.amdl.nativeapp;

import java.io.File;
import java.util.ArrayList;
import java.util.List;

/**
 * 媒体库扫描：核心直接写进 /storage 下的目录，但 MediaStore 不会自动发现这些文件，
 * 本地播放器要等扫描后才能在音乐库里看到。
 */
public final class Exporter {

    private Exporter() {
    }

    /**
     * Indexes files written straight into shared storage so they show up in music
     * players immediately.
     *
     * @return number of files handed to the scanner
     */
    public static int scanMedia(File root) {
        if (root == null || !root.isDirectory()) {
            return 0;
        }
        List<File> files = new ArrayList<File>();
        collect(root, files);
        if (files.isEmpty()) {
            return 0;
        }
        String[] paths = new String[files.size()];
        for (int i = 0; i < files.size(); i++) {
            paths[i] = files.get(i).getAbsolutePath();
        }
        try {
            android.media.MediaScannerConnection.scanFile(AppContext.get(), paths, null, null);
        } catch (Throwable e) {
            return 0;
        }
        return paths.length;
    }

    private static void collect(File dir, List<File> out) {
        File[] children = dir.listFiles();
        if (children == null) {
            return;
        }
        for (int i = 0; i < children.length; i++) {
            File child = children[i];
            if (child.isDirectory()) {
                collect(child, out);
            } else if (isMedia(child.getName())) {
                out.add(child);
            }
        }
    }

    private static boolean isMedia(String name) {
        String lower = name.toLowerCase();
        return lower.endsWith(".m4a")
                || lower.endsWith(".mp4")
                || lower.endsWith(".m4v")
                || lower.endsWith(".mov")
                || lower.endsWith(".lrc")
                || lower.endsWith(".txt");
    }

    /** Holds an application context for helpers that have none. */
    static final class AppContext {
        private static android.content.Context context;

        static void init(android.content.Context ctx) {
            context = ctx.getApplicationContext();
        }

        static android.content.Context get() {
            if (context == null) {
                throw new IllegalStateException("AppContext not initialised");
            }
            return context;
        }
    }
}
