package com.amdl.nativeapp;

import android.content.Context;
import android.os.Handler;
import android.os.Looper;
import android.util.Log;

import java.io.BufferedReader;
import java.io.File;
import java.io.IOException;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Deque;
import java.util.List;
import java.util.concurrent.CopyOnWriteArrayList;

/**
 * Owns the single download process.
 *
 * <p>The upstream downloader is a Go CLI. It is shipped inside the APK as
 * {@code lib/arm64-v8a/libamdl.so} so that the platform extracts it into
 * {@code nativeLibraryDir}, the one place in an app sandbox where an executable
 * can be started on Android 10+ (writable directories such as
 * {@code filesDir} are mounted {@code noexec}).
 *
 * <p>Standard output of the CLI is streamed line by line to whoever is
 * listening, which gives the UI live progress without patching the CLI.
 */
public final class AmdlRunner {

    private static final String TAG = "AmdlRunner";
    private static final String BINARY_NAME = "libamdl.so";
    private static final int MAX_LOG_LINES = 4000;

    /** Receives process output; always called on the main thread. */
    public interface Listener {
        void onLogLine(String line);

        /**
         * A line that overwrites itself instead of scrolling: progress bars from
         * {@code schollz/progressbar} redraw with a carriage return. Kept separate
         * so a redraw never grows the log.
         */
        default void onTransient(String line) {
        }

        void onFinished(int exitCode, boolean cancelled);
    }

    /**
     * The CLI asks which tracks to rip by printing a prompt and waiting on stdin
     * ({@code --select}). Whoever can show a picker registers here; without a
     * handler the run would stall forever, so it answers "all" instead.
     */
    public interface PromptHandler {
        void onPrompt(String prompt);
    }

    /**
     * Prompts the CLI prints before reading a line from stdin.
     *
     * <p>The core localises its prompt text, so matching translated sentences
     * would be hopeless. It therefore prints a language-independent marker line
     * {@code ##prompt} right before asking; the suffix list below stays as a
     * fallback for cores from before that marker existed.
     */
    private static final String[] PROMPT_SUFFIXES = {
            "select:", "Enter your choice:", "Enter the choice:",
            "选择：", "選択："
    };

    /** Marker the core prints on its own line when it waits for stdin. */
    private static final String MACHINE_PROMPT = "prompt";

    /** ANSI colour sequences: the CLI colours its tables and prompts. */
    private static final java.util.regex.Pattern ANSI =
            java.util.regex.Pattern.compile("\u001B\\[[0-9;?]*[ -/]*[@-~]");

    static String stripAnsi(String text) {
        if (text == null) {
            return "";
        }
        return ANSI.matcher(text).replaceAll("");
    }

    private static final Object LOCK = new Object();
    private static final Deque<String> LOG = new ArrayDeque<>();
    private static final List<Listener> LISTENERS = new CopyOnWriteArrayList<>();
    private static final Handler MAIN = new Handler(Looper.getMainLooper());

    private static Process process;
    private static java.io.OutputStream stdin;
    private static PromptHandler promptHandler;
    private static volatile boolean cancelled;
    private static volatile boolean running;

    /**
     * The CLI exits with status 0 even when individual tracks failed, so the
     * final "<code>Completed: x/y | Warnings: n | Errors: m</code>" line is the
     * only reliable success signal. It is captured here while streaming.
     */
    private static final java.util.regex.Pattern SUMMARY = java.util.regex.Pattern.compile(
            "Completed:\\s*(\\d+)/(\\d+).*?Errors:\\s*(\\d+)");

    /**
     * Machine-readable summary the core prints after its human-readable one:
     * <code>##ok=13 total=14 warn=1 err=0</code>. It is parsed here, never shown
     * in the log; the activity renders it in the active UI language instead.
     */
    private static final java.util.regex.Pattern MACHINE_FIELD =
            java.util.regex.Pattern.compile("([A-Za-z_]+)\\s*=\\s*(-?\\d+)");
    private static final String MACHINE_PREFIX = "##";

    private static volatile int summaryCompleted = -1;
    private static volatile int summaryTotal = -1;
    private static volatile int summaryErrors = -1;
    private static volatile int summaryWarn = -1;
    /** Successful item count from the {@code ##} line, or -1. */
    private static volatile int summaryOkValue = -1;
    /** True once a {@code ##} line was seen: it wins over the legacy regex. */
    private static volatile boolean summaryMachine;

    private AmdlRunner() {
    }

    public static boolean isRunning() {
        return running;
    }

    public static boolean isCancelled() {
        return cancelled;
    }

    /** Tracks completed count parsed from the CLI summary line, or -1. */
    public static int summaryCompleted() {
        return summaryCompleted;
    }

