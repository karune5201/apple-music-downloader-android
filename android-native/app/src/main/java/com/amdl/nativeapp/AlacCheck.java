package com.amdl.nativeapp;

import android.util.Log;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.IOException;
import java.io.InputStream;
import java.io.RandomAccessFile;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.concurrent.TimeUnit;

/**
 * 下载后的 ALAC 完整性检测。
 *
 * <p>判定分三层，逐层加强：
 *
 * <ol>
 *   <li><b>容器结构</b>：顺序扫描顶层盒子，检查每个盒子声明的长度是否落在文件内。
 *       下载被中途切断时 mdat 会“声明 34 MB、实际只有 5 MB”，这一层在本地就能发现，
 *       不必启动解码器；</li>
 *   <li><b>解码结果</b>：整首解码丢到 null 输出，解码器报错（Invalid data found /
 *       error while decoding / get_bits / 退出码非 0 …）即判为损坏；</li>
 *   <li><b>解码时长</b>：把解码器报告的 {@code time=} 与容器头的 {@code Duration:}
 *       对比。有些坏文件解码不报错，只解出一半就结束，只看报错是发现不了的。</li>
 * </ol>
 *
 * <p>与 alac.sh 思路一致（内置 ffmpeg），但只检查本次下载产生的文件，
 * 并且多了结构检查和时长核对。
 */
public final class AlacCheck {

    private static final String TAG = "AlacCheck";

    /** 解码器报这些关键词就判定为损坏。 */
    private static final String[] BAD_MARKS = {
            "invalid data found when processing input",
            "error while decoding",
            "unexpected eof",
            "invalid frame",
            "not yet implemented in ffmpeg",
            "patch welcome",
            "patches welcome",
            "syntax element",
            "decode error",
            "moov atom not found",
            "partial file",
            "unable to find codec parameters",
            "could not find codec parameters",
            "packet corrupt",
            "non monotonically increasing dts",
            "invalid asal",
            "get_bits",
            "overread",
            "truncated",
            "invalid sample size",
            "sample size is invalid",
    };

    /** 这个差异以内算正常（封面、编码器 padding 会带来零点几秒偏差）。 */
    private static final long DURATION_TOLERANCE_MS = 1500L;
    /** 单首最多等 3 分钟，避免个别坏文件把收尾拖死。 */
    private static final long PER_FILE_TIMEOUT_MS = 180000L;
    /** 输出只留前面一段，够匹配关键词就行。 */
    private static final int LOG_CAP = 131072;
    /** 一次最多检查这么多首，防止大库跑太久。 */
    private static final int MAX_FILES = 300;

    public interface Sink {
        void line(String text);
    }

    public static final class Result {
        public int scanned;
        public int problems;
        public int skipped;
        public final List<String> problemFiles = new ArrayList<String>();
        /** 结构不完整或只解出一半，已计入 problems。 */
        public int truncated;
        /** 非 ALAC（AAC / Atmos 等）跳过，不计入问题。 */
        public int notAlac;
    }

    private AlacCheck() {
    }

    public static Result scan(String ffmpegPath, List<String> dirs, long sinceMillis, Sink sink) {
        Result result = new Result();
        if (ffmpegPath == null) {
            log(sink, Strings.get("alac.ffmpeg.missing"));
            return result;
        }
        long since = sinceMillis - 120000L; // 给文件时间戳留 2 分钟余量
        for (int i = 0; i < dirs.size(); i++) {
            if (result.scanned >= MAX_FILES) {
                break;
            }
            collect(ffmpegPath, new File(dirs.get(i)), since, result, sink);
        }
        if (result.scanned >= MAX_FILES) {
            log(sink, Strings.get("alac.limit", MAX_FILES));
        }
        return result;
    }

    private static void collect(String ffmpegPath, File dir, long since, Result result, Sink sink) {
        if (dir == null || !dir.isDirectory()) {
            return;
        }
        File[] files = dir.listFiles();
        if (files == null) {
            return;
        }
        for (int i = 0; i < files.length; i++) {
            if (result.scanned >= MAX_FILES) {
                return;
            }
            File f = files[i];
            if (f.isDirectory()) {
                collect(ffmpegPath, f, since, result, sink);
                continue;
            }
            String name = f.getName().toLowerCase(Locale.US);
            if (!name.endsWith(".m4a")) {
                continue;
            }
            if (f.lastModified() < since) {
                continue; // 只查本次下载的文件
            }
            checkOne(ffmpegPath, f, result, sink);
        }
    }

