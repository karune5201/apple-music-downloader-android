package com.amdl.nativeapp;

import android.content.Context;
import android.os.Build;
import android.util.Log;

import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.OutputStreamWriter;
import java.io.PrintWriter;
import java.io.StringWriter;
import java.nio.charset.Charset;

/**
 * Persists uncaught exceptions so a crash can be diagnosed without adb.
 *
 * <p>The stack trace is appended to {@code files/crash.txt} inside the app data
 * directory (reachable over MTP at
 * {@code Android/data/com.amdl.nativeapp/files/crash.txt}) and shown by
 * {@link MainActivity} the next time the app starts.
 */
public final class CrashLog {

    private static final String TAG = "AmdlCrash";
    private static final String FILE_NAME = "crash.txt";
    private static volatile boolean installed;
    private static volatile String lastTrace;

    private CrashLog() {
    }

    public static void install(Context context) {
        if (installed) {
            return;
        }
        installed = true;
        final Context app = context.getApplicationContext() == null
                ? context : context.getApplicationContext();
        final Thread.UncaughtExceptionHandler previous = Thread.getDefaultUncaughtExceptionHandler();
        Thread.setDefaultUncaughtExceptionHandler(new Thread.UncaughtExceptionHandler() {
            @Override
            public void uncaughtException(Thread thread, Throwable error) {
                try {
                    record(app, thread, error);
                } catch (Throwable ignored) {
                    // Never mask the original failure.
                }
                if (previous != null) {
                    previous.uncaughtException(thread, error);
                }
            }
        });
    }

    private static void record(Context context, Thread thread, Throwable error) {
        StringWriter buffer = new StringWriter();
        PrintWriter writer = new PrintWriter(buffer);
        writer.println("=== " + System.currentTimeMillis() + " ("
                + Build.VERSION.SDK_INT + ") thread=" + thread.getName() + " ===");
        error.printStackTrace(writer);
        writer.flush();
        String trace = buffer.toString();
        lastTrace = trace;
        Log.e(TAG, trace);

        File file = file(context);
        File parent = file.getParentFile();
        if (parent != null && !parent.exists()) {
            //noinspection ResultOfMethodCallIgnored
            parent.mkdirs();
        }
        try (FileOutputStream out = new FileOutputStream(file, true)) {
            OutputStreamWriter w = new OutputStreamWriter(out, Charset.forName("UTF-8"));
            w.write(trace);
            w.write('\n');
            w.flush();
        } catch (IOException e) {
            Log.e(TAG, "cannot write crash log", e);
        }
    }

    public static File file(Context context) {
        return new File(context.getFilesDir(), FILE_NAME);
    }

    /** Contents of the persisted crash log, or an empty string. */
    public static String read(Context context) {
        File file = file(context);
        if (!file.isFile()) {
            return lastTrace == null ? "" : lastTrace;
        }
        try (java.io.FileInputStream in = new java.io.FileInputStream(file)) {
            byte[] data = new byte[(int) Math.min(file.length(), 200_000)];
            int read = in.read(data);
            if (read <= 0) {
                return "";
            }
            return new String(data, 0, read, Charset.forName("UTF-8"));
        } catch (IOException e) {
            return lastTrace == null ? "" : lastTrace;
        }
    }

    public static void clear(Context context) {
        lastTrace = null;
        //noinspection ResultOfMethodCallIgnored
        file(context).delete();
    }
}
