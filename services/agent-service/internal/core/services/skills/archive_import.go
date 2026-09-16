package skills

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"path"
	"strings"

	"agent-platform/services/agent-service/internal/core/domain"
)

const (
	maxArchiveEntries       = 100
	maxArchiveExpandedBytes = 2 * 1024 * 1024
)

func readSkillArchive(payload []byte, archiveName string) ([]byte, string, error) {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil || len(reader.File) == 0 || len(reader.File) > maxArchiveEntries {
		return nil, "", domain.Invalid("file", "ZIP không hợp lệ hoặc có quá 100 mục.")
	}
	var skillFile *zip.File
	var skillPath string
	var expanded uint64
	for _, file := range reader.File {
		cleanName, cleanErr := safeArchivePath(file.Name)
		if cleanErr != nil || file.Mode()&fs.ModeSymlink != 0 {
			return nil, "", domain.Invalid("file", "ZIP chứa đường dẫn hoặc liên kết không an toàn.")
		}
		if file.UncompressedSize64 > maxArchiveExpandedBytes || expanded > maxArchiveExpandedBytes-file.UncompressedSize64 {
			return nil, "", domain.Invalid("file", "Nội dung giải nén của ZIP vượt quá 2 MiB.")
		}
		expanded += file.UncompressedSize64
		if !file.FileInfo().IsDir() && path.Base(cleanName) == "SKILL.md" {
			if skillFile != nil {
				return nil, "", domain.Invalid("file", "ZIP phải chứa đúng một file SKILL.md.")
			}
			skillFile, skillPath = file, cleanName
		}
	}
	if skillFile == nil {
		return nil, "", domain.Invalid("file", "ZIP phải chứa đúng một file SKILL.md.")
	}
	content, err := verifyArchiveContent(reader.File, skillFile)
	if err != nil {
		return nil, "", err
	}
	fallbackName := archiveName
	if directory := path.Dir(skillPath); directory != "." {
		fallbackName = path.Base(directory)
	}
	return content, fallbackName, nil
}

func verifyArchiveContent(files []*zip.File, skillFile *zip.File) ([]byte, error) {
	var content []byte
	actualExpanded := int64(0)
	for _, file := range files {
		if file.FileInfo().IsDir() {
			continue
		}
		opened, openErr := file.Open()
		if openErr != nil {
			return nil, domain.Invalid("file", "Không thể đọc nội dung trong ZIP.")
		}
		destination := io.Discard
		var skillContent bytes.Buffer
		if file == skillFile {
			destination = &skillContent
		}
		remaining := int64(maxArchiveExpandedBytes) - actualExpanded
		copied, copyErr := io.Copy(destination, io.LimitReader(opened, remaining+1))
		closeErr := opened.Close()
		if copied > remaining {
			return nil, domain.Invalid("file", "Nội dung giải nén của ZIP vượt quá 2 MiB.")
		}
		if copyErr != nil || closeErr != nil {
			return nil, domain.Invalid("file", "Không thể xác minh nội dung trong ZIP.")
		}
		actualExpanded += copied
		if file == skillFile {
			if skillContent.Len() > domain.MaxSkillContentBytes {
				return nil, domain.Invalid("file", "SKILL.md phải có kích thước không quá 100 KiB.")
			}
			content = append([]byte(nil), skillContent.Bytes()...)
		}
	}
	return content, nil
}

func safeArchivePath(name string) (string, error) {
	normalized := strings.ReplaceAll(name, "\\", "/")
	if normalized == "" || strings.ContainsRune(normalized, 0) || path.IsAbs(normalized) || len(normalized) >= 3 && normalized[1] == ':' && normalized[2] == '/' {
		return "", domain.ErrValidation
	}
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", domain.ErrValidation
	}
	return cleaned, nil
}
