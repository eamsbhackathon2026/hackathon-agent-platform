package http

import (
	"bytes"
	"mime/multipart"
	"testing"
)

func TestSkillUploadReadsExactlyOneFile(t *testing.T) {
	reader := multipartReader(t, []multipartPart{{name: "file", filename: "writing.md", content: "# Writing"}})
	filename, content, err := skillUpload(reader)
	if err != nil || filename != "writing.md" || string(content) != "# Writing" {
		t.Fatalf("filename=%q content=%q err=%v", filename, content, err)
	}
}

func TestSkillUploadRejectsExtraFieldsOrFiles(t *testing.T) {
	tests := map[string][]multipartPart{
		"field":     {{name: "file", filename: "writing.md", content: "# Writing"}, {name: "note", content: "extra"}},
		"duplicate": {{name: "file", filename: "one.md", content: "# One"}, {name: "file", filename: "two.md", content: "# Two"}},
	}
	for name, parts := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := skillUpload(multipartReader(t, parts)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

type multipartPart struct{ name, filename, content string }

func multipartReader(t *testing.T, parts []multipartPart) *multipart.Reader {
	t.Helper()
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	for _, item := range parts {
		var partWriter interface{ Write([]byte) (int, error) }
		var err error
		if item.filename == "" {
			partWriter, err = writer.CreateFormField(item.name)
		} else {
			partWriter, err = writer.CreateFormFile(item.name, item.filename)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err = partWriter.Write([]byte(item.content)); err != nil {
			t.Fatal(err)
		}
	}
	boundary := writer.Boundary()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return multipart.NewReader(bytes.NewReader(payload.Bytes()), boundary)
}
