// Package logx 是全部终端输出的唯一出口。
//
// 设计要点：
//
//   - 级别靠**行首标记**表达（▸ 步骤、✔ 成功、⚠ 警告、✖ 错误、· 细节），
//     不依赖 ANSI 颜色。Android 端会把转义序列剥掉、只按标记上色，
//     所以标记本身就是协议的一部分，不要随手改字符。
//   - 日志一律走本包，方便统一空行折叠、缩进和 --verbose 开关。
//   - 多语言文案由 internal/i18n 提供，带 T 后缀的 helper 负责翻译后再输出；
//     不带 T 的用于文件名、URL、codec 名这类不翻译的内容。
package logx

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"

	"amdl/internal/i18n"
)

// 行首标记。改这些字符等于改 App 的上色规则，务必同时改 android-native。
const (
	MarkerStep   = "\u25b8" // ▸
	MarkerOK     = "\u2714" // ✔
	MarkerWarn   = "\u26a0" // ⚠
	MarkerError  = "\u2716" // ✖
	MarkerDetail = "\u00b7" // ·
)

// Kind 是一条日志的级别。
type Kind int

const (
	// KindPlain 普通正文，不加标记。
	KindPlain Kind = iota
	// KindStep 正在做的事（▸）。
	KindStep
	// KindOK 成功（✔）。
	KindOK
	// KindWarn 可继续的异常（⚠）。
	KindWarn
	// KindError 失败（✖）。
	KindError
	// KindDetail 补充细节，只在需要时看（·）。
	KindDetail
)

var markerOf = map[Kind]string{
	KindStep:   MarkerStep,
	KindOK:     MarkerOK,
	KindWarn:   MarkerWarn,
	KindError:  MarkerError,
	KindDetail: MarkerDetail,
}

var (
	mu         sync.Mutex
	out        io.Writer = os.Stdout
	verbose    bool
	beforeLine func()
	lastBlank  bool
)

func init() {
	// 输出不是终端时（管道、Android App）不上色，反正 App 会剥掉转义。
	if f, ok := out.(*os.File); ok {
		if st, err := f.Stat(); err == nil && st.Mode()&os.ModeCharDevice == 0 {
			color.NoColor = true
		}
	}
}

// SetOutput 改变输出目标（测试用）。
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	out = w
}

// SetVerbose 打开/关闭 Debug 级别输出（--verbose）。
func SetVerbose(v bool) {
	mu.Lock()
	defer mu.Unlock()
	verbose = v
}

// Verbose 报告是否处于 --verbose。
func Verbose() bool {
	mu.Lock()
	defer mu.Unlock()
	return verbose
}

// SetBeforeLine 注册“写日志前先执行”的回调。
//
// 进度条用 \r 重画同一行，日志落在这行中间会把进度条劈开；调用方
// 在这里把进度条清掉（bar.Clear），本包随后补一个 \r。
func SetBeforeLine(f func()) {
	mu.Lock()
	defer mu.Unlock()
	beforeLine = f
}

// leadingKind 看这行是不是已经带了标记。
//
// 有些调用点把标记写在文案里（例如 "⚠ 歌词缺失"），这时按文案自己的
// 标记定级，别再补一个，否则会变成 "✔ ⚠ …"。
func leadingKind(s string) (Kind, bool) {
	switch {
	case strings.HasPrefix(s, MarkerError):
		return KindError, true
	case strings.HasPrefix(s, MarkerWarn):
		return KindWarn, true
	case strings.HasPrefix(s, MarkerOK):
		return KindOK, true
	case strings.HasPrefix(s, MarkerStep):
		return KindStep, true
	case strings.HasPrefix(s, MarkerDetail):
		return KindDetail, true
	}
	return KindPlain, false
}

func decorate(kind Kind, text string) string {
	if kind == KindPlain || text == "" {
		return text
	}
	if _, ok := leadingKind(text); ok {
		return text
	}
	marker := markerOf[kind]
	if marker == "" {
		return text
	}
	line := marker + " " + text
	if color.NoColor {
		return line
	}
	switch kind {
	case KindStep:
		return color.CyanString(line)
	case KindOK:
		return color.GreenString(line)
	case KindWarn:
		return color.YellowString(line)
	case KindError:
		return color.New(color.FgRed, color.Bold).Sprint(line)
	case KindDetail:
		return color.New(color.Faint).Sprint(line)
	}
	return line
}