    private static void checkOne(String ffmpegPath, File file, Result result, Sink sink) {
        // 1) 容器结构：截断的文件不必再启动解码器
        String structural = structuralProblem(file);
        if (structural != null) {
            result.scanned++;
            result.problems++;
            result.truncated++;
            result.problemFiles.add(file.getAbsolutePath());
            log(sink, Strings.get("alac.broken", file.getName(), structural));
            return;
        }

        // 2) + 3) 解码并核对时长
        Probe probe = decode(ffmpegPath, file);
        result.scanned++;
        if (probe == null) {
            result.skipped++;
            log(sink, Strings.get("alac.undecodable", file.getName()));
            return;
        }
        if (probe.notAlac) {
            result.notAlac++;
            Log.i(TAG, "skip non-alac " + file.getName());
            return;
        }

        String problem = classify(probe);
        if (problem == null) {
            Log.i(TAG, "ok " + file.getName());
            return;
        }
        result.problems++;
        if (probe.shortDecode()) {
            result.truncated++;
        }
        result.problemFiles.add(file.getAbsolutePath());
        log(sink, Strings.get("alac.broken.reason", file.getName(), problem));
    }

    /** 结构、解码器报错、退出码、时长四者联合判定；正常返回 null。 */
    private static String classify(Probe probe) {
        String mark = firstBadMark(probe.log);
        if (mark != null) {
            return mark;
        }
        if (probe.exitCode != 0) {
            String first = firstErrorLine(probe.log);
            return first.isEmpty()
                    ? Strings.get("alac.exitcode", probe.exitCode)
                    : Strings.get("alac.exitcode.msg", probe.exitCode, first);
        }
        if (probe.shortDecode()) {
            return Strings.get("alac.shortdecode",
                    formatMs(probe.decodedMs), formatMs(probe.containerMs));
        }
        return null;
    }

    private static String firstErrorLine(String log) {
        if (log == null) {
            return "";
        }
        String[] lines = log.split("\n");
        for (int i = 0; i < lines.length; i++) {
            String line = lines[i].trim();
            if (line.isEmpty()) {
                continue;
            }
            if (line.contains("Error") || line.contains("error") || line.contains("Invalid")) {
                return line.length() > 160 ? line.substring(0, 160) + "…" : line;
            }
        }
        return "";
    }

    private static String firstBadMark(String log) {
        String normalized = log == null ? "" : log.toLowerCase(Locale.US);
        for (int i = 0; i < BAD_MARKS.length; i++) {
            if (normalized.contains(BAD_MARKS[i])) {
                return BAD_MARKS[i];
            }
        }
        return null;
    }

    // ------------------------------------------------------------ MP4 结构检查

    /**
     * 顺序遍历顶层盒子，确认每个盒子声明的长度都落在文件内。
     *
     * <p>返回 {@code null} 表示结构完整；否则返回一句人话，例如
     * “盒子 mdat 声明 35651584 字节，文件在 5873664 字节前就结束了”。
     */
    static String structuralProblem(File file) {
        RandomAccessFile raf = null;
        try {
            long length = file.length();
            if (length < 16) {
                return Strings.get("alac.struct.bytes", length);
            }
            raf = new RandomAccessFile(file, "r");
            long pos = 0;
            boolean sawMoov = false;
            boolean sawMdat = false;
            int boxes = 0;
            byte[] head = new byte[16];
            while (pos < length) {
                if (length - pos < 8) {
                    return Strings.get("alac.struct.tail", length - pos);
                }
                raf.seek(pos);
                raf.readFully(head, 0, 8);
                long size = readUInt32(head, 0);
                String type = new String(head, 4, 4, "US-ASCII");
                long headerLen = 8;
                if (size == 1) {
                    if (length - pos < 16) {
                        return Strings.get("alac.struct.box64", type);
                    }
                    raf.readFully(head, 8, 8);
                    size = readUInt64(head, 8);
                    headerLen = 16;
                } else if (size == 0) {
                    size = length - pos; // 延伸到文件末尾
                }
                if (size < headerLen) {
                    return Strings.get("alac.struct.size", type, size);
                }
                long end = pos + size;
                if (end > length) {
                    return Strings.get("alac.struct.overflow", type, size, end - length);
                }
                if ("moov".equals(type)) {
                    sawMoov = true;
                } else if ("mdat".equals(type)) {
                    sawMdat = true;
                }
                pos = end;
                boxes++;
                if (boxes > 100000) {
                    return Strings.get("alac.struct.toomany");
                }
            }
            if (!sawMoov) {
                return Strings.get("alac.struct.moov");
            }
            if (!sawMdat) {
                return Strings.get("alac.struct.mdat");
            }
            return null;
        } catch (Throwable e) {
            Log.w(TAG, "structure scan failed: " + file.getName(), e);
            return Strings.get("alac.struct.unparsable", e.getMessage());
        } finally {
            if (raf != null) {
                try {
                    raf.close();
                } catch (IOException ignored) {
                    // 打开失败时无需再关
                }
            }
        }
    }

    private static long readUInt32(byte[] b, int off) {
        return ((long) (b[off] & 0xFF) << 24)
                | ((long) (b[off + 1] & 0xFF) << 16)
                | ((long) (b[off + 2] & 0xFF) << 8)
                | ((long) (b[off + 3] & 0xFF));
    }

    private static long readUInt64(byte[] b, int off) {
        long high = readUInt32(b, off);
        long low = readUInt32(b, off + 4);
        return (high << 32) | low;
    }

    // ------------------------------------------------------------ 解码探测

    static final class Probe {
        String log = "";
        int exitCode;
        long containerMs = -1;
        long decodedMs = -1;
        boolean notAlac;

