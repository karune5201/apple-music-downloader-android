package com.amdl.nativeapp;

import android.content.Context;
import android.util.Log;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.charset.Charset;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * 以上游 {@code config.yaml.example} 为模板生成 {@code config.yaml}。
 *
 * <p>不再手写整份 YAML：模板随 APK 打包，运行时逐行读取，把已知键替换成 App 里的值，
 * 其余键保持上游默认值（也保留注释）。上游新增配置项时，这里会自动带上，
 * 而且会作为“其它配置项”出现在设置面板里，不需要为此发新版。
 */
public final class ConfigTemplate {

    private static final String TAG = "ConfigTemplate";
    private static final String ASSET = "config.example.yaml";
    private static final Charset UTF8 = Charset.forName("UTF-8");

    /** 模板里的一行。{@code path} 为空表示注释 / 空行 / 列表项，原样输出。 */
    public static final class Line {
        public final String raw;
        public final String path;
        public final String indent;
        public final String key;
        public final String value;
        public final String comment;
        public final String defaultValue;

        Line(String raw, String path, String indent, String key, String value, String comment,
             String defaultValue) {
            this.raw = raw;
            this.path = path;
            this.indent = indent;
            this.key = key;
            this.value = value;
            this.comment = comment;
            this.defaultValue = defaultValue;
        }

        public boolean isEntry() {
            return path != null;
        }
    }

    private final List<Line> lines = new ArrayList<Line>();

    private ConfigTemplate() {
    }

    /** 读取内置模板；失败返回 null（调用方会退回手写配置）。 */
    public static ConfigTemplate load(Context context) {
        InputStream in = null;
        try {
            in = context.getAssets().open(ASSET);
            String text = readAll(in);
            ConfigTemplate template = new ConfigTemplate();
            template.parse(text);
            if (template.entries().isEmpty()) {
                Log.w(TAG, "template has no config keys");
                return null;
            }
            return template;
        } catch (Throwable e) {
            Log.w(TAG, "cannot read " + ASSET, e);
            return null;
        } finally {
            close(in);
        }
    }

    private void parse(String text) {
        String[] rows = text.split("\n", -1);
        // 缩进栈：index = 缩进层级，value = 该层的键名
        List<String> stack = new ArrayList<String>();
        List<Integer> stackIndent = new ArrayList<Integer>();

        for (int i = 0; i < rows.length; i++) {
            String raw = rows[i];
            if (raw.endsWith("\r")) {
                raw = raw.substring(0, raw.length() - 1);
            }
            String trimmed = raw.trim();
            if (trimmed.isEmpty() || trimmed.startsWith("#") || trimmed.startsWith("-")) {
                lines.add(new Line(raw, null, null, null, null, null, null));
                continue;
            }
            int indent = 0;
            while (indent < raw.length() && raw.charAt(indent) == ' ') {
                indent++;
            }
            int colon = trimmed.indexOf(':');
            if (colon <= 0) {
                lines.add(new Line(raw, null, null, null, null, null, null));
                continue;
            }
            String key = trimmed.substring(0, colon).trim();
            String rest = trimmed.substring(colon + 1).trim();

            while (!stackIndent.isEmpty() && stackIndent.get(stackIndent.size() - 1) >= indent) {
                stackIndent.remove(stackIndent.size() - 1);
                stack.remove(stack.size() - 1);
            }
            StringBuilder path = new StringBuilder();
            for (int s = 0; s < stack.size(); s++) {
                path.append(stack.get(s)).append('.');
            }
            path.append(key);

            if (rest.isEmpty()) {
                // 分节：本行原样保留，并把该层压栈
                stack.add(key);
                stackIndent.add(Integer.valueOf(indent));
                lines.add(new Line(raw, null, null, null, null, null, null));
                continue;
            }

            String value = rest;
            String comment = "";
            int hash = indexOfComment(rest);
            if (hash >= 0) {
                value = rest.substring(0, hash).trim();
                comment = rest.substring(hash);
            }
            String unquoted = unquote(value);
            lines.add(new Line(raw, path.toString(),
                    raw.substring(0, indent), key, value, comment, unquoted));
        }
    }

    /** 找到行内注释的起点；引号内的 # 不算。 */
    private static int indexOfComment(String text) {
        boolean inSingle = false;
        boolean inDouble = false;
        for (int i = 0; i < text.length(); i++) {
            char c = text.charAt(i);
            if (c == '\'' && !inDouble) {
                inSingle = !inSingle;
            } else if (c == '"' && !inSingle) {
                inDouble = !inDouble;
            } else if (c == '#' && !inSingle && !inDouble) {
                if (i == 0 || Character.isWhitespace(text.charAt(i - 1))) {
                    return i;
                }
            }
        }
        return -1;
    }

