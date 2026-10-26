package i18n

import (
	"regexp"
	"strings"
	"testing"
)

var validKey = regexp.MustCompile(`^(app|model|media|net)\.[a-z0-9_]+(\.[a-z0-9_]+)*$`)

var verbs = regexp.MustCompile(`%[-+# 0-9.*]*[a-zA-Z]`)

func TestCatalogIsNotEmpty(t *testing.T) {
	if len(Keys()) < 50 {
		t.Fatalf("目录只有 %d 条，像是没加载 msg_*.go", len(Keys()))
	}
}

func TestKeyNaming(t *testing.T) {
	for _, key := range Keys() {
		if !validKey.MatchString(key) {
			t.Errorf("键名不合规: %q（应为 <区>.<事件>，区取 app/model/media/net）", key)
		}
	}
}

// 三语必须齐全，而且 %s/%d 的数量与顺序要一致，否则运行时会输出 %!d(MISSING)。
func TestPlaceholdersMatchAcrossLanguages(t *testing.T) {
	for key, e := range messages {
		zh := verbs.FindAllString(e.zh, -1)
		ja := verbs.FindAllString(e.ja, -1)
		en := verbs.FindAllString(e.en, -1)
		if e.zh == "" || e.ja == "" || e.en == "" {
			t.Errorf("%s: 有语言为空", key)
			continue
		}
		if len(zh) != len(ja) || len(zh) != len(en) {
			t.Errorf("%s: 占位符数量不一致 zh=%v ja=%v en=%v", key, zh, ja, en)
			continue
		}
		for i := range zh {
			if zh[i] != ja[i] || zh[i] != en[i] {
				t.Errorf("%s: 第 %d 个占位符不一致 zh=%v ja=%v en=%v", key, i+1, zh, ja, en)
				break
			}
		}
	}
}

func TestParseAcceptsCommonTags(t *testing.T) {
	cases := map[string]Lang{
		"zh": ZH, "zh-CN": ZH, "zh_CN": ZH, "CN": ZH, "tw": ZH,
		"ja": JA, "ja-JP": JA, "jp": JA,
		"en": EN, "en-US": EN, "us": EN, "GB": EN,
		"": Default, "klingon": Default,
	}
	for tag, want := range cases {
		if got := Parse(tag); got != want {
			t.Errorf("Parse(%q) = %q, 期望 %q", tag, got, want)
		}
	}
}

func TestTAppliesArguments(t *testing.T) {
	defer Set(string(Default))
	register("下载 %s（%d 首）", "%s をダウンロード（%d 曲）", "downloading %s (%d tracks)", "app.test_args")

	Set("zh")
	if got := T("app.test_args", "专辑", 12); got != "下载 专辑（12 首）" {
		t.Fatalf("zh = %q", got)
	}
	Set("ja")
	if got := T("app.test_args", "アルバム", 12); got != "アルバム をダウンロード（12 曲）" {
		t.Fatalf("ja = %q", got)
	}
	Set("en")
	if got := T("app.test_args", "Album", 12); got != "downloading Album (12 tracks)" {
		t.Fatalf("en = %q", got)
	}
}

func TestUnknownKeyReturnsKey(t *testing.T) {
	if got := T("app.definitely_not_registered"); got != "app.definitely_not_registered" {
		t.Fatalf("未知键应返回键名，得到 %q", got)
	}
}

func TestKeysAreSorted(t *testing.T) {
	keys := Keys()
	for i := 1; i < len(keys); i++ {
		if strings.Compare(keys[i-1], keys[i]) > 0 {
			t.Fatalf("Keys() 未排序: %q 在 %q 之后", keys[i-1], keys[i])
		}
	}
}
