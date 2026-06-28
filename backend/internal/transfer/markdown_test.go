package transfer

import (
	"strings"
	"testing"
)

func TestBuildMarkdownConvertsHTMLAndSkipsEmptySections(t *testing.T) {
	title := "第一幕"
	result := buildMarkdown("测试项目", []markdownSection{
		{Title: &title, Content: "<p>雨停了。<br>天亮了。</p>", Format: "html"},
		{Content: "   ", Format: "markdown"},
	})
	if !strings.Contains(result, "# 测试项目") || !strings.Contains(result, "## 第一幕") ||
		!strings.Contains(result, "雨停了。\n天亮了。") {
		t.Fatalf("unexpected markdown: %q", result)
	}
}

func TestSafeFilename(t *testing.T) {
	if got := safeFilename(`项目/主线:终章.md`); strings.ContainsAny(got, `/:`) {
		t.Fatalf("unsafe filename: %q", got)
	}
}