    private static String unquote(String value) {
        if (value.length() >= 2) {
            char first = value.charAt(0);
            char last = value.charAt(value.length() - 1);
            if ((first == '"' && last == '"') || (first == '\'' && last == '\'')) {
                return value.substring(1, value.length() - 1);
            }
        }
        return value;
    }

    /** 所有带值的键。 */
    public List<Line> entries() {
        List<Line> out = new ArrayList<Line>();
        for (int i = 0; i < lines.size(); i++) {
            if (lines.get(i).isEntry()) {
                out.add(lines.get(i));
            }
        }
        return out;
    }

    /** 模板里有、但 App 没有专门控件的键。 */
    public List<Line> unknown(Map<String, String> known) {
        List<Line> out = new ArrayList<Line>();
        for (int i = 0; i < lines.size(); i++) {
            Line line = lines.get(i);
            if (line.isEntry() && !ConfigKeys.isKnown(line.path, known)) {
                out.add(line);
            }
        }
        return out;
    }

    /**
     * 生成配置文本。
     *
     * @param known      App 接管的键 → 值
     * @param overrides  “其它配置项”里用户改过的键 → 值（原始文本）
     */
    public String render(Map<String, String> known, Map<String, String> overrides) {
        StringBuilder out = new StringBuilder(8192);
        for (int i = 0; i < lines.size(); i++) {
            Line line = lines.get(i);
            if (!line.isEntry()) {
                out.append(line.raw).append('\n');
                continue;
            }
            String value = null;
            if (known.containsKey(line.path)) {
                value = known.get(line.path);
            } else if (overrides.containsKey(line.path)) {
                value = overrides.get(line.path);
            }
            if (value == null) {
                out.append(line.raw).append('\n');
                continue;
            }
            out.append(line.indent).append(line.key).append(": ")
                    .append(format(value, line.value));
            if (!line.comment.isEmpty()) {
                // 注释前必须留空格，否则 "true# ..." 会被当成一个标量值
                out.append("  ").append(line.comment);
            }
            out.append('\n');
        }
        return out.toString();
    }

    /**
     * 按模板里原来的写法决定要不要加引号：原来是数字就继续裸写，
     * 原来是带引号的字符串就加引号。
     */
    private static String format(String value, String sample) {
        String trimmed = value == null ? "" : value.trim();
        boolean sampleQuoted = sample != null && sample.length() >= 1
                && (sample.charAt(0) == '"' || sample.charAt(0) == '\'');
        boolean numeric = trimmed.matches("-?\\d+(\\.\\d+)?");
        if (!sampleQuoted && numeric) {
            return trimmed;
        }
        if (!sampleQuoted && ("true".equals(trimmed) || "false".equals(trimmed))) {
            return trimmed;
        }
        return quote(trimmed);
    }

    private static String quote(String value) {
        String escaped = value.replace("\\", "\\\\").replace("\"", "\\\"");
        return "\"" + escaped + "\"";
    }

    static void write(File target, String text) throws IOException {
        OutputStream out = new FileOutputStream(target);
        try {
            out.write(text.getBytes(UTF8));
            out.flush();
        } finally {
            out.close();
        }
    }

    private static String readAll(InputStream in) throws IOException {
        ByteArrayOutputStream buffer = new ByteArrayOutputStream(8192);
        byte[] chunk = new byte[4096];
        int read;
        while ((read = in.read(chunk)) != -1) {
            buffer.write(chunk, 0, read);
        }
        return new String(buffer.toByteArray(), UTF8);
    }

    private static void close(InputStream in) {
        if (in != null) {
            try {
                in.close();
            } catch (IOException ignored) {
                // nothing useful to do
            }
        }
    }

    /** 供设置面板使用：把“其它配置项”按当前值渲染成可编辑项。 */
    public static Map<String, String> overridesFrom(Prefs prefs) {
        Map<String, String> map = new LinkedHashMap<String, String>();
        Map<String, ?> all = prefs.all();
        for (Map.Entry<String, ?> entry : all.entrySet()) {
            String key = entry.getKey();
            if (key.startsWith(Prefs.EXTRA_PREFIX)) {
                Object value = entry.getValue();
                map.put(key.substring(Prefs.EXTRA_PREFIX.length()),
                        value == null ? "" : String.valueOf(value));
            }
        }
        return map;
    }
}
