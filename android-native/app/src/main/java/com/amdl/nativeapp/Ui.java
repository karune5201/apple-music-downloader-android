package com.amdl.nativeapp;

import android.app.Activity;
import android.app.Dialog;
import android.content.Context;
import android.content.res.ColorStateList;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.ColorDrawable;
import android.graphics.drawable.Drawable;
import android.graphics.drawable.GradientDrawable;
import android.graphics.drawable.RippleDrawable;
import android.text.InputType;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.view.Window;
import android.widget.Button;
import android.widget.EditText;
import android.widget.LinearLayout;
import android.widget.RadioButton;
import android.widget.ScrollView;
import android.widget.Switch;
import android.widget.TextView;

/**
 * Material 3 look built from framework widgets only, in light and dark.
 *
 * <p>The app ships no AndroidX and no Material Components library, so the MD3
 * tokens (colour roles, corner radii, type scale, ripple states) are reproduced
 * by hand. Colour roles are mutable and switched by {@link #apply(boolean)};
 * every colour must be read after that call, which happens before any view is
 * created in {@code onCreate}.
 */
final class Ui {

    // ------------------------------------------------------------ colour roles

    static int PRIMARY;
    static int ON_PRIMARY;
    static int PRIMARY_CONTAINER;
    static int ON_PRIMARY_CONTAINER;
    static int SECONDARY_CONTAINER;
    static int ON_SECONDARY_CONTAINER;
    static int ERROR;
    static int ERROR_CONTAINER;
    static int ON_SURFACE;
    static int SURFACE;
    static int SURFACE_CONTAINER_LOW;
    static int SURFACE_CONTAINER;
    static int SURFACE_CONTAINER_HIGH;
    static int SURFACE_CONTAINER_HIGHEST;
    static int ON_SURFACE_VARIANT;
    static int OUTLINE;
    static int OUTLINE_VARIANT;
    static int SUCCESS;
    /** 警告色：日志里的 ⚠ 行（MD3 没有 warning 角色，取三级黄橙）。 */
    static int WARNING;

    private static boolean dark;

    static {
        apply(false);
    }

    private Ui() {
    }

    static boolean isDark() {
        return dark;
    }

    /** Switches every colour role between the MD3 baseline light and dark schemes. */
    static void apply(boolean darkMode) {
        dark = darkMode;
        if (darkMode) {
            PRIMARY = 0xFFD0BCFF;
            ON_PRIMARY = 0xFF381E72;
            PRIMARY_CONTAINER = 0xFF4F378B;
            ON_PRIMARY_CONTAINER = 0xFFEADDFF;
            SECONDARY_CONTAINER = 0xFF4A4458;
            ON_SECONDARY_CONTAINER = 0xFFE8DEF8;
            ERROR = 0xFFF2B8B5;
            ERROR_CONTAINER = 0xFF8C1D18;
            ON_SURFACE = 0xFFE6E0E9;
            SURFACE = 0xFF141218;
            SURFACE_CONTAINER_LOW = 0xFF1D1B20;
            SURFACE_CONTAINER = 0xFF211F26;
            SURFACE_CONTAINER_HIGH = 0xFF2B2930;
            SURFACE_CONTAINER_HIGHEST = 0xFF36343B;
            ON_SURFACE_VARIANT = 0xFFCAC4D0;
            OUTLINE = 0xFF938F99;
            OUTLINE_VARIANT = 0xFF49454F;
            SUCCESS = 0xFF7FD49A;
            WARNING = 0xFFFFCF7A;
        } else {
            PRIMARY = 0xFF6750A4;
            ON_PRIMARY = 0xFFFFFFFF;
            PRIMARY_CONTAINER = 0xFFEADDFF;
            ON_PRIMARY_CONTAINER = 0xFF21005D;
            SECONDARY_CONTAINER = 0xFFE8DEF8;
            ON_SECONDARY_CONTAINER = 0xFF1D192B;
            ERROR = 0xFFB3261E;
            ERROR_CONTAINER = 0xFFF9DEDC;
            ON_SURFACE = 0xFF1D1B20;
            SURFACE = 0xFFFEF7FF;
            SURFACE_CONTAINER_LOW = 0xFFF7F2FA;
            SURFACE_CONTAINER = 0xFFF3EDF7;
            SURFACE_CONTAINER_HIGH = 0xFFECE6F0;
            SURFACE_CONTAINER_HIGHEST = 0xFFE6E0E9;
            ON_SURFACE_VARIANT = 0xFF49454F;
            OUTLINE = 0xFF79747E;
            OUTLINE_VARIANT = 0xFFCAC4D0;
            SUCCESS = 0xFF1B6C36;
            WARNING = 0xFF8A5200;
        }
    }

