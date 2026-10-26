package com.amdl.nativeapp;

import android.app.Application;
import android.content.Context;

/** Installs crash reporting as early as possible and holds a static context. */
public class AmdlApp extends Application {

    @Override
    protected void attachBaseContext(Context base) {
        super.attachBaseContext(base);
        // Earliest hook available: catches failures during Application/Activity load.
        CrashLog.install(base);
    }

    @Override
    public void onCreate() {
        super.onCreate();
        CrashLog.install(this);
        Exporter.AppContext.init(this);
    }
}