func write(kind Kind, text string) {
	text = strings.TrimRight(text, "\n")
	mu.Lock()
	defer mu.Unlock()
	hook := beforeLine
	if hook != nil {
		hook()
	}
	line := decorate(kind, text)
	if text == "" {
		// 连续空行压成一行，日志里成片留白没什么用
		if lastBlank {
			return
		}
		lastBlank = true
	} else {
		lastBlank = false
	}
	if hook != nil && line != "" {
		// 进度条被清掉后，让这行从行首开始写
		line = "\r" + line
	}
	fmt.Fprintln(out, line)
}

// ---------------------------------------------------------------- 输出接口

// Println 输出一行正文（不加标记）。
func Println(args ...any) { write(KindPlain, fmt.Sprint(args...)) }

// Print 原样输出，不换行、不加标记（进度行、JSON 用）。
func Print(args ...any) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Fprint(out, fmt.Sprint(args...))
}

// Printf 原样格式化输出，不换行、不加标记。
func Printf(format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Fprintf(out, format, args...)
}

// Step 输出“正在做”的一步。
func Step(format string, args ...any) { write(KindStep, fmt.Sprintf(format, args...)) }

// OK 输出成功。
func OK(format string, args ...any) { write(KindOK, fmt.Sprintf(format, args...)) }

// Warn 输出可继续的异常。
func Warn(format string, args ...any) { write(KindWarn, fmt.Sprintf(format, args...)) }

// Error 输出失败。
func Error(format string, args ...any) { write(KindError, fmt.Sprintf(format, args...)) }

// Detail 输出补充细节。
func Detail(format string, args ...any) { write(KindDetail, fmt.Sprintf(format, args...)) }

// Debug 仅在 --verbose 时输出。
func Debug(format string, args ...any) {
	if !Verbose() {
		return
	}
	write(KindDetail, fmt.Sprintf(format, args...))
}

// Debugf 是 Debug 的别名，保留旧调用点的写法。
func Debugf(format string, args ...any) { Debug(format, args...) }

// Banner 输出一条分节标题，例如 “──── 下载 ────”。
func Banner(title string) {
	write(KindPlain, "")
	write(KindPlain, rule(title))
}

const bannerWidth = 60

func rule(title string) string {
	if title == "" {
		return strings.Repeat("\u2500", bannerWidth)
	}
	pad := bannerWidth - len([]rune(title)) - 2
	if pad < 4 {
		pad = 4
	}
	left := pad / 2
	right := pad - left
	return strings.Repeat("\u2500", left) + " " + title + " " + strings.Repeat("\u2500", right)
}

// ------------------------------------------------------------ 翻译版助手
//
// 全部走 internal/i18n：键不存在时原样显示键名，方便一眼看出漏翻译。

// StepT 输出“正在做”的一步（翻译版）。
func StepT(key string, args ...any) { Step("%s", i18n.T(key, args...)) }

// OKT 输出成功（翻译版）。
func OKT(key string, args ...any) { OK("%s", i18n.T(key, args...)) }

// WarnT 输出可继续的异常（翻译版）。
func WarnT(key string, args ...any) { Warn("%s", i18n.T(key, args...)) }

// ErrorT 输出失败（翻译版）。
func ErrorT(key string, args ...any) { Error("%s", i18n.T(key, args...)) }

// DetailT 输出补充细节（翻译版）。
func DetailT(key string, args ...any) { Detail("%s", i18n.T(key, args...)) }

// DebugT 仅在 --verbose 时输出（翻译版）。
func DebugT(key string, args ...any) {
	if !Verbose() {
		return
	}
	write(KindDetail, i18n.T(key, args...))
}

// BannerT 输出分节标题（翻译版）。
func BannerT(key string, args ...any) { Banner(i18n.T(key, args...)) }

// PlainT 输出一行正文（翻译版）。
func PlainT(key string, args ...any) { write(KindPlain, i18n.T(key, args...)) }

// ------------------------------------------------------------ 数值格式化

// Bytes 把字节数格式化成 1.5 MB 这种形式。
func Bytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// Rate 把吞吐量格式化成 2.0 MB/s。
func Rate(n int64, d time.Duration) string {
	if d <= 0 {
		return Bytes(n) + "/s"
	}
	return fmt.Sprintf("%s/s", Bytes(int64(float64(n)/d.Seconds())))
}

// Duration 把耗时格式化成 1m23s。
func Duration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	d = d.Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	if minutes < 60 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	hours := minutes / 60
	return fmt.Sprintf("%dh%dm", hours, minutes%60)
}

// Since 是 Duration(time.Since(t)) 的简写。
func Since(t time.Time) string { return Duration(time.Since(t)) }

// KV 拼一个 k=v 片段，便于在长行里塞附加信息。
func KV(key string, value any) string { return fmt.Sprintf("%s=%v", key, value) }
