package com.amdl.nativeapp;

import android.Manifest;
import android.app.Activity;
import android.app.Dialog;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.Context;
import android.content.DialogInterface;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.content.res.Configuration;
import android.graphics.Typeface;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.text.Editable;
import android.text.InputType;
import android.text.SpannableStringBuilder;
import android.text.Spanned;
import android.text.TextUtils;
import android.text.TextWatcher;
import android.text.style.ForegroundColorSpan;
import android.text.style.StyleSpan;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.view.Window;
import android.widget.Button;
import android.widget.CheckBox;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.RadioButton;
import android.widget.RadioGroup;
import android.widget.ScrollView;
import android.widget.Switch;
import android.widget.TextView;
import android.widget.CompoundButton;
import android.widget.Toast;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.InputStream;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * Single screen: service address, quality, links, destination folder, live log.
 *
 * <p>Everything is built in code — no XML layouts, no AndroidX, no Material
 * library — so the start-up path stays tiny and cannot fail on resource
 * inflation. {@link Ui} supplies the Material 3 look.
 */
public class MainActivity extends Activity implements AmdlRunner.Listener {

    private static final int LOG_MIN_DP = 88;
    private static final int LOG_DEFAULT_DP = 150;
    private static final int REQ_IMPORT = 103;
    private static final int REQ_NOTIFICATIONS = 100;
    private static final int REQ_STORAGE = 101;

    private static final int SDK_TIRAMISU = 33;
    private static final int SDK_R = 30;
    private static final String PERM_POST_NOTIFICATIONS = "android.permission.POST_NOTIFICATIONS";
    private static final String ACTION_MANAGE_APP_ALL_FILES =
            "android.settings.MANAGE_APP_ALL_FILES_ACCESS_PERMISSION";
    private static final String ACTION_MANAGE_ALL_FILES =
            "android.settings.MANAGE_ALL_FILES_ACCESS_PERMISSION";
    private static final long PICK_PROMPT_COOLDOWN_MS = 1200;

    /** 配置面板里语言单选按钮的 id（2001-2003 已被主题开关占用）。 */
    private static final int LANG_ZH_ID = 3001;
    private static final int LANG_JA_ID = 3002;
    private static final int LANG_EN_ID = 3003;

    /** 省略号与间隔号：源码保持纯 ASCII，界面上仍是 … 与 ·。 */
    private static final String ELLIPSIS = "\u2026";
    private static final String MIDDOT = "\u00b7";

    private static final java.util.regex.Pattern URL_PATTERN =
            java.util.regex.Pattern.compile("https?://[^\\s<>]+");

    private Prefs prefs;
    private EditText edUrls;
    private TextView tvQueue;
    private TextView tvStatus;
    private TextView tvLog;
    private ScrollView scLog;
    private RadioGroup rgQuality;
    private Button btnStart;
    private Button btnStop;

    private final Handler main = new Handler(Looper.getMainLooper());
    private final StringBuilder logBuffer = new StringBuilder();
    /** 与 logBuffer 同步的带样式副本：按日志级别上色、横幅加粗。 */
    private final SpannableStringBuilder logStyled = new SpannableStringBuilder();

    /** Recent output, used to rebuild the track table the CLI printed. */
    private final List<String> recentLines = new ArrayList<String>();
    private boolean promptVisible;
    private int tableParseFrom;
    private LinearLayout promptBar;
    private EditText edPrompt;
    private TextView tvPromptHint;
    /** Self-overwriting line for progress-bar frames; never grows the log. */
    private TextView tvProgress;
    private TextView tvLogPlaceholder;
    private LinearLayout logCard;
    /** 日志卡片当前高度（dp），可拖动抓手调整，双击抓手收放到最小/上次高度。 */
    private int logHeightDp;
    private int logExpandedDp;
    private boolean logCollapsed;
    /** 当前显示的配置面板；切换语言后要先关掉，才能按新语言重建整个界面。 */
    private Dialog settingsDialog;

