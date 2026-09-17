package http

import (
	"bytes"
	"context"
	"mime"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
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

type skillDownloadSpy struct {
	inbound.SkillUseCase
	file domain.SkillFile
}

func (s skillDownloadSpy) DownloadSkill(context.Context, domain.Principal, uuid.UUID) (domain.SkillFile, error) {
	return s.file, nil
}

func TestDownloadSkillSendsTheFileWithItsName(t *testing.T) {
	handler := NewSkillHandler(skillDownloadSpy{file: domain.SkillFile{Filename: "Trợ lý.zip", Content: []byte("PK-bytes")}}, nil)
	ctx := context.WithValue(t.Context(), requestContextKey{}, requestInfo{principal: domain.Principal{Kind: domain.PrincipalUser, UserID: uuid.New(), Role: domain.RoleMember}})
	response, err := handler.DownloadSkill(ctx, gen.DownloadSkillRequestObject{SkillId: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err = response.VisitDownloadSkillResponse(recorder); err != nil {
		t.Fatal(err)
	}
	header := recorder.Header().Get("Content-Disposition")
	_, params, err := mime.ParseMediaType(header)
	if err != nil || params["filename"] != "Trợ lý.zip" || !strings.HasPrefix(header, `attachment; filename="Tro ly.zip"`) || recorder.Body.String() != "PK-bytes" || recorder.Header().Get("Content-Length") != "8" {
		t.Fatalf("headers=%v body=%q err=%v", recorder.Header(), recorder.Body.String(), err)
	}
}
