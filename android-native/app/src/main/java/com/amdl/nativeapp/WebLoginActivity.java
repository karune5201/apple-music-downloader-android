package com.amdl.nativeapp;

import android.app.Activity;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.Context;
import android.graphics.Typeface;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.webkit.CookieManager;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.Button;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.widget.Toast;

/**
 * 网页登录：在内置浏览器里登录 music.apple.com，自动抓取 media-user-token。
 *
 * <p>电台需要 Apple Music 账号的 media-user-token（wrapper-lite 暂不支持电台）。
 * 手动做法要开开发者工具翻 Cookie，手机上几乎不可行，所以这里直接用 WebView
 * 走一遍登录：登录成功后 Cookie 里就有 {@code media-user-token}，读出来写进
 * {@link Prefs#K_MEDIA_TOKEN}，配置面板里的那一栏随即刷新。
 *
 * <p>抓不到（Apple 改 Cookie 名、WebView 被拦）时还有手动通道：把开发者工具里
 * 复制的值粘到底部输入框，或者点“粘贴剪贴板”。
 *
 * <p>token 只写进本机 SharedPreferences，随下载请求交给 wrapper-lite 转发给
 * Apple，App 不上传任何地方。
 */
public class WebLoginActivity extends Activity {

    /** 登录入口：站内再点“登录”，比直接跳 idmsa 更稳。 */
    private static final String START_URL = "https://music.apple.com/";
    /** 取 Cookie 用的 URL：域名要能覆盖 .music.apple.com 的 cookie。 */
    private static final String COOKIE_URL = "https://music.apple.com";
    private static final String COOKIE_NAME = "media-user-token";

    /** Cookie 轮询间隔与总时长：登录可能要点两步验证，给足 10 分钟。 */
    private static final long POLL_INTERVAL_MS = 800;
    private static final long POLL_TIMEOUT_MS = 10 * 60 * 1000;
    /** 抓到后停留一下再关，让用户看清提示。 */
    private static final long CLOSE_DELAY_MS = 1200;

    private Prefs prefs;
    private WebView web;
    private TextView tvStatus;
    private EditText edManual;

    private final Handler main = new Handler(Looper.getMainLooper());
    /** 已保存过 token：停止轮询，避免重复写盘。 */
    private boolean adopted;
    private long startedAt;

