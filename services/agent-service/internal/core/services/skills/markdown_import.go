package skills

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"go.yaml.in/yaml/v3"

	"agent-platform/services/agent-service/internal/core/domain"
)

type skillMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func normalizeSkillMarkdown(content []byte) (string, skillMetadata, string, error) {
	if len(content) > domain.MaxSkillContentBytes || !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return "", skillMetadata{}, "", domain.Invalid("file", "SKILL.md phải là UTF-8 và không quá 100 KiB.")
	}
	normalized := strings.TrimPrefix(string(content), "\ufeff")
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	frontmatter, body, hasFrontmatter, err := splitFrontmatter(normalized)
	if err != nil {
		return "", skillMetadata{}, "", err
	}
	metadata := skillMetadata{}
	if hasFrontmatter && yaml.Unmarshal([]byte(frontmatter), &metadata) != nil {
		return "", skillMetadata{}, "", domain.Invalid("file", "YAML frontmatter của SKILL.md không hợp lệ.")
	}
	if strings.TrimSpace(body) == "" {
		return "", skillMetadata{}, "", domain.Invalid("file", "SKILL.md phải có nội dung hướng dẫn.")
	}
	return normalized, metadata, body, nil
}

func splitFrontmatter(content string) (string, string, bool, error) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content, false, nil
	}
	lines := strings.Split(content, "\n")
	for index := 1; index < len(lines); index++ {
		if lines[index] == "---" || lines[index] == "..." {
			return strings.Join(lines[1:index], "\n"), strings.Join(lines[index+1:], "\n"), true, nil
		}
	}
	return "", "", true, domain.Invalid("file", "YAML frontmatter của SKILL.md chưa được đóng.")
}

func instructionBody(content string) (string, error) {
	_, body, _, err := splitFrontmatter(content)
	if err != nil || strings.TrimSpace(body) == "" {
		return "", domain.Invalid("file", "Nội dung skill đã lưu không hợp lệ.")
	}
	return strings.TrimSpace(body), nil
}

func markdownMetadata(body string) (heading, paragraph string) {
	source := []byte(body)
	document := goldmark.DefaultParser().Parse(text.NewReader(source))
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.Heading:
			if heading == "" && typed.Level == 1 {
				heading = markdownNodeText(typed, source)
			}
		case *ast.Paragraph:
			if paragraph == "" {
				paragraph = markdownNodeText(typed, source)
			}
		}
		if heading != "" && paragraph != "" {
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return heading, paragraph
}

func markdownNodeText(node ast.Node, source []byte) string {
	var content strings.Builder
	_ = ast.Walk(node, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || child == node {
			return ast.WalkContinue, nil
		}
		switch typed := child.(type) {
		case *ast.Text:
			content.Write(typed.Value(source))
			if typed.SoftLineBreak() || typed.HardLineBreak() {
				content.WriteByte(' ')
			}
		case *ast.String:
			content.Write(typed.Value)
		case *ast.AutoLink:
			content.Write(typed.Label(source))
		}
		return ast.WalkContinue, nil
	})
	return strings.Join(strings.Fields(content.String()), " ")
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
