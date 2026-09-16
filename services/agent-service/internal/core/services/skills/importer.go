package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"strings"
	"unicode/utf8"

	"agent-platform/services/agent-service/internal/core/domain"
)

const maxUploadBytes = 2 * 1024 * 1024

func parseUpload(filename string, payload []byte) (domain.Skill, error) {
	safeFilename, err := uploadFilename(filename)
	if err != nil || len(payload) == 0 || len(payload) > maxUploadBytes {
		return domain.Skill{}, domain.Invalid("file", "File skill phải có kích thước từ 1 byte đến 2 MiB.")
	}
	extension := strings.ToLower(path.Ext(safeFilename))
	sourceType := domain.SkillSourceMarkdown
	content := payload
	fallbackName := strings.TrimSuffix(safeFilename, path.Ext(safeFilename))
	switch extension {
	case ".md", ".markdown":
	case ".zip":
		sourceType = domain.SkillSourceZIP
		content, fallbackName, err = readSkillArchive(payload, fallbackName)
		if err != nil {
			return domain.Skill{}, err
		}
	default:
		return domain.Skill{}, domain.Invalid("file", "Chỉ hỗ trợ file .md, .markdown hoặc .zip.")
	}
	normalized, metadata, body, err := normalizeSkillMarkdown(content)
	if err != nil {
		return domain.Skill{}, err
	}
	name := strings.TrimSpace(metadata.Name)
	if name == "" {
		name, _ = markdownMetadata(body)
	}
	if name == "" {
		name = fallbackName
	}
	name = strings.TrimSpace(name)
	description := strings.TrimSpace(metadata.Description)
	if description == "" {
		_, description = markdownMetadata(body)
	}
	description = truncateRunes(description, 4000)
	checksum := sha256.Sum256([]byte(normalized))
	skill := domain.Skill{Name: name, Description: description, SourceType: sourceType, SourceFilename: safeFilename, Content: normalized, Checksum: hex.EncodeToString(checksum[:])}
	if err = domain.ValidateSkill(skill); err != nil {
		return domain.Skill{}, err
	}
	return skill, nil
}

func uploadFilename(filename string) (string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/")
	if normalized == "" || strings.ContainsRune(normalized, 0) {
		return "", domain.ErrValidation
	}
	base := path.Base(normalized)
	if base == "." || base == "/" || !utf8.ValidString(base) || utf8.RuneCountInString(base) > 255 {
		return "", domain.ErrValidation
	}
	return base, nil
}
