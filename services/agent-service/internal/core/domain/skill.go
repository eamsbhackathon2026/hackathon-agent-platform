package domain

import (
	"encoding/hex"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// SkillSourceType identifies the uploaded package format.
type SkillSourceType string

// Supported skill upload formats.
const (
	SkillSourceMarkdown  SkillSourceType = "markdown"
	SkillSourceZIP       SkillSourceType = "zip"
	MaxSkillContentBytes                 = 100 * 1024
	MaxAgentSkills                       = 20
	MaxAgentSkillBytes                   = 100 * 1024
)

// Skill is an immutable set of instructions imported into the shared library.
type Skill struct {
	ID                uuid.UUID
	Name, Description string
	SourceType        SkillSourceType
	SourceFilename    string
	Content, Checksum string
	// ToolRefs are the tools the instructions name with a $ sigil, parsed once at
	// import so both the run-time rewrite and the agent screen read the same list.
	ToolRefs             []string
	CreatedBy            uuid.UUID
	CreatedAt, UpdatedAt time.Time
}

// AgentSkillBindings replaces all skills enabled for one active assistant.
type AgentSkillBindings struct{ SkillIDs []uuid.UUID }

// ValidateSkill checks normalized uploaded content before persistence.
func ValidateSkill(skill Skill) error {
	if err := catalogText("name", skill.Name, 1, 200); err != nil {
		return err
	}
	if strings.ContainsAny(skill.Name, "\r\n\x00") {
		return Invalid("name", "Tên skill không được chứa ký tự điều khiển.")
	}
	if err := catalogText("description", skill.Description, 0, 4000); err != nil {
		return err
	}
	if strings.ContainsRune(skill.Description, 0) || strings.ContainsRune(skill.SourceFilename, 0) {
		return Invalid("file", "Metadata skill chứa ký tự không hợp lệ.")
	}
	if skill.SourceType != SkillSourceMarkdown && skill.SourceType != SkillSourceZIP {
		return Invalid("file", "Chỉ hỗ trợ file Markdown hoặc ZIP.")
	}
	if err := catalogText("file", skill.SourceFilename, 1, 255); err != nil {
		return err
	}
	if !utf8.ValidString(skill.Content) || strings.ContainsRune(skill.Content, 0) || strings.TrimSpace(skill.Content) == "" || len(skill.Content) > MaxSkillContentBytes {
		return Invalid("file", "Nội dung skill phải là UTF-8 và không quá 100 KiB.")
	}
	if len(skill.ToolRefs) > MaxSkillToolRefs {
		return Invalid("file", "Skill chỉ được khai báo tối đa "+strconv.Itoa(MaxSkillToolRefs)+" công cụ.")
	}
	if !skillToolRefsValid(skill.ToolRefs) {
		return Invalid("file", "Skill khai báo công cụ với tên không hợp lệ.")
	}
	checksum, err := hex.DecodeString(skill.Checksum)
	if err != nil || len(checksum) != 32 || strings.ToLower(skill.Checksum) != skill.Checksum {
		return Invalid("file", "Checksum skill không hợp lệ.")
	}
	return nil
}
