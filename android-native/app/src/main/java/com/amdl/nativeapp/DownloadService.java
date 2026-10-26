package com.amdl.nativeapp;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.graphics.drawable.Icon;
import android.os.Build;
import android.os.IBinder;
import android.util.Log;

import java.io.File;
import java.util.ArrayList;

/**
 * Keeps a download alive while the activity is in the background.
 *
 * <p>Android kills plain background threads and processes; a foreground service
 * with an ongoing notification is the supported way to keep a long download
 * running with the screen off.
 */
public class DownloadService extends Service implements AmdlRunner.Listener {

    private static final String TAG = "DownloadService";

    public static final String ACTION_START = "com.amdl.nativeapp.action.START";
    public static final String ACTION_STOP = "com.amdl.nativeapp.action.STOP";

    public static final String EXTRA_ARGS = "args";
    public static final String EXTRA_CWD = "cwd";
    public static final String EXTRA_DNS = "dns";
    public static final String EXTRA_SCAN_DIRS = "scan_dirs";

    private static final String CHANNEL_ID = "downloads";
    private static final int NOTIFICATION_ID = 42;

    private NotificationManager notificationManager;
    private volatile String lastLine = "";
    private long lastNotifiedAt;
    /** 下载根目录，完成后触发媒体库扫描，本地播放器立刻能看到。 */
    private ArrayList<String> scanDirs = new ArrayList<String>();
    /** 本次下载开始时间：ALAC 检测只查这之后落盘的文件。 */
    private long runStartedAt = System.currentTimeMillis();

