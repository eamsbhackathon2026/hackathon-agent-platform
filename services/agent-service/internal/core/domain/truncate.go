package domain

// TruncateUTF8 limits a string by bytes without splitting a UTF-8 code point.
func TruncateUTF8(value string, maxBytes int) (string, bool) {
	if maxBytes < 0 {
		maxBytes = 0
	}
	if len(value) <= maxBytes {
		return value, false
	}
	end := maxBytes
	for end > 0 && value[end]&0xc0 == 0x80 {
		end--
	}
	return value[:end], true
}
