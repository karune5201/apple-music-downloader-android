// Package i18n 提供界面与日志的多语言文案。
//
// 目录是手写的：每条文案一个键（"<区>.<事件>"），带三种语言的写法。
// 区只有四个：app（主流程）、model（选曲/选专辑）、media（转码/歌词/ALAC）、
// net（网络与解密）。键不存在时 T 原样返回键名 —— 漏翻译会直接显示在
// 日志里，比返回空白更容易发现。
//
// 语言由 general.ui-language 配置项决定（默认中文），命令行 --ui-language
// 或环境变量 AMDL_LANG 可临时覆盖；App 设置面板写的就是同一个配置项。
package i18n

import (
	"fmt"
	"sort"
	"strings"
)

// Lang 是支持的语言标记。
type Lang string

const (
	// ZH 简体中文。
	ZH Lang = "zh"
	// JA 日本語。
	JA Lang = "ja"
	// EN English。
	EN Lang = "en"
)

// Default 是没配置时的语言。
const Default = ZH

// All 按展示顺序列出全部语言。
var All = []Lang{ZH, JA, EN}

// Names 是各语言的自称（切换菜单里原样显示，不翻译）。
var Names = map[Lang]string{
	ZH: "简体中文",
	JA: "日本語",
	EN: "English",
}

// entry 是一条文案的三种写法。
type entry struct {
	zh, ja, en string
}

var messages = map[string]entry{}

var current = Default

// register 登记一条文案，由 msg_*.go 在 init 中调用。
//
// 参数顺序固定为 中文、日文、英文、键名。
func register(zh, ja, en string, keys ...string) {
	for _, key := range keys {
		if _, dup := messages[key]; dup {
			panic("i18n: 重复的键 " + key)
		}
		if strings.TrimSpace(key) == "" {
			panic("i18n: 键不能为空")
		}
		messages[key] = entry{zh: zh, ja: ja, en: en}
	}
}

// Parse 把常见写法归一化，认不出来时回退默认语言。
//
//	zh / zh-CN / zh_CN / cn / cht / tw / hk -> ZH
//	ja / jp                                 -> JA
//	en / us / uk / gb                       -> EN
func Parse(tag string) Lang {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(tag, "-_."); i >= 0 {
		tag = tag[:i]
	}
	switch tag {
	case "zh", "cn", "cht", "chs", "tw", "hk", "sg":
		return ZH
	case "ja", "jp":
		return JA
	case "en", "us", "uk", "gb", "au", "ca":
		return EN
	}
	return Default
}

// Set 切换当前语言，返回生效的语言。
func Set(tag string) Lang {
	current = Parse(tag)
	return current
}

// Current 返回当前语言。
func Current() Lang { return current }

// T 取一条文案并按当前语言格式化。
//
// 键不存在时返回键名本身（带参数时附在冒号后面），漏翻译一眼可见。
func T(key string, args ...any) string {
	e, ok := messages[key]
	if !ok {
		if len(args) == 0 {
			return key
		}
		return key + ": " + fmt.Sprint(args...)
	}
	var text string
	switch current {
	case JA:
		text = e.ja
	case EN:
		text = e.en
	default:
		text = e.zh
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

// Has 报告某个键是否登记过。
func Has(key string) bool {
	_, ok := messages[key]
	return ok
}

// Keys 返回全部键（已排序），供测试与调试使用。
func Keys() []string {
	keys := make([]string, 0, len(messages))
	for k := range messages {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Missing 返回 catalog 里缺失的键（由测试传入期望集合时使用）。
func Missing(want []string) []string {
	var out []string
	for _, key := range want {
		if !Has(key) {
			out = append(out, key)
		}
	}
	return out
}
