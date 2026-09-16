package skills

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestResolveSystemPromptStripsFrontmatterAndKeepsRepositoryOrder(t *testing.T) {
	repository := &skillRepositoryStub{resolved: []domain.Skill{
		validSkill("Writing & tone", "---\nname: Writing\n---\nUse plain language."),
		validSkill("Safety", "Never expose secrets."),
	}}
	service := &Service{Dependencies: Dependencies{Skills: repository}}
	prompt, err := service.ResolveSystemPrompt(t.Context(), uuid.New(), "You help the user.")
	if err != nil {
		t.Fatal(err)
	}
	want := "You help the user.\n\n## Skills\n\nThe following skills are configured for this assistant. Follow each skill when it is relevant to the user's request.\n\n<skill_instructions name=\"Writing &amp; tone\"><![CDATA[\nUse plain language.\n]]></skill_instructions>\n\n<skill_instructions name=\"Safety\"><![CDATA[\nNever expose secrets.\n]]></skill_instructions>\n"
	if prompt != want {
		t.Fatalf("prompt mismatch\nwant: %q\n got: %q", want, prompt)
	}
}

func TestResolveSystemPromptKeepsSkillBodyInsideCDATA(t *testing.T) {
	repository := &skillRepositoryStub{resolved: []domain.Skill{validSkill("Markup", "Use </skill_instructions> and ]]> literally.")}}
	service := &Service{Dependencies: Dependencies{Skills: repository}}
	prompt, err := service.ResolveSystemPrompt(t.Context(), uuid.New(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "Use </skill_instructions> and ]]]]><![CDATA[> literally.") {
		t.Fatalf("prompt=%q", prompt)
	}
}

func validSkill(name, content string) domain.Skill {
	return domain.Skill{ID: uuid.New(), Name: name, SourceType: domain.SkillSourceMarkdown, SourceFilename: "SKILL.md", Content: content, Checksum: "0000000000000000000000000000000000000000000000000000000000000000", CreatedBy: uuid.New(), CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

type skillRepositoryStub struct{ resolved []domain.Skill }

func (*skillRepositoryStub) LockSkills(context.Context) error                { return nil }
func (*skillRepositoryStub) CreateSkill(context.Context, domain.Skill) error { return nil }
func (*skillRepositoryStub) GetSkill(context.Context, uuid.UUID) (domain.Skill, error) {
	return domain.Skill{}, domain.ErrNotFound
}
func (*skillRepositoryStub) ListSkills(context.Context, domain.PageOptions) ([]domain.Skill, error) {
	return nil, nil
}
func (*skillRepositoryStub) DeleteSkill(context.Context, uuid.UUID) error { return nil }
func (*skillRepositoryStub) GetAgentSkillBindings(context.Context, uuid.UUID) (domain.AgentSkillBindings, error) {
	return domain.AgentSkillBindings{}, nil
}
func (*skillRepositoryStub) ReplaceAgentSkillBindings(context.Context, uuid.UUID, domain.AgentSkillBindings) error {
	return nil
}
func (r *skillRepositoryStub) ResolveAgentSkills(context.Context, uuid.UUID) ([]domain.Skill, error) {
	return r.resolved, nil
}