    private final Runnable pollTask = new Runnable() {
        @Override
        public void run() {
            if (adopted || web == null) {
                return;
            }
            String token = readToken();
            if (token != null) {
                adopt(token);
                return;
            }
            if (System.currentTimeMillis() - startedAt > POLL_TIMEOUT_MS) {
                setStatus(Strings.get("web.timeout"), Ui.ON_SURFACE_VARIANT);
                return;
            }
            main.postDelayed(this, POLL_INTERVAL_MS);
        }
    };

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        try {
            prefs = new Prefs(this);
            Strings.setLang(prefs.uiLanguage());
            Ui.apply(prefs.darkTheme(this));
            applySystemBars();
            setContentView(buildContentView());
            startedAt = System.currentTimeMillis();
            main.postDelayed(pollTask, POLL_INTERVAL_MS);
        } catch (Throwable error) {
            showFatalScreen(error);
        }
    }

    @Override
    protected void onDestroy() {
        main.removeCallbacks(pollTask);
        if (web != null) {
            try {
                web.destroy();
            } catch (Throwable ignored) {
                // 销毁失败无关紧要。
            }
        }
        super.onDestroy();
    }

    @Override
    public void onBackPressed() {
        if (web != null && web.canGoBack()) {
            web.goBack();
            return;
        }
        super.onBackPressed();
    }

    /** 状态栏 / 导航栏跟界面同色，深色下用浅色图标（与主页一致）。 */
    private void applySystemBars() {
        try {
            android.view.Window window = getWindow();
            window.setStatusBarColor(Ui.SURFACE);
            window.setNavigationBarColor(Ui.SURFACE);
            int flags = 0;
            if (!Ui.isDark()) {
                flags |= View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
                flags |= View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
            }
            window.getDecorView().setSystemUiVisibility(flags);
        } catch (Throwable ignored) {
            // 纯装饰。
        }
    }

    /** WebView 都建不起来时（ROM 删了 WebView 组件）也别白屏。 */
    private void showFatalScreen(Throwable error) {
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setBackgroundColor(Ui.SURFACE);
        root.setPadding(Ui.dp(this, 20), Ui.dp(this, 40), Ui.dp(this, 20), Ui.dp(this, 20));
        TextView title = Ui.text(this, Strings.get("web.title"), 20, Ui.ERROR);
        title.setTypeface(Typeface.create("sans-serif-medium", Typeface.NORMAL));
        root.addView(title);
        TextView detail = Ui.text(this, Strings.get("web.unsupported") + "\n\n"
                + String.valueOf(error), 12, Ui.ON_SURFACE_VARIANT);
        root.addView(detail);
        Button close = Ui.filledButton(this, Strings.get("common.close"));
        close.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                finish();
            }
        });
        LinearLayout.LayoutParams closeParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        closeParams.topMargin = Ui.dp(this, 16);
        root.addView(close, closeParams);
        setContentView(root);
    }

    // ------------------------------------------------------------------ 视图

    private View buildContentView() {
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setBackgroundColor(Ui.SURFACE);
        root.setFitsSystemWindows(true);

        buildAppBar(root);
        buildHelpCard(root);
        buildWebView(root);
        buildFooter(root);
        return root;
    }

    private void buildAppBar(LinearLayout root) {
        LinearLayout appBar = new LinearLayout(this);
        appBar.setOrientation(LinearLayout.HORIZONTAL);
        appBar.setGravity(Gravity.CENTER_VERTICAL);
        appBar.setPadding(Ui.dp(this, 20), Ui.dp(this, 12), Ui.dp(this, 8), Ui.dp(this, 4));

        LinearLayout titles = new LinearLayout(this);
        titles.setOrientation(LinearLayout.VERTICAL);
        titles.addView(Ui.appBarTitle(this, Strings.get("web.title")));
        TextView subtitle = Ui.text(this, Strings.get("web.subtitle"), 11, Ui.ON_SURFACE_VARIANT);
        subtitle.setPadding(0, Ui.dp(this, 2), 0, 0);
        titles.addView(subtitle);
        appBar.addView(titles, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));

        Button btnReload = Ui.appBarButton(this, Strings.get("web.reload"));
        btnReload.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                if (web != null) {
                    web.reload();
                }
            }
        });
        Button btnClose = Ui.appBarButton(this, Strings.get("common.close"));
        btnClose.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                finish();
            }
        });
        appBar.addView(btnReload);
        appBar.addView(btnClose);
        root.addView(appBar);
    }

    /** 说明卡片：为什么要 token、自动抓不到时怎么办、隐私边界。 */
    private void buildHelpCard(LinearLayout root) {
        LinearLayout card = Ui.card(this);
        LinearLayout.LayoutParams cardParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        cardParams.setMargins(Ui.dp(this, 16), Ui.dp(this, 4), Ui.dp(this, 16), Ui.dp(this, 8));
        card.setLayoutParams(cardParams);

        card.addView(Ui.cardTitle(this, Strings.get("web.help.title")));
        TextView body = Ui.text(this, Strings.get("web.help.body"), 12, Ui.ON_SURFACE_VARIANT);
        body.setPadding(0, Ui.dp(this, 6), 0, 0);
        body.setTextIsSelectable(true);
        card.addView(body);
        root.addView(card);
    }

    private void buildWebView(LinearLayout root) {
        FrameLayout host = new FrameLayout(this);
        host.setBackground(Ui.outlined(Ui.SURFACE_CONTAINER_LOW, Ui.OUTLINE_VARIANT, 1, 12, this));
        try {
            host.setClipToOutline(true);
        } catch (Throwable ignored) {
            // 直角也不影响使用。
        }

        try {
            web = new WebView(this);
        } catch (Throwable error) {
            web = null;
        }

        if (web == null) {
            TextView fallback = Ui.text(this, Strings.get("web.unsupported"), 13, Ui.ERROR);
            fallback.setGravity(Gravity.CENTER);
            fallback.setPadding(Ui.dp(this, 16), Ui.dp(this, 16), Ui.dp(this, 16), Ui.dp(this, 16));
            host.addView(fallback, new FrameLayout.LayoutParams(
                    ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        } else {
            configure(web);
            web.setWebViewClient(new WebViewClient() {
                @Override
                public void onPageFinished(WebView view, String url) {
                    // 每加载完一页就查一次：登录跳转多，早一点查到省一次等待。
                    main.removeCallbacks(pollTask);
                    main.post(pollTask);
                }

                @Override
                public void onReceivedError(WebView view, WebResourceRequest request,
                                            WebResourceError error) {
                    if (request != null && request.isForMainFrame()) {
                        setStatus(Strings.get("web.error", describe(error)), Ui.ERROR);
                    }
                }
            });
            host.addView(web, new FrameLayout.LayoutParams(
                    ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
            try {
                web.loadUrl(START_URL);
            } catch (Throwable error) {
                setStatus(Strings.get("web.error", String.valueOf(error)), Ui.ERROR);
            }
        }

        LinearLayout.LayoutParams hostParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f);
        hostParams.setMargins(Ui.dp(this, 16), 0, Ui.dp(this, 16), 0);
        root.addView(host, hostParams);
    }

    private void configure(WebView view) {
        WebSettings settings = view.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setDatabaseEnabled(true);
        settings.setUseWideViewPort(true);
        settings.setLoadWithOverviewMode(true);
        settings.setJavaScriptCanOpenWindowsAutomatically(false);
        try {
            settings.setMixedContentMode(WebSettings.MIXED_CONTENT_COMPATIBILITY_MODE);
        } catch (Throwable ignored) {
            // 老 WebView 没有这个开关。
        }
        try {
            // 去掉 UA 里的 "; wv"（WebView 标记）：Apple 的登录页见到它会提示浏览器不受支持。
            String ua = settings.getUserAgentString();
            if (ua != null && ua.contains("; wv")) {
                settings.setUserAgentString(ua.replace("; wv", ""));
            }
        } catch (Throwable ignored) {
            // UA 改不了也还能用，最坏情况退回手动粘贴。
        }
        try {
            // Apple 的登录流程跨多个域，第三方 cookie 必须放行，否则回跳后没有会话。
            CookieManager.getInstance().setAcceptCookie(true);
            CookieManager.getInstance().setAcceptThirdPartyCookies(view, true);
        } catch (Throwable ignored) {
            // 个别 ROM 的 WebView 实现没有这组接口。
        }
    }

    /** 底部：状态行 + 手动粘贴通道。抓不到时这里是唯一的兜底。 */
    private void buildFooter(LinearLayout root) {
        LinearLayout footer = new LinearLayout(this);
        footer.setOrientation(LinearLayout.VERTICAL);
        footer.setPadding(Ui.dp(this, 16), Ui.dp(this, 8), Ui.dp(this, 16), Ui.dp(this, 10));

        tvStatus = Ui.text(this, Strings.get("web.waiting"), 12, Ui.ON_SURFACE_VARIANT);
        footer.addView(tvStatus);

        Ui.Field field = Ui.field(this, Strings.get("web.manual"),
                Strings.get("web.manual.hint"), false);
        edManual = field.input;
        LinearLayout.LayoutParams fieldParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        fieldParams.topMargin = Ui.dp(this, 8);
        footer.addView(field.container, fieldParams);

        LinearLayout actions = new LinearLayout(this);
        actions.setOrientation(LinearLayout.HORIZONTAL);
        actions.setGravity(Gravity.CENTER_VERTICAL);
        actions.addView(new View(this), new LinearLayout.LayoutParams(0, 1, 1f));

        Button btnPaste = Ui.textButton(this, Strings.get("web.paste"));
        btnPaste.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                pasteFromClipboard();
            }
        });
        Button btnSave = Ui.filledButton(this, Strings.get("web.save"));
        btnSave.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                saveManually();
            }
        });
        actions.addView(btnPaste);
        actions.addView(btnSave);
        footer.addView(actions);

        root.addView(footer);
    }

    // ------------------------------------------------------------------ 逻辑

    private void pasteFromClipboard() {
        try {
            ClipboardManager cm = (ClipboardManager) getSystemService(Context.CLIPBOARD_SERVICE);
            if (cm == null || !cm.hasPrimaryClip()) {
                toast(Strings.get("web.empty"));
                return;
            }
            ClipData clip = cm.getPrimaryClip();
            if (clip == null || clip.getItemCount() == 0) {
                toast(Strings.get("web.empty"));
                return;
            }
            CharSequence text = clip.getItemAt(0).coerceToText(this);
            String value = text == null ? "" : text.toString().trim();
            if (value.isEmpty()) {
                toast(Strings.get("web.empty"));
                return;
            }
            edManual.setText(value);
            saveManually();
        } catch (Throwable error) {
            toast(Strings.get("web.empty"));
        }
    }

    private void saveManually() {
        String token = cleanToken(edManual.getText().toString());
        if (token.isEmpty()) {
            toast(Strings.get("web.empty"));
            return;
        }
        adopt(token);
    }

    /** 存进设置并给个明确反馈，然后返回。 */
    private void adopt(String token) {
        adopted = true;
        main.removeCallbacks(pollTask);
        prefs.setStr(Prefs.K_MEDIA_TOKEN, token);
        if (edManual != null) {
            edManual.setText(token);
        }
        setStatus(Strings.get("web.saved.preview", shorten(token)), Ui.SUCCESS);
        toast(Strings.get("web.saved"));
        main.postDelayed(new Runnable() {
            @Override
            public void run() {
                finish();
            }
        }, CLOSE_DELAY_MS);
    }

    private void setStatus(String text, int color) {
        if (tvStatus != null) {
            tvStatus.setText(text);
            tvStatus.setTextColor(color);
        }
    }

    /** 从 Cookie 里读 media-user-token；读不到返回 null。 */
    private String readToken() {
        if (web == null) {
            return null;
        }
        try {
            String cookies = CookieManager.getInstance().getCookie(COOKIE_URL);
            return findCookie(cookies, COOKIE_NAME);
        } catch (Throwable ignored) {
            return null;
        }
    }

    /** 在 "a=1; b=2" 形式的 Cookie 串里取某个键的值。 */
    static String findCookie(String cookies, String name) {
        if (cookies == null || cookies.isEmpty()) {
            return null;
        }
        String[] parts = cookies.split(";");
        for (int i = 0; i < parts.length; i++) {
            String part = parts[i].trim();
            int eq = part.indexOf('=');
            if (eq <= 0) {
                continue;
            }
            if (!name.equals(part.substring(0, eq).trim())) {
                continue;
            }
            String value = cleanToken(decodeCookieValue(part.substring(eq + 1).trim()));
            if (!value.isEmpty()) {
                return value;
            }
        }
        return null;
    }

    /**
     * Cookie 里的 %3D / %2B / %2F 要还原成 = + /，否则 base64 会缺字符。
     *
     * <p>不用 URLDecoder：它会把 token 里合法的 '+' 变成空格。
     */
    static String decodeCookieValue(String value) {
        if (value == null || value.indexOf('%') < 0) {
            return value == null ? "" : value;
        }
        return value
                .replaceAll("(?i)%3d", "=")
                .replaceAll("(?i)%2b", "+")
                .replaceAll("(?i)%2f", "/")
                .replaceAll("(?i)%3d", "=");
    }

    /** 去掉首尾空白与成对引号（从开发者工具复制时常带引号）。 */
    static String cleanToken(String raw) {
        String value = raw == null ? "" : raw.trim();
        while (value.length() >= 2
                && ((value.charAt(0) == '"' && value.charAt(value.length() - 1) == '"')
                || (value.charAt(0) == '\'' && value.charAt(value.length() - 1) == '\''))) {
            value = value.substring(1, value.length() - 1).trim();
        }
        return value;
    }

    /** UI 上只露出首尾，避免把整串 token 摊在屏幕上。 */
    static String shorten(String token) {
        if (token == null || token.length() <= 16) {
            return token == null ? "" : token;
        }
        return token.substring(0, 8) + "\u2026" + token.substring(token.length() - 6);
    }

    private static String describe(WebResourceError error) {
        if (error == null) {
            return "?";
        }
        CharSequence text = error.getDescription();
        return text == null || text.length() == 0 ? "?" : text.toString();
    }

    private void toast(String message) {
        Toast.makeText(this, message, Toast.LENGTH_LONG).show();
    }
}