    // ------------------------------------------------------------- utilities

    static int dp(Context context, float value) {
        return (int) (value * context.getResources().getDisplayMetrics().density + 0.5f);
    }

    private static int rippleOverlay() {
        return dark ? 0x33FFFFFF : 0x33000000;
    }

    static Drawable rounded(int color, float radiusDp, Context context) {
        GradientDrawable shape = new GradientDrawable();
        shape.setColor(color);
        shape.setCornerRadius(dp(context, radiusDp));
        return shape;
    }

    static Drawable outlined(int fill, int strokeColor, float strokeDp, float radiusDp,
                             Context context) {
        GradientDrawable shape = new GradientDrawable();
        shape.setColor(fill);
        shape.setCornerRadius(dp(context, radiusDp));
        shape.setStroke(dp(context, strokeDp), strokeColor);
        return shape;
    }

    /** Bounded ripple over the given shape, so presses stay inside rounded corners. */
    static Drawable ripple(Drawable content, float radiusDp, Context context) {
        GradientDrawable mask = new GradientDrawable();
        mask.setColor(0xFFFFFFFF);
        mask.setCornerRadius(dp(context, radiusDp));
        return new RippleDrawable(ColorStateList.valueOf(rippleOverlay()), content, mask);
    }

    /** Chips drawn as a tonal container — used for status text. */
    static TextView chip(Context context, String value, int textColor, int containerColor) {
        TextView view = text(context, value, 12, textColor);
        view.setBackground(rounded(containerColor, 8, context));
        view.setPadding(dp(context, 10), dp(context, 5), dp(context, 10), dp(context, 5));
        return view;
    }

    // ------------------------------------------------------------ typography

    static TextView text(Context context, String value, float sizeSp, int color) {
        TextView view = new TextView(context);
        view.setText(value);
        view.setTextSize(TypedValue.COMPLEX_UNIT_SP, sizeSp);
        view.setTextColor(color);
        view.setIncludeFontPadding(false);
        view.setLineSpacing(dp(context, 2), 1f);
        return view;
    }

    /** MD3 "headline small" — top app bar title. */
    static TextView appBarTitle(Context context, String value) {
        TextView view = text(context, value, 22, ON_SURFACE);
        view.setTypeface(Typeface.create("sans-serif-medium", Typeface.NORMAL));
        return view;
    }

    /** MD3 "title medium" — card headers. */
    static TextView cardTitle(Context context, String value) {
        TextView view = text(context, value, 16, ON_SURFACE);
        view.setTypeface(Typeface.create("sans-serif-medium", Typeface.NORMAL));
        return view;
    }

    /** MD3 "label medium" — field captions. */
    static TextView label(Context context, String value) {
        TextView view = text(context, value, 12, ON_SURFACE_VARIANT);
        view.setTypeface(Typeface.create("sans-serif-medium", Typeface.NORMAL));
        view.setLetterSpacing(0.02f);
        return view;
    }

    // --------------------------------------------------------------- buttons

    /** MD3 filled button (40dp tall, fully rounded). */
    static Button filledButton(Context context, String label) {
        Button button = baseButton(context, label, ON_PRIMARY, 14);
        button.setBackground(ripple(rounded(PRIMARY, 20, context), 20, context));
        return button;
    }

    /** MD3 tonal button — secondary container, for secondary actions. */
    static Button tonalButton(Context context, String label) {
        Button button = baseButton(context, label, ON_SECONDARY_CONTAINER, 14);
        button.setBackground(ripple(rounded(SECONDARY_CONTAINER, 20, context), 20, context));
        return button;
    }

    /** MD3 outlined button. */
    static Button outlinedButton(Context context, String label) {
        Button button = baseButton(context, label, PRIMARY, 14);
        button.setBackground(ripple(
                outlined(Color.TRANSPARENT, OUTLINE, 1, 20, context), 20, context));
        return button;
    }

    /** MD3 text button — low emphasis, no container. */
    static Button textButton(Context context, String label) {
        Button button = baseButton(context, label, PRIMARY, 14);
        button.setBackground(ripple(rounded(Color.TRANSPARENT, 20, context), 20, context));
        return button;
    }