    /** Tracks total count parsed from the CLI summary line, or -1. */
    public static int summaryTotal() {
        return summaryTotal;
    }

    /** Tracks error count parsed from the CLI summary line, or -1. */
    public static int summaryErrors() {
        return summaryErrors;
    }

    /** Warning count from the {@code ##} line, or -1 when the core is older. */
    public static int summaryWarn() {
        return summaryWarn;
    }

    /** Successful item count from the {@code ##} line, or -1. */
    public static int summaryOk() {
        return summaryOkValue;
    }

    /** Whether the last run printed a {@code ##} summary line. */
    public static boolean summaryMachine() {
        return summaryMachine;
    }

    private static void parseSummary(String line) {
        if (line == null || summaryMachine || !line.contains("Completed:")) {
            return;
        }
        java.util.regex.Matcher m = SUMMARY.matcher(line);
        if (m.find()) {
            try {
                summaryCompleted = Integer.parseInt(m.group(1));
                summaryTotal = Integer.parseInt(m.group(2));
                summaryErrors = Integer.parseInt(m.group(3));
            } catch (NumberFormatException ignored) {
                // Keep the previous values; the line is informational only.
            }
        }
    }

    /**
     * Parses a {@code ##ok=13 total=14 warn=1 err=0} line.
     *
     * @return true when the line is machine data, i.e. it must not reach the log
     *         view. Absent fields count as 0.
     */
    static boolean parseMachine(String line) {
        String text = line == null ? "" : line.trim();
        if (!text.startsWith(MACHINE_PREFIX)) {
            return false;
        }
        int ok = 0;
        int total = 0;
        int warn = 0;
        int err = 0;
        java.util.regex.Matcher m = MACHINE_FIELD.matcher(text.substring(MACHINE_PREFIX.length()));
        while (m.find()) {
            int value;
            try {
                value = Integer.parseInt(m.group(2));
            } catch (NumberFormatException e) {
                continue;
            }
            String key = m.group(1).toLowerCase(java.util.Locale.US);
            if ("ok".equals(key) || "done".equals(key)) {
                ok = value;
            } else if ("total".equals(key)) {
                total = value;
            } else if ("warn".equals(key) || "warns".equals(key)
                    || "warning".equals(key) || "warnings".equals(key)) {
                warn = value;
            } else if ("err".equals(key) || "errs".equals(key)
                    || "error".equals(key) || "errors".equals(key)) {
                err = value;
            }
        }
        summaryOkValue = ok;
        summaryCompleted = ok;
        summaryTotal = total;
        summaryWarn = warn;
        summaryErrors = err;
        summaryMachine = true;
        return true;
    }

    public static void addListener(Listener listener) {
        if (listener != null && !LISTENERS.contains(listener)) {
            LISTENERS.add(listener);
        }
    }

    public static void removeListener(Listener listener) {
        LISTENERS.remove(listener);
    }

    /** Snapshot of the buffered log, so a re-created activity can restore it. */
    public static List<String> logSnapshot() {
        synchronized (LOCK) {
            return new ArrayList<>(LOG);
        }
    }

    public static void clearLog() {
        synchronized (LOCK) {
            LOG.clear();
        }
    }

    /** Path of the bundled executable, or {@code null} when the ABI is missing. */
    public static File binary(Context context) {
        File lib = new File(context.getApplicationInfo().nativeLibraryDir, BINARY_NAME);
        if (lib.isFile()) {
            return lib;
        }
        return null;
    }

    /**
     * Starts the CLI.
     *
     * @param args  full argument vector placed after the binary path
     * @param cwd   working directory for relative paths (the output folder)
     * @param dns   optional DNS server list exported as {@code AMDL_DNS}
     */
    public static void start(Context context, List<String> args, File cwd, String dns) {
        synchronized (LOCK) {
            if (running) {
                return;
            }
            cancelled = false;
            running = true;
            summaryCompleted = -1;
            summaryTotal = -1;
            summaryErrors = -1;
            summaryWarn = -1;
            summaryOkValue = -1;
            summaryMachine = false;
        }

        File binary = binary(context);
        if (binary == null) {
            emitAll(Strings.get("runner.binary.missing", BINARY_NAME));
            finish(-1, false);
            return;
        }
        if (!cwd.exists() && !cwd.mkdirs()) {
            emitAll(Strings.get("runner.dir.create", cwd));
            finish(-1, false);
            return;
        }

        List<String> command = new ArrayList<>(args.size() + 1);
        command.add(binary.getAbsolutePath());
        command.addAll(args);

        ProcessBuilder pb = new ProcessBuilder(command);
        pb.directory(cwd);
        pb.redirectErrorStream(true);
        if (dns != null && !dns.trim().isEmpty()) {
            pb.environment().put("AMDL_DNS", dns.trim());
        }
        // Android has no HOME; the CLI does not need it, but keep the env tidy.
        pb.environment().put("TMPDIR", context.getCacheDir().getAbsolutePath());

        try {
            process = pb.start();
            stdin = process.getOutputStream();
        } catch (IOException first) {
            Log.w(TAG, "direct exec failed", first);
            emitAll(Strings.get("runner.exec.fail", String.valueOf(first.getMessage())));
            if (!startFallback(context, binary, command, cwd, pb)) {
                emitAll(Strings.get("runner.exec.failed"));
                finish(-1, false);
                return;
            }
        }

        emitAll("$ " + join(command));
        final Process started = process;
        Thread reader = new Thread(new Runnable() {
            @Override
            public void run() {
                pump(started);
            }
        }, "amdl-stdout");
        reader.setDaemon(true);
        reader.start();

        Thread waiter = new Thread(new Runnable() {
            @Override
            public void run() {
                int code;
                try {
                    code = started.waitFor();
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    code = -1;
                }
                finish(code, cancelled);
            }
        }, "amdl-wait");
        waiter.setDaemon(true);
        waiter.start();
    }