    private final AmdlRunner.PromptHandler promptHandler = new AmdlRunner.PromptHandler() {
        @Override
        public void onPrompt(String prompt) {
            showPromptBar();
        }
    };

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        try {
            prefs = new Prefs(this);
            // 先定语言，再建视图：所有文案都在 buildContentView() 里取。
            Strings.setLang(prefs.uiLanguage());
            Ui.apply(isDarkRequested());
            applySystemBars();
            setContentView(buildContentView());
            wireUp();
        } catch (Throwable error) {
            showFatalScreen(error);
        }
    }

    /** 跟随系统或按用户选择决定明暗；系统切换主题会重建 Activity，从而重新计算。 */
    private boolean isDarkRequested() {
        String mode = prefs.themeMode();
        if (Prefs.THEME_DARK.equals(mode)) {
            return true;
        }
        if (Prefs.THEME_LIGHT.equals(mode)) {
            return false;
        }
        int uiMode = getResources().getConfiguration().uiMode
                & Configuration.UI_MODE_NIGHT_MASK;
        return uiMode == Configuration.UI_MODE_NIGHT_YES;
    }

    /** 状态栏 / 导航栏跟界面同色，深色下用浅色图标。 */
    private void applySystemBars() {
        try {
            Window window = getWindow();
            window.setStatusBarColor(Ui.SURFACE);
            window.setNavigationBarColor(Ui.SURFACE);
            int flags = 0;
            if (!Ui.isDark()) {
                flags |= View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                    flags |= View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
                }
            }
            window.getDecorView().setSystemUiVisibility(flags);
        } catch (Throwable ignored) {
            // Purely cosmetic.
        }
    }

    /** Last resort: show what broke instead of letting the process die silently. */
    private void showFatalScreen(Throwable error) {
        java.io.StringWriter sw = new java.io.StringWriter();
        error.printStackTrace(new java.io.PrintWriter(sw));
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setBackgroundColor(Ui.SURFACE);
        root.setPadding(Ui.dp(this, 16), Ui.dp(this, 40), Ui.dp(this, 16), Ui.dp(this, 16));
        TextView title = Ui.text(this, Strings.get("fatal.title"), 20, Ui.ERROR);
        title.setTypeface(Typeface.create("sans-serif-medium", Typeface.NORMAL));
        TextView detail = Ui.text(this, sw.toString(), 10, Ui.ON_SURFACE_VARIANT);
        detail.setTypeface(Typeface.MONOSPACE);
        ScrollView scroll = new ScrollView(this);
        scroll.addView(detail);
        root.addView(title);
        root.addView(scroll, new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f));
        setContentView(root);
    }

    // ------------------------------------------------------------------ views

    private View buildContentView() {
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setBackgroundColor(Ui.SURFACE);
        root.setFitsSystemWindows(true);

        buildAppBar(root);

        ScrollView configScroll = new ScrollView(this);
        LinearLayout form = new LinearLayout(this);
        form.setOrientation(LinearLayout.VERTICAL);
        form.setPadding(Ui.dp(this, 16), Ui.dp(this, 4), Ui.dp(this, 16), Ui.dp(this, 4));
        configScroll.addView(form);

        buildQualityCard(form);
        buildLinkCard(form);

        // 上半部可滚动，日志固定在下方并占据剩余空间。
        LinearLayout.LayoutParams formParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f);
        root.addView(configScroll, formParams);

        buildActions(root);
        buildStatusAndLog(root);
        return root;
    }

    /** MD3 top app bar: title left, secondary actions right. */
    private void buildAppBar(LinearLayout root) {
        LinearLayout appBar = new LinearLayout(this);
        appBar.setOrientation(LinearLayout.HORIZONTAL);
        appBar.setGravity(Gravity.CENTER_VERTICAL);
        appBar.setPadding(Ui.dp(this, 20), Ui.dp(this, 12), Ui.dp(this, 12), Ui.dp(this, 4));

        LinearLayout titles = new LinearLayout(this);
        titles.setOrientation(LinearLayout.VERTICAL);
        titles.addView(Ui.appBarTitle(this, Strings.get("app.title")));
        TextView subtitle = Ui.text(this, Strings.get("app.subtitle"), 11,
                Ui.ON_SURFACE_VARIANT);
        subtitle.setPadding(0, Ui.dp(this, 2), 0, 0);
        titles.addView(subtitle);
        appBar.addView(titles, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));

        Button btnInspect = Ui.textButton(this, Strings.get("app.inspect"));
        btnInspect.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                startDownload(true);
            }
        });
        Button btnConfig = Ui.textButton(this, Strings.get("app.settings"));
        btnConfig.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                showSettingsDialog();
            }
        });
        appBar.addView(btnInspect);
        appBar.addView(btnConfig);
        root.addView(appBar);
    }

    private void buildQualityCard(LinearLayout form) {
        LinearLayout card = Ui.stackedCard(this);
        card.addView(Ui.cardTitle(this, Strings.get("quality.title")));
        rgQuality = new RadioGroup(this);
        rgQuality.setOrientation(RadioGroup.VERTICAL);
        rgQuality.setPadding(0, Ui.dp(this, 4), 0, 0);
        RadioButton alac = Ui.radio(this, Strings.get("quality.alac"));
        alac.setId(1001);
        RadioButton atmos = Ui.radio(this, Strings.get("quality.atmos"));
        atmos.setId(1002);
        RadioButton aac = Ui.radio(this, Strings.get("quality.aac"));
        aac.setId(1003);
        rgQuality.addView(alac);
        rgQuality.addView(atmos);
        rgQuality.addView(aac);
        rgQuality.check(1001);
        card.addView(rgQuality);
        form.addView(card);
    }

    private void buildLinkCard(LinearLayout form) {
        LinearLayout card = Ui.stackedCard(this);
        card.addView(Ui.cardTitle(this, Strings.get("link.title")));

        TextView hint = Ui.text(this, Strings.get("link.hint"), 12, Ui.ON_SURFACE_VARIANT);
        hint.setPadding(0, Ui.dp(this, 4), 0, Ui.dp(this, 12));
        card.addView(hint);

        Ui.Field field = Ui.field(this, Strings.get("link.field"),
                "https://music.apple.com/us/album/" + ELLIPSIS, true);
        edUrls = field.input;
        card.addView(field.container);

        LinearLayout row = new LinearLayout(this);
        row.setOrientation(LinearLayout.HORIZONTAL);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(0, Ui.dp(this, 4), 0, 0);
        tvQueue = Ui.text(this, Strings.get("link.queue.empty"), 12, Ui.ON_SURFACE_VARIANT);
        row.addView(tvQueue, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        Button btnImport = Ui.textButton(this, Strings.get("link.import"));
        btnImport.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                importLinkFile();
            }
        });
        row.addView(btnImport);
        card.addView(row);

        LinearLayout selectRow = new LinearLayout(this);
        selectRow.setOrientation(LinearLayout.HORIZONTAL);
        selectRow.setGravity(Gravity.CENTER_VERTICAL);
        selectRow.setPadding(0, Ui.dp(this, 10), 0, 0);
        TextView selectLabel = Ui.text(this, Strings.get("link.select.label"), 14, Ui.ON_SURFACE);
        selectRow.addView(selectLabel, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        Switch selectSwitch = Ui.switchControl(this, prefs.selectTracks());
        selectSwitch.setOnCheckedChangeListener(new CompoundButton.OnCheckedChangeListener() {
            @Override
            public void onCheckedChanged(CompoundButton buttonView, boolean isChecked) {
                prefs.setSelectTracks(isChecked);
                toast(isChecked
                        ? Strings.get("link.select.on")
                        : Strings.get("link.select.off"));
            }
        });
        selectRow.addView(selectSwitch);
        card.addView(selectRow);

        edUrls.addTextChangedListener(new TextWatcher() {
            @Override
            public void beforeTextChanged(CharSequence s, int start, int count, int after) {
            }

            @Override
            public void onTextChanged(CharSequence s, int start, int before, int count) {
            }

            @Override
            public void afterTextChanged(Editable s) {
                updateQueueCount();
            }
        });
        form.addView(card);
    }

    /**
     * 日志高度抓手：上下拖动调整日志卡片高度，单击在最小高度与上次高度之间切换。
     * 手指移动不超过 touchSlop 才算单击，避免拖动结束时误触发。
     */
    private void buildLogHandle(LinearLayout root) {
        LinearLayout touchArea = new LinearLayout(this);
        touchArea.setOrientation(LinearLayout.VERTICAL);
        touchArea.setGravity(Gravity.CENTER);
        touchArea.setPadding(0, Ui.dp(this, 10), 0, Ui.dp(this, 10));
        touchArea.setBackground(Ui.ripple(
                Ui.rounded(android.graphics.Color.TRANSPARENT, 8, this), 8, this));

        View grip = new View(this);
        grip.setBackground(Ui.rounded(Ui.OUTLINE_VARIANT, 2, this));
        touchArea.addView(grip, new LinearLayout.LayoutParams(
                Ui.dp(this, 36), Ui.dp(this, 4)));

        final int slop = android.view.ViewConfiguration.get(this).getScaledTouchSlop();
        touchArea.setOnTouchListener(new View.OnTouchListener() {
            private float startY;
            private int startHeightDp;
            private boolean dragging;

            @Override
            public boolean onTouch(View v, android.view.MotionEvent event) {
                switch (event.getActionMasked()) {
                    case android.view.MotionEvent.ACTION_DOWN:
                        startY = event.getRawY();
                        startHeightDp = logHeightDp;
                        dragging = false;
                        return true;
                    case android.view.MotionEvent.ACTION_MOVE:
                        float delta = startY - event.getRawY();
                        if (!dragging && Math.abs(delta) > slop) {
                            dragging = true;
                        }
                        if (dragging) {
                            applyLogHeight(startHeightDp + Math.round(
                                    delta / getResources().getDisplayMetrics().density));
                        }
                        return true;
                    case android.view.MotionEvent.ACTION_UP:
                    case android.view.MotionEvent.ACTION_CANCEL:
                        if (!dragging) {
                            toggleLogHeight();
                        } else {
                            prefs.setLogHeightDp(logHeightDp);
                        }
                        return true;
                    default:
                        return false;
                }
            }
        });
        root.addView(touchArea, new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));
    }

    private int maxLogHeightDp() {
        int screenDp = getResources().getDisplayMetrics().heightPixels
                / Math.max(1, (int) getResources().getDisplayMetrics().density);
        int limit = (int) (screenDp * 0.55f);
        return Math.max(LOG_MIN_DP + 40, limit);
    }

    /** 应用新的日志高度（dp），内部已做上下限收敛。 */
    private void applyLogHeight(int dp) {
        int clamped = Math.max(LOG_MIN_DP, Math.min(maxLogHeightDp(), dp));
        if (clamped == logHeightDp || logCard == null) {
            return;
        }
        logHeightDp = clamped;
        LinearLayout.LayoutParams params =
                (LinearLayout.LayoutParams) logCard.getLayoutParams();
        params.height = Ui.dp(this, clamped);
        logCard.setLayoutParams(params);
        logCollapsed = clamped <= LOG_MIN_DP;
        if (!logCollapsed) {
            logExpandedDp = clamped;
        }
    }

    /** 单击抓手：折叠到最小高度，或回到折叠前的高度。 */
    private void toggleLogHeight() {
        if (logCollapsed) {
            applyLogHeight(Math.max(LOG_MIN_DP + 40, logExpandedDp));
        } else {
            logExpandedDp = logHeightDp;
            applyLogHeight(LOG_MIN_DP);
        }
        prefs.setLogHeightDp(logHeightDp);
    }

    private void buildActions(LinearLayout root) {
        LinearLayout primaryRow = new LinearLayout(this);
        primaryRow.setOrientation(LinearLayout.HORIZONTAL);
        primaryRow.setPadding(Ui.dp(this, 16), Ui.dp(this, 6), Ui.dp(this, 16), Ui.dp(this, 6));

        btnStart = Ui.filledButton(this, Strings.get("action.start"));
        btnStart.setLayoutParams(new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        btnStart.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                startDownload(false);
            }
        });

        btnStop = Ui.outlinedButton(this, Strings.get("action.stop"));
        btnStop.setEnabled(false);
        LinearLayout.LayoutParams stopParams = new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f);
        stopParams.setMarginStart(Ui.dp(this, 8));
        btnStop.setLayoutParams(stopParams);
        btnStop.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                DownloadService.stop(MainActivity.this);
                AmdlRunner.cancel();
            }
        });
        primaryRow.addView(btnStart);
        primaryRow.addView(btnStop);
        root.addView(primaryRow);
    }

    private void buildStatusAndLog(LinearLayout root) {
        LinearLayout header = new LinearLayout(this);
        header.setOrientation(LinearLayout.HORIZONTAL);
        header.setGravity(Gravity.CENTER_VERTICAL);
        header.setPadding(Ui.dp(this, 20), Ui.dp(this, 4), Ui.dp(this, 20), Ui.dp(this, 4));

        TextView logTitle = Ui.text(this, Strings.get("log.title"), 14, Ui.ON_SURFACE);
        logTitle.setTypeface(Typeface.create("sans-serif-medium", Typeface.NORMAL));
        header.addView(logTitle);

        tvStatus = Ui.text(this, Strings.get("status.ready"), 12, Ui.ON_SURFACE_VARIANT);
        tvStatus.setGravity(Gravity.END);
        header.addView(tvStatus, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        root.addView(header);

        buildLogHandle(root);

        logCard = new LinearLayout(this);
        logCard.setOrientation(LinearLayout.VERTICAL);
        logCard.setBackground(Ui.rounded(Ui.SURFACE_CONTAINER_HIGHEST, 12, this));
        logCard.setPadding(Ui.dp(this, 12), Ui.dp(this, 10), Ui.dp(this, 12), Ui.dp(this, 10));

        FrameLayout logArea = new FrameLayout(this);
        scLog = new ScrollView(this);
        tvLog = Ui.text(this, "", 11, Ui.ON_SURFACE);
        tvLog.setTypeface(Typeface.MONOSPACE);
        tvLog.setTextIsSelectable(true);
        tvLog.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                showLogMenu();
            }
        });
        scLog.addView(tvLog);
        logArea.addView(scLog, new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));

        tvLogPlaceholder = Ui.text(this, Strings.get("log.placeholder"), 12,
                Ui.ON_SURFACE_VARIANT);
        tvLogPlaceholder.setGravity(Gravity.CENTER);
        FrameLayout.LayoutParams placeholderParams = new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT);
        placeholderParams.gravity = Gravity.CENTER;
        logArea.addView(tvLogPlaceholder, placeholderParams);
        logCard.addView(logArea, new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f));

        tvProgress = Ui.text(this, "", 11, Ui.ON_SURFACE_VARIANT);
        tvProgress.setTypeface(Typeface.MONOSPACE);
        tvProgress.setSingleLine(true);
        tvProgress.setEllipsize(android.text.TextUtils.TruncateAt.END);
        tvProgress.setPadding(0, Ui.dp(this, 6), 0, 0);
        tvProgress.setVisibility(View.GONE);
        logCard.addView(tvProgress);

        int savedHeight = prefs.logHeightDp();
        logHeightDp = Math.max(LOG_MIN_DP, savedHeight > 0 ? savedHeight : LOG_DEFAULT_DP);
        logExpandedDp = logHeightDp;
        logCollapsed = logHeightDp <= LOG_MIN_DP;
        LinearLayout.LayoutParams logParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, Ui.dp(this, logHeightDp));
        logParams.setMargins(Ui.dp(this, 16), 0, Ui.dp(this, 16), Ui.dp(this, 8));
        root.addView(logCard, logParams);
        buildPromptBar(root);
    }

    /**
     * Docked input bar for the CLI's track-selection prompt.
     *
     * <p>Deliberately not a dialog: the track numbers live in the log above, and a
     * modal window would cover exactly what the user needs to read.
     */
    private void buildPromptBar(LinearLayout root) {
        promptBar = new LinearLayout(this);
        promptBar.setOrientation(LinearLayout.VERTICAL);
        promptBar.setBackground(Ui.rounded(Ui.SURFACE_CONTAINER, 12, this));
        promptBar.setPadding(Ui.dp(this, 12), Ui.dp(this, 10), Ui.dp(this, 12), Ui.dp(this, 10));
        promptBar.setVisibility(View.GONE);

        tvPromptHint = Ui.text(this, "", 12, Ui.ON_SURFACE_VARIANT);
        promptBar.addView(tvPromptHint);

        LinearLayout inputRow = new LinearLayout(this);
        inputRow.setOrientation(LinearLayout.HORIZONTAL);
        inputRow.setGravity(Gravity.CENTER_VERTICAL);
        inputRow.setPadding(0, Ui.dp(this, 8), 0, 0);

        Ui.Field field = Ui.field(this, null, Strings.get("prompt.field.hint"), false);
        edPrompt = field.input;
        edPrompt.setInputType(InputType.TYPE_CLASS_TEXT);
        field.input.setImeOptions(android.view.inputmethod.EditorInfo.IME_ACTION_DONE);
        inputRow.addView(field.container, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));

        Button submit = Ui.filledButton(this, Strings.get("common.ok"));
        LinearLayout.LayoutParams submitParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        submitParams.setMarginStart(Ui.dp(this, 8));
        submit.setLayoutParams(submitParams);
        submit.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                submitSelection(edPrompt.getText().toString());
            }
        });
        inputRow.addView(submit);
        promptBar.addView(inputRow);

        LinearLayout actionRow = new LinearLayout(this);
        actionRow.setOrientation(LinearLayout.HORIZONTAL);
        actionRow.setGravity(Gravity.END);

        Button all = Ui.textButton(this, Strings.get("prompt.all"));
        all.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                submitSelection("all");
            }
        });
        Button skip = Ui.textButton(this, Strings.get("prompt.skip"));
        skip.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                sendSelection("");
            }
        });
        actionRow.addView(all);
        actionRow.addView(skip);
        promptBar.addView(actionRow);

        LinearLayout.LayoutParams barParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        barParams.setMargins(Ui.dp(this, 16), 0, Ui.dp(this, 16), Ui.dp(this, 8));
        root.addView(promptBar, barParams);
    }

    private void wireUp() {
        edUrls.setText(prefs.lastUrls());
        selectQuality(prefs.quality());

        restoreLog();
        String previousCrash = CrashLog.read(this);
        if (!previousCrash.trim().isEmpty()) {
            appendLine(Strings.get("log.crash.header"));
            appendLine(previousCrash.trim());
            appendLine(Strings.get("log.crash.footer"));
            CrashLog.clear(this);
        }
        updateRunningUi(AmdlRunner.isRunning());
        updateQueueCount();
        requestNotificationPermissionIfNeeded();
        warnAboutMissingBinary();
    }

    @Override
    protected void onStart() {
        super.onStart();
        AmdlRunner.addListener(this);
        AmdlRunner.setPromptHandler(promptHandler);
        updateRunningUi(AmdlRunner.isRunning());
    }

    @Override
    protected void onStop() {
        AmdlRunner.removeListener(this);
        AmdlRunner.setPromptHandler(null);
        super.onStop();
        String urls = edUrls == null ? "" : edUrls.getText().toString();
        if (!urls.trim().isEmpty()) {
            prefs.setLastUrls(urls);
        }
    }

    @Override
    protected void onResume() {
        super.onResume();
        updateRunningUi(AmdlRunner.isRunning());
    }

    // ---------------------------------------------------------------- actions

    /**
     * Runs the CLI.
     *
     * @param inspectOnly true = only print audio-quality information
     *                    ({@code --debug}, like am.sh menu 4); nothing is
     *                    downloaded and no folder is required.
     */
    private void startDownload(boolean inspectOnly) {
        String lite = prefs.liteServer();
        if (lite == null || lite.trim().isEmpty()) {
            toast(Strings.get("err.lite.empty"));
            showSettingsDialog();
            return;
        }

        List<String> urls = parseUrls(edUrls.getText().toString());
        if (urls.isEmpty()) {
            toast(Strings.get("err.urls.empty"));
            edUrls.requestFocus();
            return;
        }
        for (int i = 0; i < urls.size(); i++) {
            String url = urls.get(i);
            if (!url.startsWith("http://") && !url.startsWith("https://")) {
                toast(Strings.get("err.url.invalid", url));
                return;
            }
        }
        if (AmdlRunner.binary(this) == null) {
            toast(Strings.get("err.binary.missing"));
            return;
        }

        if (!inspectOnly && !hasAllFilesAccess()) {
            askForStoragePermission();
            return;
        }

        if (!inspectOnly) {
            prefs.setQuality(currentQuality());
        }
        prefs.setLastUrls(edUrls.getText().toString());

        // 核心只认真实路径：四个目录都来自“配置”，这里先确保存在。
        ArrayList<String> scanDirs = new ArrayList<String>();
        String[] configured = new String[]{
                prefs.dirAlac(), prefs.dirAtmos(), prefs.dirAac(), prefs.dirMv()};
        File cwd = null;
        for (int i = 0; i < configured.length; i++) {
            String path = configured[i] == null ? "" : configured[i].trim();
            if (path.isEmpty()) {
                continue;
            }
            if (!path.startsWith("/")) {
                toast(Strings.get("err.path.absolute", path));
                showSettingsDialog();
                return;
            }
            File dir = new File(path);
            if (!dir.exists() && !dir.mkdirs()) {
                toast(Strings.get("err.dir.create", path));
                return;
            }
            scanDirs.add(dir.getAbsolutePath());
            if (cwd == null) {
                cwd = dir;
            }
        }
        if (cwd == null) {
            toast(Strings.get("err.dir.empty"));
            showSettingsDialog();
            return;
        }

        ArrayList<String> args = new ArrayList<String>();
        try {
            File config = ConfigWriter.write(this, new File(getFilesDir(), "config.yaml"), prefs);
            args.add("-c");
            args.add(config.getAbsolutePath());
        } catch (Exception e) {
            toast(Strings.get("err.config.write", e.getMessage()));
            return;
        }

        if (inspectOnly) {
            args.add("--debug");
        } else {
            String quality = currentQuality();
            if (Prefs.QUALITY_ATMOS.equals(quality)) {
                args.add("--atmos");
            } else if (Prefs.QUALITY_AAC.equals(quality)) {
                args.add("--aac");
            }
            if (prefs.saveM3u8()) {
                args.add("--save-m3u8-playlist");
            }
            if (prefs.selectTracks()) {
                args.add("--select");
            }
        }

        List<String> extra = splitArgs(prefs.extraArgs());
        for (int i = 0; i < extra.size(); i++) {
            args.add(extra.get(i));
        }
        for (int i = 0; i < urls.size(); i++) {
            args.add(urls.get(i));
        }

        clearLogView();
        DownloadService.start(this, args, cwd, prefs.dns(), scanDirs);
        updateRunningUi(true);
        if (inspectOnly) {
            tvStatus.setText(Strings.get("status.inspecting"));
        } else {
            tvStatus.setText(Strings.get("status.started", cwd.getAbsolutePath()));
        }
    }

    /** 没有“所有文件访问权限”时无法写到 /storage 下的真实路径。 */
    private void askForStoragePermission() {
        Ui.dialog(this, Strings.get("dialog.perm.title"),
                Ui.text(this, Strings.get("dialog.perm.body", Prefs.DEFAULT_MUSIC_ROOT),
                        13, Ui.ON_SURFACE),
                Strings.get("dialog.perm.grant"), new Runnable() {
                    @Override
                    public void run() {
                        requestAllFilesAccess();
                    }
                }, Strings.get("common.cancel"), null).show();
    }

    private void testLiteServer(String base, final TextView statusView) {
        if (base == null || base.trim().isEmpty()) {
            toast(Strings.get("server.empty"));
            return;
        }
        base = base.trim();
        statusView.setText(Strings.get("server.testing"));
        statusView.setTextColor(Ui.ON_SURFACE_VARIANT);
        final String url = base.replaceAll("/+$", "") + "/status";
        Thread thread = new Thread(new Runnable() {
            @Override
            public void run() {
                String message;
                boolean ok = false;
                java.net.HttpURLConnection conn = null;
                try {
                    conn = (java.net.HttpURLConnection) new java.net.URL(url).openConnection();
                    conn.setConnectTimeout(6000);
                    conn.setReadTimeout(6000);
                    conn.setRequestMethod("GET");
                    int code = conn.getResponseCode();
                    InputStream in = code >= 400 ? conn.getErrorStream() : conn.getInputStream();
                    String body = readAll(in);
                    if (code == 200) {
                        ok = true;
                        message = Strings.get("server.connected", trimForUi(body, 120));
                    } else {
                        message = "HTTP " + code + " " + MIDDOT + " " + trimForUi(body, 100);
                    }
                } catch (Exception e) {
                    message = Strings.get("server.failed", e.getMessage());
                } finally {
                    if (conn != null) {
                        conn.disconnect();
                    }
                }
                final boolean success = ok;
                final String text = message;
                main.post(new Runnable() {
                    @Override
                    public void run() {
                        statusView.setText(text);
                        statusView.setTextColor(success ? Ui.SUCCESS : Ui.ERROR);
                    }
                });
            }
        }, "lite-status");
        thread.start();
    }

    // ------------------------------------------------------------------ 权限

    private boolean hasAllFilesAccess() {
        if (Build.VERSION.SDK_INT < SDK_R) {
            return checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE)
                    == PackageManager.PERMISSION_GRANTED;
        }
        try {
            java.lang.reflect.Method method =
                    android.os.Environment.class.getMethod("isExternalStorageManager");
            Object result = method.invoke(null);
            return result instanceof Boolean && ((Boolean) result).booleanValue();
        } catch (Throwable ignored) {
            return false;
        }
    }

    private void requestAllFilesAccess() {
        if (Build.VERSION.SDK_INT < SDK_R) {
            requestPermissions(new String[]{Manifest.permission.WRITE_EXTERNAL_STORAGE}, REQ_STORAGE);
            return;
        }
        try {
            Intent intent = new Intent(ACTION_MANAGE_APP_ALL_FILES);
            intent.setData(Uri.parse("package:" + getPackageName()));
            startActivity(intent);
        } catch (Exception e) {
            try {
                startActivity(new Intent(ACTION_MANAGE_ALL_FILES));
            } catch (Exception e2) {
                toast(Strings.get("err.perm.guide"));
            }
        }
    }

    // ------------------------------------------------------------ 批量导入

    private void importLinkFile() {
        try {
            Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
            intent.addCategory(Intent.CATEGORY_OPENABLE);
            intent.setType("text/*");
            startActivityForResult(intent, REQ_IMPORT);
        } catch (Exception e) {
            toast(Strings.get("err.pick.unsupported", e.getMessage()));
        }
    }

    private void readImportedFile(Uri uri) {
        if (uri == null) {
            return;
        }
        String body;
        try {
            InputStream in = getContentResolver().openInputStream(uri);
            body = readAll(in);
        } catch (Exception e) {
            toast(Strings.get("err.read.file", e.getMessage()));
            return;
        }
        List<String> found = extractUrls(body);
        if (found.isEmpty()) {
            toast(Strings.get("err.import.empty"));
            return;
        }
        List<String> existing = parseUrls(edUrls.getText().toString());
        StringBuilder text = new StringBuilder();
        for (int i = 0; i < existing.size(); i++) {
            if (text.length() > 0) {
                text.append('\n');
            }
            text.append(existing.get(i));
        }
        int added = 0;
        for (int i = 0; i < found.size(); i++) {
            String url = found.get(i);
            if (!existing.contains(url)) {
                existing.add(url);
                if (text.length() > 0) {
                    text.append('\n');
                }
                text.append(url);
                added++;
            }
        }
        edUrls.setText(text.toString());
        toast(Strings.get("import.done", added, existing.size()));
    }

    /** Pulls every http(s) URL out of a text file, de-duplicated. */
    static List<String> extractUrls(String body) {
        List<String> out = new ArrayList<String>();
        if (body == null || body.isEmpty()) {
            return out;
        }
        java.util.regex.Matcher matcher = URL_PATTERN.matcher(body);
        while (matcher.find()) {
            String url = matcher.group();
            while (url.length() > 0 && isLinkTail(url.charAt(url.length() - 1))) {
                url = url.substring(0, url.length() - 1);
            }
            if (url.length() > 8 && !out.contains(url)) {
                out.add(url);
            }
        }
        return out;
    }

    /** Characters that end a link when it was copied out of prose. */
    private static boolean isLinkTail(char c) {
        return c == '\u0022' || c == '\'' || c == ')' || c == ']' || c == '}'
                || c == ',' || c == ';' || c == '.'
                || c == '\u3002' || c == '\uff0c' || c == '\uff1b' || c == '\u3001'
                || c == '\u201d' || c == '\u2019' || c == '\u00bb';
    }

    private void updateQueueCount() {
        if (tvQueue == null) {
            return;
        }
        int count = parseUrls(edUrls.getText().toString()).size();
        tvQueue.setText(count == 0
                ? Strings.get("link.queue.empty")
                : Strings.get("link.queue.count", count));
        tvQueue.setTextColor(count == 0 ? Ui.ON_SURFACE_VARIANT : Ui.PRIMARY);
    }

    // ------------------------------------------------------------ 配置面板

    private void showSettingsDialog() {
        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);

        content.addView(sectionHeader(Strings.get("settings.language.title"), false));
        content.addView(buildLanguagePicker());

        content.addView(sectionHeader(Strings.get("settings.section.appearance"), false));
        final RadioGroup rgTheme = new RadioGroup(this);
        rgTheme.setOrientation(RadioGroup.VERTICAL);
        RadioButton rbSystem = Ui.radio(this, Strings.get("theme.system"));
        rbSystem.setId(2001);
        RadioButton rbLight = Ui.radio(this, Strings.get("theme.light"));
        rbLight.setId(2002);
        RadioButton rbDark = Ui.radio(this, Strings.get("theme.dark"));
        rbDark.setId(2003);
        rgTheme.addView(rbSystem);
        rgTheme.addView(rbLight);
        rgTheme.addView(rbDark);
        String themeMode = prefs.themeMode();
        if (Prefs.THEME_LIGHT.equals(themeMode)) {
            rgTheme.check(2002);
        } else if (Prefs.THEME_DARK.equals(themeMode)) {
            rgTheme.check(2003);
        } else {
            rgTheme.check(2001);
        }
        content.addView(rgTheme);

        content.addView(sectionHeader(Strings.get("settings.section.server"), true));
        final List<EditText> liteFields = new ArrayList<EditText>();
        final List<String> liteKeys = new ArrayList<String>();
        addField(content, liteFields, liteKeys, Prefs.K_LITE_SERVER,
        Strings.get("server.lite.label"),
                "http://192.168.1.10:12340");
        liteFields.get(0).setText(prefs.liteServer());

        LinearLayout testRow = new LinearLayout(this);
        testRow.setOrientation(LinearLayout.HORIZONTAL);
        testRow.setGravity(Gravity.CENTER_VERTICAL);
        final TextView tvTest = Ui.text(this, "", 12, Ui.ON_SURFACE_VARIANT);
        Button btnTest = Ui.textButton(this, Strings.get("server.test"));
        btnTest.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                testLiteServer(liteFields.get(0).getText().toString(), tvTest);
            }
        });
        testRow.addView(btnTest);
        testRow.addView(tvTest, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        content.addView(testRow);

        content.addView(sectionHeader(Strings.get("settings.section.paths"), true));
        final List<EditText> dirFields = new ArrayList<EditText>();
        final List<String> dirKeys = new ArrayList<String>();
        addField(content, dirFields, dirKeys, Prefs.K_DIR_ALAC, Strings.get("dir.alac"),
                Prefs.DEFAULT_MUSIC_ROOT + "/ALAC");
        addField(content, dirFields, dirKeys, Prefs.K_DIR_ATMOS, Strings.get("dir.atmos"),
                Prefs.DEFAULT_MUSIC_ROOT + "/Atmos");
        addField(content, dirFields, dirKeys, Prefs.K_DIR_AAC, "AAC",
                Prefs.DEFAULT_MUSIC_ROOT + "/AAC");
        addField(content, dirFields, dirKeys, Prefs.K_DIR_MV, "MV",
                Prefs.DEFAULT_MUSIC_ROOT + "/MV");
        dirFields.get(0).setText(prefs.dirAlac());
        dirFields.get(1).setText(prefs.dirAtmos());
        dirFields.get(2).setText(prefs.dirAac());
        dirFields.get(3).setText(prefs.dirMv());

        LinearLayout permRow = new LinearLayout(this);
        permRow.setOrientation(LinearLayout.HORIZONTAL);
        permRow.setGravity(Gravity.CENTER_VERTICAL);
        permRow.setPadding(0, Ui.dp(this, 4), 0, Ui.dp(this, 4));
        final TextView tvPerm = Ui.text(this,
                hasAllFilesAccess() ? Strings.get("perm.granted") : Strings.get("perm.denied"),
                12, hasAllFilesAccess() ? Ui.SUCCESS : Ui.ERROR);
        Button btnPerm = Ui.textButton(this, Strings.get("perm.request"));
        btnPerm.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                requestAllFilesAccess();
            }
        });
        permRow.addView(btnPerm);
        permRow.addView(tvPerm, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        content.addView(permRow);

        content.addView(sectionHeader(Strings.get("settings.section.account"), true));
        final List<EditText> fields = new ArrayList<EditText>();
        final List<String> fieldKeys = new ArrayList<String>();
        addField(content, fields, fieldKeys, Prefs.K_STOREFRONT,
        Strings.get("account.storefront"), "us");
        addField(content, fields, fieldKeys, Prefs.K_LANGUAGE,
        Strings.get("account.lyricslang"), "zh-Hans-CN");
        addField(content, fields, fieldKeys, Prefs.K_PROXY,
        Strings.get("account.proxy"), "http://127.0.0.1:7890");
        addField(content, fields, fieldKeys, Prefs.K_MEDIA_TOKEN,
        Strings.get("account.mtoken"), "");
        addField(content, fields, fieldKeys, Prefs.K_AUTH_TOKEN, Strings.get("account.atoken"), "");
        addField(content, fields, fieldKeys, Prefs.K_DNS,
        Strings.get("account.dns"), "223.5.5.5, 8.8.8.8");
        addField(content, fields, fieldKeys, Prefs.K_MAX_MEMORY,
        Strings.get("account.maxmem"), "256");

        content.addView(sectionHeader(Strings.get("settings.section.quality"), true));
        addField(content, fields, fieldKeys, Prefs.K_ALAC_MAX,
        Strings.get("quality.alacmax"), "192000");
        addField(content, fields, fieldKeys, Prefs.K_ATMOS_MAX,
        Strings.get("quality.atmosmax"), "2768");
        addField(content, fields, fieldKeys, Prefs.K_AAC_TYPE,
        Strings.get("quality.aactype"), "aac-lc");
        addField(content, fields, fieldKeys, Prefs.K_M3U8_MODE,
        Strings.get("quality.m3u8mode"), "hires");

        content.addView(sectionHeader(Strings.get("settings.section.lyrics"), true));
        addField(content, fields, fieldKeys, Prefs.K_LYRICS_TYPE,
        Strings.get("lyrics.type"), "lyrics");
        addField(content, fields, fieldKeys, Prefs.K_LYRICS_FORMAT,
        Strings.get("lyrics.format"), "lrc");
        addField(content, fields, fieldKeys, Prefs.K_LYRICS_EXTRA,
        Strings.get("lyrics.extra"), "translation");

        content.addView(sectionHeader(Strings.get("settings.section.artwork"), true));
        addField(content, fields, fieldKeys, Prefs.K_COVER_SIZE,
        Strings.get("artwork.size"), "3000x3000");
        addField(content, fields, fieldKeys, Prefs.K_COVER_FORMAT,
        Strings.get("artwork.format"), "jpg");

        content.addView(sectionHeader(Strings.get("settings.section.naming"), true));
        addField(content, fields, fieldKeys, Prefs.K_ALBUM_TPL,
        Strings.get("naming.album"), "{AlbumName}");
        addField(content, fields, fieldKeys, Prefs.K_PLAYLIST_TPL,
        Strings.get("naming.playlist"), "{PlaylistName}");
        addField(content, fields, fieldKeys, Prefs.K_SONG_TPL,
        Strings.get("naming.song"), "{SongNumer}. {SongName}");
        addField(content, fields, fieldKeys, Prefs.K_ARTIST_TPL,
        Strings.get("naming.artist"), "{UrlArtistName}");
        addField(content, fields, fieldKeys, Prefs.K_TAG_EXPLICIT,
        Strings.get("naming.explicit"), "[E]");
        addField(content, fields, fieldKeys, Prefs.K_TAG_CLEAN, Strings.get("naming.clean"), "[C]");
        addField(content, fields, fieldKeys, Prefs.K_TAG_MASTER,
        Strings.get("naming.master"), "[M]");
        addField(content, fields, fieldKeys, Prefs.K_LIMIT_MAX, Strings.get("naming.limit"), "200");

        content.addView(sectionHeader(Strings.get("settings.section.mv"), true));
        addField(content, fields, fieldKeys, Prefs.K_MV_AUDIO, Strings.get("mv.audio"), "atmos");
        addField(content, fields, fieldKeys, Prefs.K_MV_MAX, Strings.get("mv.max"), "2160");
        addField(content, fields, fieldKeys, Prefs.K_EXTRA_ARGS,
        Strings.get("mv.extraargs"), "--save-m3u8-playlist");

        final List<Switch> switches = new ArrayList<Switch>();
        final List<String> switchKeys = new ArrayList<String>();
        addSwitch(content, switches, switchKeys, Prefs.K_ALAC_FIX,
        Strings.get("settings.sw.alacfix"));
        addSwitch(content, switches, switchKeys, Prefs.K_LYRICS_SAVE,
        Strings.get("settings.sw.lyricssave"));
        addSwitch(content, switches, switchKeys, Prefs.K_LYRICS_EMBED,
        Strings.get("settings.sw.lyricsembed"));
        addSwitch(content, switches, switchKeys, Prefs.K_COVER_EMBED,
        Strings.get("settings.sw.coverembed"));
        addSwitch(content, switches, switchKeys, Prefs.K_COVER_ARTIST,
        Strings.get("settings.sw.coverartist"));
        addSwitch(content, switches, switchKeys, Prefs.K_TAG_SORT,
        Strings.get("settings.sw.tagsort"));
        addSwitch(content, switches, switchKeys, Prefs.K_TAG_ITUNES,
        Strings.get("settings.sw.itunesid"));
        addSwitch(content, switches, switchKeys, Prefs.K_TAG_SONGINFO,
        Strings.get("settings.sw.songinfo"));
        addSwitch(content, switches, switchKeys, Prefs.K_ARTIST_FOLDER,
        Strings.get("settings.sw.artistfolder"));
        addSwitch(content, switches, switchKeys, Prefs.K_SAVE_M3U8,
        Strings.get("settings.sw.m3u8"));

        content.addView(sectionHeader(Strings.get("settings.section.convert"), true));
        String ffmpegPath = ConfigWriter.ffmpegPath(this);
        TextView convertNote = Ui.text(this, ffmpegPath == null
                        ? Strings.get("convert.missing")
                        : Strings.get("convert.note"),
                11, ffmpegPath == null ? Ui.ERROR : Ui.ON_SURFACE_VARIANT);
        convertNote.setPadding(0, 0, 0, Ui.dp(this, 8));
        content.addView(convertNote);

        final List<Switch> convertSwitches = new ArrayList<Switch>();
        LinearLayout convertRow = new LinearLayout(this);
        convertRow.setOrientation(LinearLayout.HORIZONTAL);
        convertRow.setGravity(Gravity.CENTER_VERTICAL);
        convertRow.setPadding(0, Ui.dp(this, 4), 0, Ui.dp(this, 4));
        TextView convertLabel = Ui.text(this, Strings.get("convert.enable"), 14, Ui.ON_SURFACE);
        convertRow.addView(convertLabel, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        final Switch convertSwitch = Ui.switchControl(this, prefs.convertEnabled());
        convertSwitch.setEnabled(ffmpegPath != null);
        convertRow.addView(convertSwitch);
        content.addView(convertRow);

        final List<EditText> convertFields = new ArrayList<EditText>();
        final List<String> convertKeys = new ArrayList<String>();
        addField(content, convertFields, convertKeys, Prefs.K_CONVERT_FORMAT,
                Strings.get("convert.format"), "flac");

        LinearLayout keepRow = new LinearLayout(this);
        keepRow.setOrientation(LinearLayout.HORIZONTAL);
        keepRow.setGravity(Gravity.CENTER_VERTICAL);
        keepRow.setPadding(0, Ui.dp(this, 4), 0, Ui.dp(this, 4));
        TextView keepLabel = Ui.text(this, Strings.get("convert.keep"), 14, Ui.ON_SURFACE);
        keepRow.addView(keepLabel, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        final Switch keepSwitch = Ui.switchControl(this, prefs.convertKeepOriginal());
        keepRow.addView(keepSwitch);
        content.addView(keepRow);

        LinearLayout alacRow = new LinearLayout(this);
        alacRow.setOrientation(LinearLayout.HORIZONTAL);
        alacRow.setGravity(Gravity.CENTER_VERTICAL);
        alacRow.setPadding(0, Ui.dp(this, 4), 0, Ui.dp(this, 4));
        TextView alacLabel = Ui.text(this, Strings.get("alaccheck.enable"), 14, Ui.ON_SURFACE);
        alacRow.addView(alacLabel, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        final Switch alacSwitch = Ui.switchControl(this, prefs.alacCheck());
        alacSwitch.setEnabled(ffmpegPath != null);
        alacRow.addView(alacSwitch);
        content.addView(alacRow);
        TextView alacNote = Ui.text(this, Strings.get("alaccheck.note"),
                11, Ui.ON_SURFACE_VARIANT);
        alacNote.setPadding(0, 0, 0, Ui.dp(this, 8));
        content.addView(alacNote);

        // 上游模板里有、但本 App 还没有专门控件的键：自动生成控件
        final List<EditText> extraFields = new ArrayList<EditText>();
        final List<String> extraPaths = new ArrayList<String>();
        final List<Switch> extraSwitches = new ArrayList<Switch>();
        final List<String> extraSwitchPaths = new ArrayList<String>();
        ConfigTemplate template = ConfigTemplate.load(this);
        if (template != null) {
            Map<String, String> known = ConfigKeys.values(prefs);
            known.putAll(ConfigKeys.forced());
            List<ConfigTemplate.Line> unknown = template.unknown(known);
            int total = template.entries().size();
            content.addView(sectionHeader(Strings.get("settings.section.extra"), true));
            TextView note = Ui.text(this, unknown.isEmpty()
                            ? Strings.get("settings.extra.all", total)
                            : Strings.get("settings.extra.some", total, unknown.size()),
                    11, Ui.ON_SURFACE_VARIANT);
            note.setPadding(0, 0, 0, Ui.dp(this, 8));
            content.addView(note);

            for (int i = 0; i < unknown.size(); i++) {
                ConfigTemplate.Line line = unknown.get(i);
                String current = prefs.extra(line.path, line.defaultValue);
                String caption = line.path + (line.comment.isEmpty() ? "" : "  " + line.comment);
                if ("true".equals(line.defaultValue) || "false".equals(line.defaultValue)) {
                    LinearLayout row = new LinearLayout(this);
                    row.setOrientation(LinearLayout.HORIZONTAL);
                    row.setGravity(Gravity.CENTER_VERTICAL);
                    row.setPadding(0, Ui.dp(this, 4), 0, Ui.dp(this, 4));
                    TextView label = Ui.text(this, caption, 12, Ui.ON_SURFACE);
                    row.addView(label, new LinearLayout.LayoutParams(0,
                            ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
                    Switch toggle = Ui.switchControl(this, "true".equals(current));
                    row.addView(toggle);
                    content.addView(row);
                    extraSwitches.add(toggle);
                    extraSwitchPaths.add(line.path);
                } else {
                    addField(content, extraFields, extraPaths, Prefs.EXTRA_PREFIX + line.path,
                            caption, line.defaultValue);
                }
            }
        }

        Dialog dialog = Ui.dialog(this, Strings.get("settings.title"), content,
                Strings.get("common.save"), new Runnable() {
            @Override
            public void run() {
                for (int i = 0; i < fields.size(); i++) {
                    prefs.setStr(fieldKeys.get(i), fields.get(i).getText().toString());
                }
                for (int i = 0; i < liteFields.size(); i++) {
                    prefs.setStr(liteKeys.get(i), liteFields.get(i).getText().toString().trim());
                }
                for (int i = 0; i < dirFields.size(); i++) {
                    prefs.setStr(dirKeys.get(i), dirFields.get(i).getText().toString().trim());
                }
                for (int i = 0; i < extraFields.size(); i++) {
                    prefs.setStr(extraPaths.get(i),
                            extraFields.get(i).getText().toString().trim());
                }
                prefs.setBool(Prefs.K_CONVERT_ENABLED, convertSwitch.isChecked());
                prefs.setBool(Prefs.K_CONVERT_KEEP, keepSwitch.isChecked());
                prefs.setBool(Prefs.K_ALAC_CHECK, alacSwitch.isChecked());
                for (int i = 0; i < convertFields.size(); i++) {
                    prefs.setStr(convertKeys.get(i),
                            convertFields.get(i).getText().toString().trim());
                }
                for (int i = 0; i < extraSwitches.size(); i++) {
                    prefs.setExtra(extraSwitchPaths.get(i),
                            String.valueOf(extraSwitches.get(i).isChecked()));
                }
                for (int i = 0; i < switches.size(); i++) {
                    prefs.setBool(switchKeys.get(i), switches.get(i).isChecked());
                }
                String selected = Prefs.THEME_SYSTEM;
                int themeId = rgTheme.getCheckedRadioButtonId();
                if (themeId == 2002) {
                    selected = Prefs.THEME_LIGHT;
                } else if (themeId == 2003) {
                    selected = Prefs.THEME_DARK;
                }
                boolean themeChanged = !selected.equals(prefs.themeMode());
                prefs.setThemeMode(selected);
                if (themeChanged) {
                    toast(Strings.get("settings.theme.changed"));
                    recreate();
                    return;
                }
                toast(Strings.get("settings.saved"));
            }
        }, Strings.get("common.cancel"), null);
        settingsDialog = dialog;
        dialog.setOnDismissListener(new DialogInterface.OnDismissListener() {
            @Override
            public void onDismiss(DialogInterface dismissed) {
                if (settingsDialog == dismissed) {
                    settingsDialog = null;
                }
            }
        });
        dialog.show();
    }

    /** 单选框里的当前语言：只显示语言自己的名字，不做翻译。 */
    private RadioGroup buildLanguagePicker() {
        RadioGroup group = new RadioGroup(this);
        group.setOrientation(RadioGroup.VERTICAL);
        RadioButton zh = Ui.radio(this, Strings.get("lang.name.zh"));
        zh.setId(LANG_ZH_ID);
        RadioButton ja = Ui.radio(this, Strings.get("lang.name.ja"));
        ja.setId(LANG_JA_ID);
        RadioButton en = Ui.radio(this, Strings.get("lang.name.en"));
        en.setId(LANG_EN_ID);
        group.addView(zh);
        group.addView(ja);
        group.addView(en);
        // 先勾选再挂监听，避免初始化时误触发一次切换。
        group.check(languageRadioId(Strings.lang()));
        group.setOnCheckedChangeListener(new RadioGroup.OnCheckedChangeListener() {
            @Override
            public void onCheckedChanged(RadioGroup radioGroup, int checkedId) {
                switchLanguage(languageTag(checkedId));
            }
        });
        return group;
    }

    private static int languageRadioId(String tag) {
        if (Strings.JA.equals(tag)) {
            return LANG_JA_ID;
        }
        if (Strings.EN.equals(tag)) {
            return LANG_EN_ID;
        }
        return LANG_ZH_ID;
    }

    private static String languageTag(int radioId) {
        if (radioId == LANG_JA_ID) {
            return Strings.JA;
        }
        if (radioId == LANG_EN_ID) {
            return Strings.EN;
        }
        return Strings.ZH;
    }

    /** 语言自己的名字（简体中文 / 日本語 / English），三种界面语言下都相同。 */
    private static String languageName(String tag) {
        if (Strings.JA.equals(tag)) {
            return Strings.get("lang.name.ja");
        }
        if (Strings.EN.equals(tag)) {
            return Strings.get("lang.name.en");
        }
        return Strings.get("lang.name.zh");
    }

    /**
     * 切换界面语言：写进 Prefs（ConfigWriter 会同步到 config.yaml 的
     * general.ui-language，核心日志跟着换语言），然后就地重建界面，不重启 Activity。
     */
    private void switchLanguage(String tag) {
        String target = Strings.normalise(tag);
        if (target.equals(Strings.lang())) {
            return;
        }
        String name = languageName(target);
        prefs.setUiLanguage(target);
        Strings.setLang(target);
        toast(Strings.get("settings.language.changed", name));
        if (settingsDialog != null) {
            settingsDialog.dismiss();
            settingsDialog = null;
        }
        rebuildContent();
        showSettingsDialog();
    }

    /**
     * 按当前语言重建整个内容视图。
     *
     * <p>日志不会丢：{@code logBuffer} 是纯文本权威副本，重建后用
     * {@link #renderLog()} 从它恢复带样式的 {@code logStyled} 再挂到新视图上；
     * 链接输入、音质选择、进度行与正在等待的选曲输入也一并保留。
     */
    private void rebuildContent() {
        String urls = edUrls == null ? "" : edUrls.getText().toString();
        String quality = rgQuality == null ? prefs.quality() : currentQuality();
        String progress = tvProgress == null ? "" : tvProgress.getText().toString();
        CharSequence status = tvStatus == null ? "" : tvStatus.getText();
        int statusColor = tvStatus == null ? Ui.ON_SURFACE_VARIANT : tvStatus.getCurrentTextColor();
        String promptText = edPrompt == null ? "" : edPrompt.getText().toString();
        boolean restorePrompt = promptVisible;
        int promptCount = 0;
        if (restorePrompt) {
            // 曲目表还在 recentLines 里，先数一遍，重建后好把提示原样写回去。
            List<Integer> numbers = new ArrayList<Integer>();
            List<String> names = new ArrayList<String>();
            parseTrackTable(numbers, names);
            promptCount = numbers.size();
        }
        promptVisible = false;

        setContentView(buildContentView());

        edUrls.setText(urls.isEmpty() ? prefs.lastUrls() : urls);
        selectQuality(quality);
        logStyled.clear();
        renderLog();
        tvLog.setText(logStyled);
        if (tvLogPlaceholder != null) {
            tvLogPlaceholder.setVisibility(logBuffer.length() == 0 ? View.VISIBLE : View.GONE);
        }
        if (!progress.isEmpty()) {
            onTransient(progress);
        }
        updateRunningUi(AmdlRunner.isRunning());
        if (status.length() > 0) {
            tvStatus.setText(status);
            tvStatus.setTextColor(statusColor);
        }
        updateQueueCount();
        if (restorePrompt) {
            // 不重跑 showPromptBar()：它会重新设定表格解析起点，计数会丢。
            tvPromptHint.setText(promptCount == 0
                    ? Strings.get("prompt.hint.empty")
                    : Strings.get("prompt.hint.count", promptCount));
            promptVisible = true;
            promptBar.setVisibility(View.VISIBLE);
            edPrompt.setText(promptText);
            edPrompt.requestFocus();
        }
        scrollLogToBottom();
    }

    private TextView sectionHeader(String title, boolean withDivider) {
        TextView header = Ui.cardTitle(this, title);
        header.setTextColor(Ui.PRIMARY);
        header.setPadding(0, withDivider ? Ui.dp(this, 16) : 0, 0, Ui.dp(this, 8));
        return header;
    }

    private void addField(LinearLayout parent, List<EditText> sink, List<String> keys,
                          String key, String caption, String hint) {
        Ui.Field field = Ui.field(this, caption, hint, false);
        field.input.setText(prefs.raw(key));
        parent.addView(field.container);
        LinearLayout.LayoutParams params =
                (LinearLayout.LayoutParams) field.container.getLayoutParams();
        params.bottomMargin = Ui.dp(this, 12);
        field.container.setLayoutParams(params);
        sink.add(field.input);
        keys.add(key);
    }

    private void addSwitch(LinearLayout parent, List<Switch> sink, List<String> keys,
                           String key, String label) {
        parent.addView(Ui.switchRow(this, label, key, prefs.bool(key, defaultBool(key))));
        // switchRow returns the row; the switch is its last child.
        LinearLayout row = (LinearLayout) parent.getChildAt(parent.getChildCount() - 1);
        sink.add((Switch) row.getChildAt(row.getChildCount() - 1));
        keys.add(key);
    }

    /** Defaults must match the ones used when the config file is written. */
    private static boolean defaultBool(String key) {
        if (Prefs.K_ALAC_FIX.equals(key)
                || Prefs.K_LYRICS_SAVE.equals(key)
                || Prefs.K_LYRICS_EMBED.equals(key)
                || Prefs.K_COVER_EMBED.equals(key)
                || Prefs.K_TAG_SORT.equals(key)
                || Prefs.K_TAG_ITUNES.equals(key)
                || Prefs.K_ARTIST_FOLDER.equals(key)) {
            return true;
        }
        return false;
    }

    // ------------------------------------------------------------ 选曲下载

    /**
     * Reveals the docked input bar when the CLI asks which tracks to rip.
     *
     * <p>The numbers stay visible in the log; the user types them here and the
     * answer is written to the child process' stdin.
     */
    private void showPromptBar() {
        if (promptBar == null || promptVisible) {
            return;
        }
        List<Integer> numbers = new ArrayList<Integer>();
        List<String> names = new ArrayList<String>();
        parseTrackTable(numbers, names);
        tableParseFrom = recentLines.size();

        promptVisible = true;
        tvPromptHint.setText(numbers.isEmpty()
                ? Strings.get("prompt.hint.empty")
                : Strings.get("prompt.hint.count", numbers.size()));
        edPrompt.setText("");
        promptBar.setVisibility(View.VISIBLE);
        edPrompt.requestFocus();
        main.postDelayed(new Runnable() {
            @Override
            public void run() {
                scrollLogToBottom();
            }
        }, 80);
    }

    private void hidePromptBar() {
        promptVisible = false;
        if (promptBar != null) {
            promptBar.setVisibility(View.GONE);
        }
        if (edPrompt != null) {
            edPrompt.setText("");
        }
    }

    /** Validates the typed selection before handing it to the CLI. */
    private void submitSelection(String raw) {
        String normalized = normalizeSelection(raw);
        if ("all".equals(normalized)) {
            sendSelection("all");
            return;
        }
        if (normalized.isEmpty()) {
            toast(Strings.get("prompt.err.empty"));
            return;
        }
        if (!normalized.matches("[0-9]+(-[0-9]+)?(,[0-9]+(-[0-9]+)?)*")) {
            toast(Strings.get("prompt.err.format"));
            return;
        }
        sendSelection(normalized);
    }

    private void sendSelection(String text) {
        hidePromptBar();
        if (!AmdlRunner.writeStdin(text)) {
            toast(Strings.get("prompt.err.dead"));
            return;
        }
        appendLine(text.isEmpty()
                ? Strings.get("prompt.sent.skip")
                : Strings.get("prompt.sent.all", text));
    }

    /**
     * Cleans up what people actually type: full-width commas and spaces are
     * common on a Chinese keyboard and the CLI rejects them ("Invalid option").
     */
    static String normalizeSelection(String raw) {
        if (raw == null) {
            return "";
        }
        String value = raw.trim().toLowerCase();
        if (value.isEmpty()) {
            return "";
        }
        if (Strings.matchesAny("select.all", value)) {
            return "all";
        }
        value = value.replace('，', ',').replace('、', ',')
                .replace('；', ',').replace(';', ',')
                .replace('－', '-').replace('—', '-').replace('–', '-')
                .replace(" ", "").replace("\t", "");
        value = value.replaceAll(",+", ",");
        if (value.startsWith(",")) {
            value = value.substring(1);
        }
        if (value.endsWith(",")) {
            value = value.substring(0, value.length() - 1);
        }
        return value;
    }

    /**
     * Reads the track table the CLI printed.
     *
     * <p>Long titles wrap: the number column is then blank and the title continues
     * on following rows, so those rows are appended to the previous track. The
     * table looks like:
     * <pre>
     * |   |  TRACK NAME  | RATING | TYPE |
     * | 1 | 01. Lorem /  | None   | SONG |
     * |   | Ipsum Dolor  |        |      |
     * </pre>
     */
    private void parseTrackTable(List<Integer> numbers, List<String> names) {
        numbers.clear();
        names.clear();
        int current = -1;
        for (int i = Math.max(0, tableParseFrom); i < recentLines.size(); i++) {
            String line = AmdlRunner.stripAnsi(recentLines.get(i)).trim();
            if (!line.startsWith("|")) {
                continue;
            }
            String[] raw = line.split("\\|", -1);
            if (raw.length < 3) {
                continue;
            }
            List<String> cells = new ArrayList<String>();
            for (int c = 1; c < raw.length - 1; c++) {
                cells.add(raw[c].trim());
            }
            if (cells.size() < 2) {
                continue;
            }
            String numberCell = cells.get(0);
            String nameCell = cells.get(1);
            if (numberCell.isEmpty()) {
                // Wrapped continuation of the previous title.
                if (current >= 0 && !nameCell.isEmpty()) {
                    names.set(current, names.get(current) + " " + nameCell);
                }
                continue;
            }
            int number;
            try {
                number = Integer.parseInt(numberCell);
            } catch (NumberFormatException e) {
                continue;
            }
            if (numbers.contains(Integer.valueOf(number))) {
                continue;
            }
            numbers.add(Integer.valueOf(number));
            names.add(nameCell);
            current = names.size() - 1;
        }
    }

    // ---------------------------------------------------------------- 日志菜单

    private void showLogMenu() {
        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.addView(menuRow(Strings.get("log.menu.copy"), new Runnable() {
            @Override
            public void run() {
                ClipboardManager cm = (ClipboardManager) getSystemService(Context.CLIPBOARD_SERVICE);
                if (cm != null) {
                    cm.setPrimaryClip(ClipData.newPlainText("amdl-log", logBuffer.toString()));
                    toast(Strings.get("log.copied"));
                }
            }
        }));
        content.addView(menuRow(Strings.get("log.menu.clear"), new Runnable() {
            @Override
            public void run() {
                AmdlRunner.clearLog();
                clearLogView();
            }
        }));
        content.addView(menuRow(Strings.get("log.menu.clearCrash"), new Runnable() {
            @Override
            public void run() {
                CrashLog.clear(MainActivity.this);
                toast(Strings.get("log.crash.cleared"));
            }
        }));
        Ui.dialog(this, Strings.get("log.menu.title"), content, null, null,
                Strings.get("common.close"), null).show();
    }

    private View menuRow(String label, final Runnable action) {
        LinearLayout row = Ui.row(this);
        TextView text = Ui.text(this, label, 15, Ui.ON_SURFACE);
        row.addView(text, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        row.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                action.run();
            }
        });
        return row;
    }

    // ------------------------------------------------------- runner callbacks

    @Override
    public void onLogLine(String line) {
        appendLine(line);
        rememberLine(line);
        if (isSelectPrompt(line)) {
            showPromptBar();
        }
    }

    @Override
    public void onTransient(String line) {
        if (tvProgress == null) {
            return;
        }
        if (line == null || line.isEmpty()) {
            tvProgress.setVisibility(View.GONE);
            return;
        }
        tvProgress.setText(line);
        tvProgress.setVisibility(View.VISIBLE);
    }

    private void rememberLine(String line) {
        recentLines.add(line);
        while (recentLines.size() > 800) {
            recentLines.remove(0);
        }
    }

    /** The CLI prints "select: " (album/playlist) or "Enter your choice:" (artist). */
    private static boolean isSelectPrompt(String line) {
        if (line == null) {
            return false;
        }
        String t = line.trim();
        return t.equals("select:")
                || t.endsWith("Enter your choice:")
                || t.endsWith("Enter the choice:");
    }

    @Override
    public void onFinished(int exitCode, boolean cancelled) {
        hidePromptBar();
        clearProgressLine();
        updateRunningUi(false);
        int total = AmdlRunner.summaryTotal();
        int completed = AmdlRunner.summaryCompleted();
        int errors = AmdlRunner.summaryErrors();
        String message;
        // 成功与否不再靠文案前缀判断：本地化后不同语言的措辞完全不同。
        boolean success = false;
        if (cancelled) {
            message = Strings.get("result.cancelled");
        } else if (total > 0) {
            message = Strings.get("result.done", completed, total, Math.max(errors, 0));
            success = true;
        } else if (errors > 0) {
            message = Strings.get("result.failed", errors, exitCode);
        } else {
            // 上游即使全部失败也返回 0，所以不把退出码当成功依据。
            message = Strings.get("result.empty", exitCode);
        }
        tvStatus.setText(message);
        tvStatus.setTextColor(success ? Ui.SUCCESS : Ui.ERROR);
        appendLine("=== " + message + " ===");
    }

    // ---------------------------------------------------------------- helpers

    private File outputRoot() {
        File base = getExternalFilesDir(null);
        if (base == null) {
            base = getFilesDir();
        }
        return new File(base, "Music");
    }

    private void restoreLog() {
        logBuffer.setLength(0);
        hideLogPlaceholder();
        List<String> lines = AmdlRunner.logSnapshot();
        for (int i = 0; i < lines.size(); i++) {
            if (logBuffer.length() > 0) {
                logBuffer.append('\n');
            }
            logBuffer.append(lines.get(i));
        }
        // 恢复历史时同时重建带样式副本：否则第一行新日志会把历史覆盖掉，
        // 而且历史行也不会有级别颜色。
        logStyled.clear();
        renderLog();
        tvLog.setText(logStyled);
        if (logBuffer.length() == 0 && tvLogPlaceholder != null) {
            tvLogPlaceholder.setVisibility(View.VISIBLE);
        }
        scrollLogToBottom();
    }

    private void hideLogPlaceholder() {
        if (tvLogPlaceholder != null) {
            tvLogPlaceholder.setVisibility(View.GONE);
        }
    }

    private void appendLine(String line) {
        hideLogPlaceholder();
        if (logBuffer.length() > 0) {
            logBuffer.append('\n');
        }
        logBuffer.append(line);
        appendStyledLine(line);
        if (logBuffer.length() > 200_000) {
            int cut = logBuffer.indexOf("\n", logBuffer.length() - 150_000);
            if (cut > 0) {
                logBuffer.delete(0, cut + 1);
            }
            renderLog(); // 裁剪后重建样式副本，二者保持一致
        }
        tvLog.setText(logStyled);
        if (logBuffer.length() == 0 && tvLogPlaceholder != null) {
            tvLogPlaceholder.setVisibility(View.VISIBLE);
        }
        scrollLogToBottom();
    }

    /** 追加一行并按 CLI 的级别标记上色（✔ ✔ ⚠ ✖ ▸ · ─）。 */
    private void appendStyledLine(String line) {
        if (logStyled.length() > 0) {
            logStyled.append('\n');
        }
        int start = logStyled.length();
        logStyled.append(line);
        styleRange(start, logStyled.length(), line);
    }

    private void styleRange(int start, int end, String line) {
        if (end <= start) {
            return;
        }
        String trimmed = line.trim();
        if (trimmed.isEmpty()) {
            return;
        }
        char mark = trimmed.charAt(0);
        int color = 0;
        boolean bold = false;
        switch (mark) {
            case '\u2714': // ✔
                color = Ui.SUCCESS;
                break;
            case '\u26A0': // ⚠
                color = Ui.WARNING;
                break;
            case '\u2716': // ✖
                color = Ui.ERROR;
                break;
            case '\u25B8': // ▸ 阶段
                color = Ui.PRIMARY;
                bold = true;
                break;
            case '\u00B7': // · 细节
                color = Ui.ON_SURFACE_VARIANT;
                break;
            case '\u2500': // ─ 横幅
                color = Ui.PRIMARY;
                bold = true;
                break;
            case '[': // [检测] 之类的分类行
                color = Ui.ON_SURFACE_VARIANT;
                break;
            default:
                break;
        }
        if (color != 0) {
            logStyled.setSpan(new ForegroundColorSpan(color), start, end,
                    Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        }
        if (bold) {
            logStyled.setSpan(new StyleSpan(Typeface.BOLD), start, end,
                    Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        }
        if (trimmed.startsWith("===") || trimmed.startsWith("Completed:")) {
            logStyled.setSpan(new StyleSpan(Typeface.BOLD), start, end,
                    Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        }
    }

    /** 由纯文本缓冲重建带样式副本（裁剪日志后调用）。 */
    private void renderLog() {
        logStyled.clear();
        int index = 0;
        int length = logBuffer.length();
        while (index <= length) {
            int next = logBuffer.indexOf("\n", index);
            if (next < 0) {
                next = length;
            }
            appendStyledLine(logBuffer.substring(index, next));
            index = next + 1;
        }
    }

    private void scrollLogToBottom() {
        scLog.post(new Runnable() {
            @Override
            public void run() {
                scLog.fullScroll(View.FOCUS_DOWN);
            }
        });
    }

    private void clearProgressLine() {
        if (tvProgress != null) {
            tvProgress.setText("");
            tvProgress.setVisibility(View.GONE);
        }
    }

    private void clearLogView() {
        logBuffer.setLength(0);
        logStyled.clear();
        tvLog.setText("");
        if (tvLogPlaceholder != null) {
            tvLogPlaceholder.setVisibility(View.VISIBLE);
        }
    }

    private void updateRunningUi(boolean running) {
        btnStart.setEnabled(!running);
        btnStop.setEnabled(running);
        btnStart.setAlpha(running ? 0.38f : 1f);
        tvStatus.setText(running ? Strings.get("status.running") : Strings.get("status.ready"));
        tvStatus.setTextColor(Ui.ON_SURFACE_VARIANT);
    }

    private String currentQuality() {
        int id = rgQuality.getCheckedRadioButtonId();
        if (id == 1002) {
            return Prefs.QUALITY_ATMOS;
        }
        if (id == 1003) {
            return Prefs.QUALITY_AAC;
        }
        return Prefs.QUALITY_ALAC;
    }

    private void selectQuality(String quality) {
        if (Prefs.QUALITY_ATMOS.equals(quality)) {
            rgQuality.check(1002);
        } else if (Prefs.QUALITY_AAC.equals(quality)) {
            rgQuality.check(1003);
        } else {
            rgQuality.check(1001);
        }
    }

    private void requestNotificationPermissionIfNeeded() {
        if (Build.VERSION.SDK_INT >= SDK_TIRAMISU
                && checkSelfPermission(PERM_POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            try {
                requestPermissions(new String[]{PERM_POST_NOTIFICATIONS}, REQ_NOTIFICATIONS);
            } catch (Throwable ignored) {
                // Optional permission; never fail start-up for it.
            }
        }
    }

    private void warnAboutMissingBinary() {
        if (AmdlRunner.binary(this) == null) {
            appendLine(Strings.get("log.binary.missing"));
            btnStart.setEnabled(false);
        }
    }

    static List<String> parseUrls(String raw) {
        List<String> out = new ArrayList<String>();
        if (raw == null) {
            return out;
        }
        String[] lines = raw.split("[\\r\\n]+");
        for (int i = 0; i < lines.length; i++) {
            String trimmed = lines[i].trim();
            if (!trimmed.isEmpty()) {
                out.add(trimmed);
            }
        }
        return out;
    }

    /** Splits a user supplied argument string, honouring single and double quotes. */
    static List<String> splitArgs(String raw) {
        List<String> out = new ArrayList<String>();
        if (raw == null || TextUtils.isEmpty(raw.trim())) {
            return out;
        }
        StringBuilder current = new StringBuilder();
        char quote = 0;
        for (int i = 0; i < raw.length(); i++) {
            char c = raw.charAt(i);
            if (quote != 0) {
                if (c == quote) {
                    quote = 0;
                } else {
                    current.append(c);
                }
            } else if (c == '"' || c == '\'') {
                quote = c;
            } else if (Character.isWhitespace(c)) {
                if (current.length() > 0) {
                    out.add(current.toString());
                    current.setLength(0);
                }
            } else {
                current.append(c);
            }
        }
        if (current.length() > 0) {
            out.add(current.toString());
        }
        return out;
    }

    private static String readAll(InputStream in) {
        if (in == null) {
            return "";
        }
        try {
            ByteArrayOutputStream buffer = new ByteArrayOutputStream();
            byte[] chunk = new byte[4096];
            int read;
            while ((read = in.read(chunk)) != -1) {
                buffer.write(chunk, 0, read);
            }
            in.close();
            return buffer.toString("UTF-8");
        } catch (Exception e) {
            return "";
        }
    }

    private static String trimForUi(String value, int max) {
        String v = value == null ? "" : value.replace('\n', ' ').trim();
        return v.length() <= max ? v : v.substring(0, max) + ELLIPSIS;
    }

    private void toast(String message) {
        Toast.makeText(this, message, Toast.LENGTH_LONG).show();
    }
}
