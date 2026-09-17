package http

import (
	"fmt"
	"mime"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// attachmentDisposition names a download for every client: filename carries an ASCII
// form with diacritics removed ("Trợ lý.zip" becomes "Tro ly.zip") for clients that only
// read that parameter, and filename* carries the exact UTF-8 name (RFC 6266).
func attachmentDisposition(filename string) string {
	fallback := mime.FormatMediaType("attachment", map[string]string{"filename": asciiFilename(filename)})
	if fallback == "" {
		fallback = `attachment; filename="download"`
	}
	return fallback + "; filename*=UTF-8''" + encodeExtValue(filename)
}

// asciiFilename strips diacritics and replaces anything else outside printable ASCII.
func asciiFilename(name string) string {
	var builder strings.Builder
	for _, r := range norm.NFD.String(name) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case r == 'đ':
			builder.WriteRune('d')
		case r == 'Đ':
			builder.WriteRune('D')
		case r >= 0x20 && r < 0x7f:
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	if result := strings.TrimSpace(builder.String()); result != "" {
		return result
	}
	return "download"
}

// encodeExtValue percent-encodes every byte outside RFC 8187 attr-char.
func encodeExtValue(value string) string {
	var builder strings.Builder
	for _, b := range []byte(value) {
		if b < 0x80 && (b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || strings.IndexByte("!#$&+-.^_`|~", b) >= 0) {
			builder.WriteByte(b)
		} else {
			fmt.Fprintf(&builder, "%%%02X", b)
		}
	}
	return builder.String()
}
