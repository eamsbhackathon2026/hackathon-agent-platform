package skills

import (
	"context"
	"html"
	"strings"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// ResolveSystemPrompt snapshots bound skills once at run start and inlines their instructions.
func (s *Service) ResolveSystemPrompt(ctx context.Context, agentID uuid.UUID, basePrompt string, specs []domain.ToolSpec) (string, error) {
	items, err := s.Skills.ResolveAgentSkills(ctx, agentID)
	if err != nil || len(items) == 0 {
		return basePrompt, err
	}
	var section strings.Builder
	section.WriteString("## Skills\n\nThe following skills are configured for this assistant. Follow each skill when it is relevant to the user's request.\n")
	total := 0
	for _, skill := range items {
		body, bodyErr := instructionBody(skill.Content)
		if bodyErr != nil {
			return "", bodyErr
		}
		// Rewriting before the budget check keeps the ceiling measured against the text
		// that actually reaches the prompt, since a run-time name is rarely the same
		// length as the reference it replaces.
		body = domain.RewriteSkillToolRefs(body, specs)
		total += len(body)
		if total > domain.MaxAgentSkillBytes {
			return "", domain.Invalid("skill_ids", "Tổng nội dung skill của trợ lý không được vượt quá 100 KiB.")
		}
		section.WriteString("\n<skill_instructions name=\"")
		section.WriteString(html.EscapeString(skill.Name))
		section.WriteString("\"><![CDATA[\n")
		section.WriteString(strings.ReplaceAll(body, "]]>", "]]]]><![CDATA[>"))
		section.WriteString("\n]]></skill_instructions>\n")
	}
	if strings.TrimSpace(basePrompt) == "" {
		return section.String(), nil
	}
	return basePrompt + "\n\n" + section.String(), nil
}

var _ outbound.SkillResolver = (*Service)(nil)