    /**
     * Some vendor ROMs refuse to exec a file from {@code nativeLibraryDir}.
     * Executing through {@code sh} keeps the same semantics and works on those
     * devices because the shell performs the exec from its own domain.
     */
    private static boolean startFallback(Context context, File binary, List<String> command,
                                         File cwd, ProcessBuilder original) {
        List<String> wrapped = new ArrayList<>(command.size() + 2);
        wrapped.add("/system/bin/sh");
        wrapped.add("-c");
        wrapped.add("exec " + shellQuote(binary.getAbsolutePath())
                + " " + tailQuoted(command));
        ProcessBuilder pb = new ProcessBuilder(wrapped);
        pb.directory(cwd);
        pb.redirectErrorStream(true);
        pb.environment().putAll(original.environment());
        try {
            process = pb.start();
            stdin = process.getOutputStream();
            return true;
        } catch (IOException second) {
            Log.w(TAG, "sh fallback failed", second);
            return false;
        }
    }

    public static void cancel() {
        Process p;
        synchronized (LOCK) {
            p = process;
            cancelled = true;
        }
        if (p != null) {
            emitAll(Strings.get("runner.stopping"));
            p.destroy();
            MAIN.postDelayed(new Runnable() {
                @Override
                public void run() {
                    Process still = process;
                    if (still != null && still.isAlive()) {
                        still.destroyForcibly();
                    }
                }
            }, 3000);
        }
    }

    /** Registers the UI that can answer track-selection prompts; null to clear. */
    public static void setPromptHandler(PromptHandler handler) {
        promptHandler = handler;
    }

    /**
     * Answers a CLI prompt.
     *
     * @param text raw selection such as {@code "1,3,5-8"} or {@code "all"}; an
     *             empty string makes the CLI skip that item
     * @return true when the line was accepted by the child process
     */
    public static boolean writeStdin(String text) {
        java.io.OutputStream out = stdin;
        if (out == null) {
            return false;
        }
        try {
            out.write((text == null ? "" : text).getBytes(StandardCharsets.UTF_8));
            out.write('\n');
            out.flush();
            Log.i(TAG, "stdin -> " + text);
            return true;
        } catch (IOException e) {
            Log.w(TAG, "cannot write to stdin", e);
            return false;
        }
    }

    /**
     * Streams stdout, splitting on newlines.
     *
     * <p>A plain {@code readLine()} cannot be used: the selection prompt is
     * printed with {@code fmt.Print}, i.e. without a trailing newline, so waiting
     * for a line would deadlock. Characters are therefore consumed one by one and
     * a known prompt suffix ends the current fragment.
     */
    private static void pump(Process p) {
        try (java.io.Reader reader = new InputStreamReader(
                p.getInputStream(), StandardCharsets.UTF_8)) {
            StringBuilder pending = new StringBuilder();
            long lastTransientAt = 0;
            boolean lastWasTransient = false;
            int ch;
            while ((ch = reader.read()) != -1) {
                if (ch == '\r') {
                    // Progress-bar frame: show it, but never append it to the log.
                    String frame = pending.toString();
                    pending.setLength(0);
                    if (frame.length() > 0) {
                        long now = System.currentTimeMillis();
                        if (now - lastTransientAt >= 100) {
                            lastTransientAt = now;
                            emitTransient(frame);
                        }
                        lastWasTransient = true;
                    }
                    continue;
                }
                if (ch == '\n') {
                    String line = pending.toString();
                    if (line.endsWith("\r")) {
                        line = line.substring(0, line.length() - 1);
                    }
                    pending.setLength(0);
                    if (line.isEmpty() && lastWasTransient) {
                        lastWasTransient = false;
                        continue;
                    }
                    lastWasTransient = false;
                    if (isPromptMarker(line)) {
                        // The core is about to read a line from stdin: open the
                        // picker. The marker itself never reaches the log.
                        handlePrompt("select:");
                        continue;
                    }
                    emitAll(line);
                    continue;
                }
                pending.append((char) ch);
                String prompt = matchPrompt(pending);
                if (prompt != null) {
                    pending.setLength(0);
                    emitAll(prompt.trim());
                    handlePrompt(prompt);
                }
            }
            if (pending.length() > 0) {
                String tail = pending.toString();
                pending.setLength(0);
                if (isPromptMarker(tail)) {
                    // The question follows on the next line; ask the user now.
                    handlePrompt("select:");
                } else {
                    emitAll(tail);
                }
            }
        } catch (IOException e) {
            emitAll(Strings.get("runner.read.fail", String.valueOf(e.getMessage())));
        }
    }

