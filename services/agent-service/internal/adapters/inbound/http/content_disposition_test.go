package http

import (
	"mime"
	"strings"
	"testing"
)

func TestAttachmentDispositionOffersASCIIAndExactNames(t *testing.T) {
	tests := map[string]struct{ name, ascii string }{
		"vietnamese": {"Trợ lý tài chính Đông Á.zip", "Tro ly tai chinh Dong A.zip"},
		"plain":      {"guide.md", "guide.md"},
		"quotes":     {`a "b" \c.md`, `a "b" \c.md`},
		"line break": {"a\r\nX-Evil: 1.zip", "a__X-Evil: 1.zip"},
		"no letters": {"数据.md", "__.md"},
		"empty":      {"", "download"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			header := attachmentDisposition(test.name)
			if strings.ContainsAny(header, "\r\n") {
				t.Fatalf("header breaks lines: %q", header)
			}
			for _, r := range header {
				if r > 0x7e {
					t.Fatalf("header is not ASCII: %q", header)
				}
			}
			disposition, params, err := mime.ParseMediaType(header)
			if err != nil || disposition != "attachment" {
				t.Fatalf("header=%q err=%v", header, err)
			}
			// Go's parser prefers filename*, so check the ASCII part on its own.
			_, fallback, err := mime.ParseMediaType(header[:strings.Index(header, "; filename*=")])
			if err != nil || fallback["filename"] != test.ascii {
				t.Fatalf("fallback=%q want %q err=%v", fallback["filename"], test.ascii, err)
			}
			if test.name != "" && params["filename"] != test.name {
				t.Fatalf("exact=%q want %q", params["filename"], test.name)
			}
		})
	}
}
