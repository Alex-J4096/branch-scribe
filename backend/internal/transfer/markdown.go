package transfer

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

type markdownSection struct {
	Title   *string
	Type    string
	Content string
	Format  string
}

var (
	htmlTagPattern  = regexp.MustCompile(`<[^>]+>`)
	filenamePattern = regexp.MustCompile(`[\\/:*?"<>|]+`)
)

func buildMarkdown(title string, sections []markdownSection) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# %s\n", title)
	for _, section := range sections {
		content := normalizeMarkdownContent(section.Content, section.Format)
		if strings.TrimSpace(content) == "" {
			continue
		}
		if section.Title != nil && strings.TrimSpace(*section.Title) != "" {
			fmt.Fprintf(&builder, "\n## %s\n\n", strings.TrimSpace(*section.Title))
		} else {
			builder.WriteString("\n")
		}
		builder.WriteString(strings.TrimSpace(content))
		builder.WriteString("\n")
	}
	return builder.String()
}

func normalizeMarkdownContent(content, format string) string {
	if format != "html" {
		return content
	}
	replacer := strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n", "</p>", "\n\n", "</h1>", "\n\n", "</h2>", "\n\n", "</li>", "\n")
	return strings.TrimSpace(html.UnescapeString(htmlTagPattern.ReplaceAllString(replacer.Replace(content), "")))
}

func safeFilename(value string) string {
	value = filenamePattern.ReplaceAllString(strings.TrimSpace(value), "-")
	if value == "" {
		return "branchscribe-export.md"
	}
	return value
}