        boolean shortDecode() {
            if (containerMs <= 0 || decodedMs <= 0) {
                return false;
            }
            return decodedMs + DURATION_TOLERANCE_MS < containerMs;
        }
    }

    /**
     * 解码整首到 null 输出，收集解码器日志与两处时长：容器声明时长和解出的时长。
     * 进程起不来时返回 {@code null}。
     */
    static Probe decode(String ffmpegPath, File file) {
        Process process = null;
        Probe probe = new Probe();
        try {
            ProcessBuilder pb = new ProcessBuilder(ffmpegPath,
                    "-hide_banner", "-nostdin", "-loglevel", "info", "-stats",
                    "-i", file.getAbsolutePath(),
                    "-map", "0:a:0", "-f", "null", "-");
            pb.redirectErrorStream(true);
            process = pb.start();

            final InputStream in = process.getInputStream();
            final ByteArrayOutputStream buf = new ByteArrayOutputStream(8192);
            Thread reader = new Thread(new Runnable() {
                @Override
                public void run() {
                    byte[] chunk = new byte[4096];
                    try {
                        int n;
                        while ((n = in.read(chunk)) != -1) {
                            if (buf.size() < LOG_CAP) {
                                buf.write(chunk, 0, n);
                            }
                        }
                    } catch (IOException ignored) {
                        // 进程被 kill 时会走到这里
                    }
                }
            }, "alac-decode");
            reader.setDaemon(true);
            reader.start();

            boolean done = process.waitFor(PER_FILE_TIMEOUT_MS, TimeUnit.MILLISECONDS);
            if (!done) {
                process.destroy();
                reader.join(2000);
                probe.log = "timeout";
                probe.exitCode = -1;
                return probe;
            }
            reader.join(2000);
            probe.log = buf.toString("UTF-8");
            probe.exitCode = process.exitValue();
            probe.containerMs = parseDuration(probe.log, "Duration:");
            probe.decodedMs = parseLastTime(probe.log);
            inspectCodec(probe);
            return probe;
        } catch (Throwable e) {
            Log.w(TAG, "decode failed: " + file.getName(), e);
            return null;
        } finally {
            if (process != null) {
                process.destroy();
            }
        }
    }

    /** 只关心 ALAC：其它编码（AAC / Atmos）不走这套判定。 */
    private static void inspectCodec(Probe probe) {
        String lower = probe.log == null ? "" : probe.log.toLowerCase(Locale.US);
        int audio = lower.indexOf("audio:");
        if (audio < 0) {
            return;
        }
        int lineEnd = lower.indexOf('\n', audio);
        String line = lineEnd > audio ? lower.substring(audio, lineEnd) : lower.substring(audio);
        probe.notAlac = !line.contains("alac");
    }

    /** 解析 "Duration: 00:04:12.34" 之类的第一个时长。 */
    static long parseDuration(String log, String key) {
        if (log == null) {
            return -1;
        }
        int at = log.indexOf(key);
        if (at < 0) {
            return -1;
        }
        int start = at + key.length();
        int end = log.indexOf(',', start);
        if (end < 0) {
            end = Math.min(log.length(), start + 24);
        }
        return parseClock(log.substring(start, end).trim());
    }

    /** 解析最后一次出现的 "time=00:04:12.34"（解码进度）。 */
    static long parseLastTime(String log) {
        if (log == null) {
            return -1;
        }
        long result = -1;
        int from = 0;
        while (true) {
            int at = log.indexOf("time=", from);
            if (at < 0) {
                break;
            }
            int start = at + 5;
            int end = start;
            while (end < log.length() && end < start + 16) {
                char c = log.charAt(end);
                if ((c >= '0' && c <= '9') || c == ':' || c == '.') {
                    end++;
                    continue;
                }
                break;
            }
            long ms = parseClock(log.substring(start, end));
            if (ms >= 0) {
                result = ms;
            }
            from = start;
        }
        return result;
    }

    /** "HH:MM:SS.ss" → 毫秒；无法解析返回 -1。 */
    static long parseClock(String text) {
        if (text == null) {
            return -1;
        }
        text = text.trim();
        if (text.isEmpty() || text.startsWith("N/A")) {
            return -1;
        }
        String[] parts = text.split(":");
        if (parts.length != 3) {
            return -1;
        }
        try {
            double hours = Double.parseDouble(parts[0]);
            double minutes = Double.parseDouble(parts[1]);
            double seconds = Double.parseDouble(parts[2]);
            return (long) ((hours * 3600 + minutes * 60 + seconds) * 1000.0);
        } catch (NumberFormatException e) {
            return -1;
        }
    }

    private static String formatMs(long ms) {
        if (ms < 0) {
            return "?";
        }
        long totalSeconds = ms / 1000;
        return String.format(Locale.US, "%d:%02d", totalSeconds / 60, totalSeconds % 60);
    }

    private static void log(Sink sink, String text) {
        if (sink != null) {
            sink.line(text);
        }
        Log.i(TAG, text);
    }
}
