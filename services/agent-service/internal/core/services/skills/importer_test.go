package skills

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"strconv"
	"strings"
	"testing"
)

func TestParseUploadNormalizesMarkdownMetadata(t *testing.T) {
	skill, err := parseUpload("review.markdown", []byte("\ufeff---\r\nname: Code review\r\ndescription: Check risky changes.\r\n---\r\n# Instructions\r\nReview the diff.\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "Code review" || skill.Description != "Check risky changes." || skill.SourceFilename != "review.markdown" || strings.Contains(skill.Content, "\r") {
		t.Fatalf("unexpected skill: %+v", skill)
	}
	if len(skill.Checksum) != 64 {
		t.Fatalf("checksum length=%d", len(skill.Checksum))
	}
}

func TestParseUploadUsesNestedSkillFromZIP(t *testing.T) {
	payload := zipPayload(t, []zipEntry{{name: "writing/SKILL.md", content: "# Clear writing\n\nUse short sentences."}, {name: "writing/example.txt", content: "ignored"}})
	skill, err := parseUpload("writing.zip", payload)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "Clear writing" || skill.SourceType != "zip" || skill.Content != "# Clear writing\n\nUse short sentences." {
		t.Fatalf("unexpected skill: %+v", skill)
	}
}

func TestParseUploadRejectsUnsafeOrAmbiguousZIP(t *testing.T) {
	tests := map[string][]zipEntry{
		"traversal":           {{name: "../SKILL.md", content: "# Unsafe"}},
		"backslash traversal": {{name: "..\\SKILL.md", content: "# Unsafe"}},
		"cleaned traversal":   {{name: "nested/../../SKILL.md", content: "# Unsafe"}},
		"absolute":            {{name: "/SKILL.md", content: "# Unsafe"}},
		"windows absolute":    {{name: "C:\\SKILL.md", content: "# Unsafe"}},
		"duplicate":           {{name: "one/SKILL.md", content: "# One"}, {name: "two/SKILL.md", content: "# Two"}},
		"missing":             {{name: "README.md", content: "# Missing"}},
		"symlink":             {{name: "SKILL.md", content: "target", mode: fs.ModeSymlink | 0o777}},
	}
	for name, entries := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseUpload(name+".zip", zipPayload(t, entries)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestParseUploadRejectsInvalidFrontmatterAndOversizedSkill(t *testing.T) {
	if _, err := parseUpload("broken.md", []byte("---\nname: [\n---\nBody")); err == nil {
		t.Fatal("expected invalid YAML")
	}
	if _, err := parseUpload("large.md", bytes.Repeat([]byte("a"), 100*1024+1)); err == nil {
		t.Fatal("expected oversized content error")
	}
	if _, err := parseUpload("nul.md", []byte("# Valid\n\x00")); err == nil {
		t.Fatal("expected NUL validation error")
	}
	if _, err := parseUpload("invalid.md", []byte{'#', ' ', 0xff}); err == nil {
		t.Fatal("expected UTF-8 validation error")
	}
}

func TestParseUploadUsesMarkdownASTForFallbackMetadata(t *testing.T) {
	skill, err := parseUpload("fallback.md", []byte("```sh\n# Not a heading\necho hidden\n```\n\n# Actual heading\n\nActual description."))
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "Actual heading" || skill.Description != "Actual description." {
		t.Fatalf("skill=%+v", skill)
	}
}

func TestParseUploadRejectsOversizedExpandedZIP(t *testing.T) {
	payload := zipPayload(t, []zipEntry{
		{name: "SKILL.md", content: "# Valid\n\nInstructions."},
		{name: "asset.bin", content: strings.Repeat("x", maxArchiveExpandedBytes)},
	})
	if _, err := parseUpload("expanded.zip", payload); err == nil {
		t.Fatal("expected expanded ZIP validation error")
	}
}

func TestParseUploadAcceptsExactContentAndArchiveEntryLimits(t *testing.T) {
	content := bytes.Repeat([]byte("a"), 100*1024)
	if _, err := parseUpload("exact.md", content); err != nil {
		t.Fatalf("exact content limit: %v", err)
	}
	entries := make([]zipEntry, 0, maxArchiveEntries)
	entries = append(entries, zipEntry{name: "bundle/SKILL.md", content: "# Exact entries\n\nValid."})
	for index := 1; index < maxArchiveEntries; index++ {
		entries = append(entries, zipEntry{name: "bundle/asset-" + strconv.Itoa(index) + ".txt"})
	}
	if _, err := parseUpload("exact.zip", zipPayload(t, entries)); err != nil {
		t.Fatalf("exact entry limit: %v", err)
	}
	entries = append(entries, zipEntry{name: "one-too-many.txt"})
	if _, err := parseUpload("too-many.zip", zipPayload(t, entries)); err == nil {
		t.Fatal("expected entry limit validation error")
	}
}

type zipEntry struct {
	name, content string
	mode          fs.FileMode
}

func zipPayload(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var payload bytes.Buffer
	writer := zip.NewWriter(&payload)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		part, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write([]byte(entry.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return payload.Bytes()
}