    private static Button baseButton(Context context, String label, int textColor, float sizeSp) {
        Button button = new Button(context);
        button.setText(label);
        button.setAllCaps(false);
        button.setTextSize(TypedValue.COMPLEX_UNIT_SP, sizeSp);
        button.setTextColor(textColor);
        button.setLetterSpacing(0.01f);
        button.setMinHeight(dp(context, 40));
        button.setMinimumHeight(dp(context, 40));
        button.setMinimumWidth(0);
        button.setPadding(dp(context, 24), dp(context, 10), dp(context, 24), dp(context, 10));
        button.setStateListAnimator(null); // MD3 uses tonal surfaces, not shadows
        button.setIncludeFontPadding(false);
        button.setLayoutParams(new ViewGroup.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT));
        return button;
    }

    // ----------------------------------------------------------- text fields

    /** Result of {@link #field}: the outline container plus the editable text. */
    static final class Field {
        LinearLayout container;
        EditText input;
    }

    /**
     * MD3 outlined text field: 4dp corner radius, 1dp outline that turns into a
     * 2dp primary stroke while focused, caption above the box.
     */
    static Field field(Context context, String labelText, String hint, boolean multiline) {
        Field field = new Field();

        LinearLayout wrapper = new LinearLayout(context);
        wrapper.setOrientation(LinearLayout.VERTICAL);

        if (labelText != null && !labelText.isEmpty()) {
            TextView caption = label(context, labelText);
            caption.setPadding(dp(context, 4), 0, 0, dp(context, 6));
            wrapper.addView(caption);
        }

        LinearLayout box = new LinearLayout(context);
        box.setOrientation(LinearLayout.VERTICAL);
        box.setBackground(outlined(Color.TRANSPARENT, OUTLINE, 1, 4, context));
        box.setPadding(dp(context, 16), dp(context, 12), dp(context, 16), dp(context, 12));

        EditText input = new EditText(context);
        input.setBackground(null);
        input.setHint(hint);
        input.setTextSize(TypedValue.COMPLEX_UNIT_SP, 16);
        input.setTextColor(ON_SURFACE);
        input.setHintTextColor(ON_SURFACE_VARIANT);
        input.setIncludeFontPadding(false);
        input.setPadding(0, dp(context, 4), 0, dp(context, 4));
        if (multiline) {
            input.setInputType(InputType.TYPE_CLASS_TEXT
                    | InputType.TYPE_TEXT_FLAG_MULTI_LINE
                    | InputType.TYPE_TEXT_VARIATION_URI);
            input.setMinLines(3);
            input.setGravity(Gravity.TOP | Gravity.START);
            input.setHorizontallyScrolling(false);
        } else {
            input.setSingleLine(true);
            input.setInputType(InputType.TYPE_CLASS_TEXT);
        }
        box.addView(input, new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));
        wrapper.addView(box, new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));

        final LinearLayout fBox = box;
        input.setOnFocusChangeListener(new View.OnFocusChangeListener() {
            @Override
            public void onFocusChange(View v, boolean hasFocus) {
                fBox.setBackground(hasFocus
                        ? outlined(Color.TRANSPARENT, PRIMARY, 2, 4, v.getContext())
                        : outlined(Color.TRANSPARENT, OUTLINE, 1, 4, v.getContext()));
            }
        });

        field.container = wrapper;
        field.input = input;
        return field;
    }

    // ------------------------------------------------------------------ cards

    /** MD3 filled card: surface container colour, 12dp radius, no shadow. */
    static LinearLayout card(Context context) {
        LinearLayout card = new LinearLayout(context);
        card.setOrientation(LinearLayout.VERTICAL);
        card.setBackground(rounded(SURFACE_CONTAINER_LOW, 12, context));
        card.setPadding(dp(context, 16), dp(context, 16), dp(context, 16), dp(context, 16));
        return card;
    }

    /** Full-width card with bottom margin, ready to be stacked in a form. */
    static LinearLayout stackedCard(Context context) {
        LinearLayout card = card(context);
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        params.bottomMargin = dp(context, 12);
        card.setLayoutParams(params);
        return card;
    }

    /** MD3 list-style row with ripple, used inside dialogs. */
    static LinearLayout row(Context context) {
        LinearLayout row = new LinearLayout(context);
        row.setOrientation(LinearLayout.HORIZONTAL);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setBackground(ripple(rounded(Color.TRANSPARENT, 12, context), 12, context));
        row.setPadding(dp(context, 16), dp(context, 14), dp(context, 16), dp(context, 14));
        row.setClickable(true);
        row.setFocusable(true);
        return row;
    }

    // ------------------------------------------------------- selection inputs

    static RadioButton radio(Context context, String label) {
        RadioButton button = new RadioButton(context);
        button.setText(label);
        button.setTextSize(TypedValue.COMPLEX_UNIT_SP, 14);
        button.setTextColor(ON_SURFACE);
        button.setButtonTintList(new ColorStateList(
                new int[][]{
                        new int[]{android.R.attr.state_checked},
                        new int[]{}},
                new int[]{PRIMARY, OUTLINE}));
        button.setPadding(0, dp(context, 8), 0, dp(context, 8));
        return button;
    }

    /** MD3 switch row: label on the left, switch on the right. */
    static LinearLayout switchRow(Context context, String label, String key, boolean checked) {
        LinearLayout row = new LinearLayout(context);
        row.setOrientation(LinearLayout.HORIZONTAL);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(0, dp(context, 6), 0, dp(context, 6));

        TextView text = text(context, label, 14, ON_SURFACE);
        row.addView(text, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1f));

        Switch toggle = switchControl(context, checked);
        toggle.setTag(key);
        row.addView(toggle);
        return row;
    }

    /** Bare MD3-tinted switch, for callers that build their own row. */
    static Switch switchControl(Context context, boolean checked) {
        Switch toggle = new Switch(context);
        toggle.setChecked(checked);
        try {
            toggle.setThumbTintList(new ColorStateList(
                    new int[][]{
                            new int[]{android.R.attr.state_checked},
                            new int[]{}},
                    new int[]{ON_PRIMARY, SURFACE_CONTAINER_HIGHEST}));
            toggle.setTrackTintList(new ColorStateList(
                    new int[][]{
                            new int[]{android.R.attr.state_checked},
                            new int[]{}},
                    new int[]{PRIMARY_CONTAINER, OUTLINE_VARIANT}));
        } catch (Throwable ignored) {
            // Tinting is cosmetic; never fail the screen over it.
        }
        return toggle;
    }

    // ---------------------------------------------------------------- dialogs

    /**
     * MD3 dialog: 28dp rounded surface container, headline, scrollable body and
     * right-aligned text buttons.
     */
    static Dialog dialog(Activity activity, String title, View content,
                         String positive, final Runnable onPositive,
                         String negative, final Runnable onNegative) {
        final Dialog dialog = new Dialog(activity);
        LinearLayout outer = new LinearLayout(activity);
        outer.setOrientation(LinearLayout.VERTICAL);
        outer.setPadding(dp(activity, 24), dp(activity, 24), dp(activity, 24), dp(activity, 24));

        LinearLayout card = new LinearLayout(activity);
        card.setOrientation(LinearLayout.VERTICAL);
        card.setBackground(rounded(SURFACE_CONTAINER_HIGH, 28, activity));
        card.setPadding(dp(activity, 24), dp(activity, 24), dp(activity, 24), dp(activity, 12));

        TextView heading = text(activity, title, 20, ON_SURFACE);
        heading.setTypeface(Typeface.create("sans-serif-medium", Typeface.NORMAL));
        card.addView(heading, new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));

        ScrollView scroll = new ScrollView(activity);
        scroll.addView(content);
        LinearLayout.LayoutParams scrollParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f);
        scrollParams.topMargin = dp(activity, 12);
        card.addView(scroll, scrollParams);

        LinearLayout actions = new LinearLayout(activity);
        actions.setOrientation(LinearLayout.HORIZONTAL);
        actions.setGravity(Gravity.END);
        actions.setPadding(0, dp(activity, 8), 0, 0);

        if (negative != null) {
            Button cancel = textButton(activity, negative);
            cancel.setOnClickListener(new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    dialog.dismiss();
                    if (onNegative != null) {
                        onNegative.run();
                    }
                }
            });
            actions.addView(cancel);
        }
        if (positive != null) {
            Button confirm = textButton(activity, positive);
            confirm.setOnClickListener(new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    dialog.dismiss();
                    if (onPositive != null) {
                        onPositive.run();
                    }
                }
            });
            actions.addView(confirm);
        }
        card.addView(actions);

        outer.addView(card);
        dialog.setContentView(outer);
        Window window = dialog.getWindow();
        if (window != null) {
            window.setBackgroundDrawable(new ColorDrawable(Color.TRANSPARENT));
            window.setLayout(ViewGroup.LayoutParams.MATCH_PARENT,
                    ViewGroup.LayoutParams.WRAP_CONTENT);
        }
        return dialog;
    }

    /** Hairline divider between sections. */
    static View divider(Context context) {
        View line = new View(context);
        line.setBackgroundColor(OUTLINE_VARIANT);
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, Math.max(1, dp(context, 0.5f)));
        params.topMargin = dp(context, 12);
        params.bottomMargin = dp(context, 12);
        line.setLayoutParams(params);
        return line;
    }
}