    public static void start(Context context, ArrayList<String> args, File cwd, String dns,
                             ArrayList<String> scanDirs) {
        Intent intent = new Intent(context, DownloadService.class);
        intent.setAction(ACTION_START);
        intent.putStringArrayListExtra(EXTRA_ARGS, args);
        intent.putExtra(EXTRA_CWD, cwd.getAbsolutePath());
        intent.putExtra(EXTRA_DNS, dns);
        intent.putStringArrayListExtra(EXTRA_SCAN_DIRS, scanDirs);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.startForegroundService(intent);
        } else {
            context.startService(intent);
        }
    }

    public static void stop(Context context) {
        Intent intent = new Intent(context, DownloadService.class);
        intent.setAction(ACTION_STOP);
        context.startService(intent);
    }

    @Override
    public void onCreate() {
        super.onCreate();
        notificationManager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        createChannel();
        AmdlRunner.addListener(this);
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        String action = intent == null ? null : intent.getAction();
        if (ACTION_STOP.equals(action)) {
            AmdlRunner.cancel();
            return START_NOT_STICKY;
        }
        if (!ACTION_START.equals(action)) {
            stopSelf();
            return START_NOT_STICKY;
        }
        if (AmdlRunner.isRunning()) {
            Log.i(TAG, "a download is already running");
            return START_NOT_STICKY;
        }

        ArrayList<String> args = intent.getStringArrayListExtra(EXTRA_ARGS);
        String cwd = intent.getStringExtra(EXTRA_CWD);
        String dns = intent.getStringExtra(EXTRA_DNS);
        ArrayList<String> dirs = intent.getStringArrayListExtra(EXTRA_SCAN_DIRS);
        scanDirs = dirs == null ? new ArrayList<String>() : dirs;

        if (args == null || cwd == null) {
            Log.w(TAG, "missing arguments");
            stopSelf();
            return START_NOT_STICKY;
        }

        runStartedAt = System.currentTimeMillis();
        startForegroundSafely(Strings.get("service.preparing"));
        AmdlRunner.start(this, args, new File(cwd), dns);

        if (!AmdlRunner.isRunning()) {
            stopForegroundCompat();
            stopSelf();
            return START_NOT_STICKY;
        }
        return START_NOT_STICKY;
    }

    @Override
    public void onLogLine(String line) {
        if (line == null || line.trim().isEmpty()) {
            return;
        }
        lastLine = line.trim();
        long now = System.currentTimeMillis();
        if (now - lastNotifiedAt > 900) {
            lastNotifiedAt = now;
            notifyText(lastLine);
        }
    }

    /**
     * Progress-bar frames: surface them in the notification too, otherwise a
     * backgrounded download looks stalled between tracks.
     */
    @Override
    public void onTransient(String line) {
        if (line == null || line.trim().isEmpty()) {
            return;
        }
        long now = System.currentTimeMillis();
        if (now - lastNotifiedAt > 900) {
            lastNotifiedAt = now;
            notifyText(line.trim());
        }
    }

    @Override
    public void onFinished(int exitCode, boolean cancelled) {
        // A zero exit status does not mean "downloaded something": the CLI also
        // exits 0 when the queue produced no files. Combine both signals.
        int errors = AmdlRunner.summaryErrors();
        int completed = AmdlRunner.summaryCompleted();
        boolean ok = exitCode == 0 && !cancelled && errors <= 0 && completed > 0;

        // 核心直接写进配置里的目录，所以这里只做媒体库扫描。
        if (ok) {
            Thread scanThread = new Thread(new Runnable() {
                @Override
                public void run() {
                    int scanned = 0;
                    for (int i = 0; i < scanDirs.size(); i++) {
                        scanned += Exporter.scanMedia(new File(scanDirs.get(i)));
                    }
                    Log.i(TAG, "scanned " + scanned + " files");
                    if (prefs().alacCheck()) {
                        runAlacCheck();
                    }
                    notifyText(describeResult(0, false, true));
                    finishService();
                }
            }, "amdl-scan");
            scanThread.setDaemon(true);
            scanThread.start();
            return;
        }
        notifyText(describeResult(exitCode, cancelled, ok));
        finishService();
    }

    private String describeResult(int exitCode, boolean cancelled, boolean ok) {
        if (cancelled) {
            return Strings.get("result.cancelled");
        }
        int completed = AmdlRunner.summaryCompleted();
        int total = AmdlRunner.summaryTotal();
        int errors = AmdlRunner.summaryErrors();
        if (total > 0) {
            return Strings.get("result.done", completed, total, Math.max(errors, 0));
        }
        if (ok) {
            return Strings.get("service.result.done");
        }
        return Strings.get("service.result.empty", exitCode);
    }

    private Prefs prefs() {
        return new Prefs(this);
    }

    /**
     * 下载完成后检查这批 ALAC 是否完好：整首解码丢到 /dev/null，捕捉解码器报错。
     * 与alac.sh同样的判定，只是换成 App 内置的 ffmpeg，且只看本次下载的文件。
     */
    private void runAlacCheck() {
        final String ffmpeg = ConfigWriter.realFFmpegPath(this);
        AmdlRunner.notifyLine(Strings.get("alac.start"));
        AlacCheck.Result result = AlacCheck.scan(ffmpeg, scanDirs, runStartedAt,
                new AlacCheck.Sink() {
                    @Override
                    public void line(String text) {
                        AmdlRunner.notifyLine(text);
                    }
                });
        if (result.scanned == 0) {
            AmdlRunner.notifyLine(Strings.get("alac.none"));
        } else if (result.problems == 0) {
            AmdlRunner.notifyLine(Strings.get("alac.ok", result.scanned)
                    + (result.notAlac > 0
                            ? Strings.get("alac.ok.notalac", result.notAlac) : ""));
        } else {
            StringBuilder summary =
                    new StringBuilder(Strings.get("alac.bad", result.scanned, result.problems));
            if (result.truncated > 0) {
                summary.append(Strings.get("alac.bad.truncated", result.truncated));
            }
            if (result.skipped > 0) {
                summary.append(Strings.get("alac.bad.skipped", result.skipped));
            }
            AmdlRunner.notifyLine(summary.toString());
            for (int i = 0; i < result.problemFiles.size(); i++) {
                AmdlRunner.notifyLine(Strings.get("alac.bad.file", result.problemFiles.get(i)));
            }
        }
    }

    private void finishService() {
        stopForegroundCompat();
        stopSelf();
    }

    private void startForegroundSafely(String text) {
        Notification notification = buildNotification(text);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(NOTIFICATION_ID, notification,
                    ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
        } else {
            startForeground(NOTIFICATION_ID, notification);
        }
    }

    private void stopForegroundCompat() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            stopForeground(STOP_FOREGROUND_REMOVE);
        } else {
            stopForeground(true);
        }
    }

    private void notifyText(String text) {
        if (notificationManager == null) {
            return;
        }
        try {
            notificationManager.notify(NOTIFICATION_ID, buildNotification(text));
        } catch (Throwable e) {
            Log.w(TAG, "cannot update notification", e);
        }
    }

    private Notification buildNotification(String text) {
        Intent open = new Intent(this, MainActivity.class)
                .addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP);
        PendingIntent contentIntent = PendingIntent.getActivity(this, 0, open, pendingFlags());

        Intent stopIntent = new Intent(this, DownloadService.class).setAction(ACTION_STOP);
        PendingIntent stopPending = PendingIntent.getService(this, 1, stopIntent, pendingFlags());

        Notification.Builder builder = Build.VERSION.SDK_INT >= Build.VERSION_CODES.O
                ? new Notification.Builder(this, CHANNEL_ID)
                : new Notification.Builder(this);
        builder.setContentTitle(Strings.get("notify.title"))
                .setContentText(text)
                // Framework drawable: the app ships no drawable resources.
                .setSmallIcon(android.R.drawable.stat_sys_download)
                .setContentIntent(contentIntent)
                .setOngoing(AmdlRunner.isRunning())
                .setOnlyAlertOnce(true)
                .addAction(new Notification.Action.Builder(
                        Icon.createWithResource(this, android.R.drawable.stat_sys_download),
                        Strings.get("notify.stop"), stopPending).build())
                .setVisibility(Notification.VISIBILITY_PUBLIC);
        return builder.build();
    }

    private int pendingFlags() {
        int flags = PendingIntent.FLAG_UPDATE_CURRENT;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            flags |= PendingIntent.FLAG_IMMUTABLE;
        }
        return flags;
    }

    private void createChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O || notificationManager == null) {
            return;
        }
        NotificationChannel channel = new NotificationChannel(
                CHANNEL_ID, Strings.get("channel.name"), NotificationManager.IMPORTANCE_LOW);
        channel.setShowBadge(false);
        notificationManager.createNotificationChannel(channel);
    }

    @Override
    public void onDestroy() {
        AmdlRunner.removeListener(this);
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
