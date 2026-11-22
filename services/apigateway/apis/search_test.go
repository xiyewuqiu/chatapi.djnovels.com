package apis

import (
	"strings"
	"testing"
)

// 关键词长度必须按 rune 计。UTF-8 下一个汉字 3 字节，
// 按字节算会把"书单"（6 字节）判成合法、把四个字判成超限，全是错的。
func TestNormalizeSearchKeyword(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"中文两字合法", "书单", "书单", true},
		{"中文四字合法", "书单推荐", "书单推荐", true},
		{"单个字不合法", "书", "", false},
		{"空白串不合法", "   ", "", false},
		{"空串不合法", "", "", false},
		{"前后空白被去掉", "  书单  ", "书单", true},
		{"32 个字符合法", strings.Repeat("a", 32), strings.Repeat("a", 32), true},
		{"33 个字符超限", strings.Repeat("a", 33), "", false},
		{"32 个汉字合法", strings.Repeat("书", 32), strings.Repeat("书", 32), true},
		{"33 个汉字超限", strings.Repeat("书", 33), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := NormalizeSearchKeyword(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("NormalizeSearchKeyword(%q) = (%q,%v), want (%q,%v)", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// MsgContent 是 JSON 而不是裸文本；解析失败要返回空串让调用方跳过，
// 不能把原始 JSON 当正文展示给用户。
func TestExtractTextContent(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"正常文本", `{"content":"今晚讨论这本书"}`, "今晚讨论这本书"},
		{"空内容字段", `{"content":""}`, ""},
		{"没有 content 字段", `{"other":"x"}`, ""},
		{"不是 JSON", `今晚讨论这本书`, ""},
		{"空字节", ``, ""},
		{"带转义", `{"content":"他说\"好\""}`, `他说"好"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractTextContent([]byte(tc.in)); got != tc.want {
				t.Fatalf("ExtractTextContent(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSnippetMarksTruncation(t *testing.T) {
	long := strings.Repeat("前", 60) + "书单" + strings.Repeat("后", 60)

	got := Snippet(long, "书单")
	if !strings.Contains(got, "书单") {
		t.Fatalf("摘要应包含关键词，实际=%q", got)
	}
	if !strings.HasPrefix(got, "…") {
		t.Fatalf("关键词前有内容时应加前省略号，实际=%q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("关键词后有内容时应加后省略号，实际=%q", got)
	}
}

func TestSnippetNoLeadingEllipsisAtStart(t *testing.T) {
	got := Snippet("书单在今天讨论" + strings.Repeat("尾", 60), "书单")
	if strings.HasPrefix(got, "…") {
		t.Fatalf("关键词在开头时不该有前省略号，实际=%q", got)
	}
	if !strings.Contains(got, "书单") {
		t.Fatalf("摘要应包含关键词，实际=%q", got)
	}
}

// 关键词理论上不会落空（调用方先过滤过），但摘要函数不能因此崩掉。
func TestSnippetFallsBackWhenKeywordMissing(t *testing.T) {
	got := Snippet(strings.Repeat("文", 100), "书单")
	if got == "" {
		t.Fatal("未命中时也应返回开头片段，而不是空串")
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("截断后应带省略号，实际=%q", got)
	}
}

func TestTrimToRunes(t *testing.T) {
	if got := TrimToRunes("短文本", 10); got != "短文本" {
		t.Fatalf("未超限时不应改动，实际=%q", got)
	}
	got := TrimToRunes(strings.Repeat("文", 20), 5)
	if got != strings.Repeat("文", 5)+"…" {
		t.Fatalf("超限时应按 rune 截断，实际=%q", got)
	}
}

func TestContainsFold(t *testing.T) {
	if !containsFold("Hello World", "hello") {
		t.Fatal("应忽略大小写")
	}
	if !containsFold("今晚讨论书单", "书单") {
		t.Fatal("中文应能命中")
	}
	if containsFold("今晚讨论书单", "影单") {
		t.Fatal("不相关的关键词不应命中")
	}
}

// ---------------- 限流 ----------------

func resetSearchRate(t *testing.T) {
	t.Helper()
	searchRateMu.Lock()
	searchRateSeen = map[string][]int64{}
	searchRateMu.Unlock()
	t.Cleanup(func() {
		searchRateMu.Lock()
		searchRateSeen = map[string][]int64{}
		searchRateMu.Unlock()
	})
}

func TestAllowSearchRateLimit(t *testing.T) {
	resetSearchRate(t)
	const key = "app|user"
	const now int64 = 1762900000000

	for i := 0; i < searchRatePerMinute; i++ {
		if !allowSearch(key, now) {
			t.Fatalf("第 %d 次应被允许", i+1)
		}
	}
	if allowSearch(key, now) {
		t.Fatal("超过每分钟上限后应拒绝")
	}
}

func TestAllowSearchWindowSlides(t *testing.T) {
	resetSearchRate(t)
	const key = "app|slice"
	const now int64 = 1762900000000

	for i := 0; i < searchRatePerMinute; i++ {
		allowSearch(key, now)
	}
	if allowSearch(key, now) {
		t.Fatal("窗口内应拒绝")
	}
	// 一分钟之后，旧的计数全部滑出窗口。
	if !allowSearch(key, now+61_000) {
		t.Fatal("窗口滑过后应重新允许")
	}
}

// 不同用户之间不能互相影响：一个用户刷满不该把别人挡在门外。
func TestAllowSearchIsPerKey(t *testing.T) {
	resetSearchRate(t)
	const now int64 = 1762900000000

	for i := 0; i < searchRatePerMinute; i++ {
		allowSearch("app|noisy", now)
	}
	if allowSearch("app|noisy", now) {
		t.Fatal("刷满的用户应被拒绝")
	}
	if !allowSearch("app|quiet", now) {
		t.Fatal("其他用户不应受影响")
	}
}