    /** True when the accumulated text is the {@code ##prompt} marker line. */
    private static boolean isPromptMarker(String text) {
        String clean = stripAnsi(text).trim();
        return clean.equals(MACHINE_PREFIX + MACHINE_PROMPT);
    }

    private static String matchPrompt(StringBuilder pending) {
        String clean = stripAnsi(pending.toString()).trim();
        for (int i = 0; i < PROMPT_SUFFIXES.length; i++) {
            if (clean.endsWith(PROMPT_SUFFIXES[i])) {
                return PROMPT_SUFFIXES[i];
            }
        }
        return null;
    }

    private static void handlePrompt(String prompt) {
        final PromptHandler handler = promptHandler;
        MAIN.post(new Runnable() {
            @Override
            public void run() {
                if (handler != null) {
                    handler.onPrompt(prompt);
                } else {
                    // Nobody can ask the user: keep the run going by taking all.
                    emitAll(Strings.get("runner.no.ui"));
                    writeStdin("all");
                }
            }
        });
    }

    private static void finish(int exitCode, boolean wasCancelled) {
        synchronized (LOCK) {
            process = null;
            stdin = null;
            running = false;
        }
        for (int i = 0; i < LISTENERS.size(); i++) {
            final Listener listener = LISTENERS.get(i);
            MAIN.post(new Runnable() {
                @Override
                public void run() {
                    listener.onFinished(exitCode, wasCancelled);
                }
            });
        }
    }

    private static void emitTransient(String text) {
        String clean = stripAnsi(text).trim();
        if (clean.isEmpty()) {
            return;
        }
        if (clean.length() > 200) {
            clean = clean.substring(0, 200) + "…";
        }
        final String shown = clean;
        for (int i = 0; i < LISTENERS.size(); i++) {
            final Listener listener = LISTENERS.get(i);
            MAIN.post(new Runnable() {
                @Override
                public void run() {
                    listener.onTransient(shown);
                }
            });
        }
    }

    /** 运行结束后（如下载后的 ALAC 检测）往上抛日志行，界面照常显示。 */
    public static void notifyLine(String line) {
        if (line == null || line.isEmpty()) {
            return;
        }
        emitAll(line.startsWith("[")
                ? line
                : "[" + Strings.get("log.check.tag") + "] " + line);
    }

    private static void emitAll(String line) {
        line = stripAnsi(line);
        if (parseMachine(line)) {
            // 机器可读汇总：只更新状态，不进日志、不打扰用户
            return;
        }
        parseSummary(line);
        synchronized (LOCK) {
            LOG.addLast(line);
            while (LOG.size() > MAX_LOG_LINES) {
                LOG.removeFirst();
            }
        }
        for (int i = 0; i < LISTENERS.size(); i++) {
            final Listener listener = LISTENERS.get(i);
            final String text = line;
            MAIN.post(new Runnable() {
                @Override
                public void run() {
                    listener.onLogLine(text);
                }
            });
        }
    }

    private static String join(List<String> command) {
        StringBuilder sb = new StringBuilder();
        for (String part : command) {
            if (sb.length() > 0) {
                sb.append(' ');
            }
            sb.append(shellQuote(part));
        }
        return sb.toString();
    }

    private static String tailQuoted(List<String> command) {
        StringBuilder sb = new StringBuilder();
        for (int i = 1; i < command.size(); i++) {
            if (sb.length() > 0) {
                sb.append(' ');
            }
            sb.append(shellQuote(command.get(i)));
        }
        return sb.toString();
    }

    static String shellQuote(String value) {
        return "'" + value.replace("'", "'\\''") + "'";
    }
}
