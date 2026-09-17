package skills

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

func TestReplaceAgentSkillsEnforcesAuthorizationIdentityAndBudget(t *testing.T) {
	service, repository, agentID, admin := skillServiceFixture(t)
	first, second := validSkill("First", strings.Repeat("a", 60*1024)), validSkill("Second", strings.Repeat("b", 60*1024))
	repository.skills[first.ID], repository.skills[second.ID] = first, second

	member := admin
	member.Role = domain.RoleMember
	if _, err := service.ReplaceAgentSkills(t.Context(), member, agentID, domain.AgentSkillBindings{SkillIDs: []uuid.UUID{first.ID}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member err=%v", err)
	}
	if _, err := service.ReplaceAgentSkills(t.Context(), admin, uuid.New(), domain.AgentSkillBindings{SkillIDs: []uuid.UUID{first.ID}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown agent err=%v", err)
	}
	if _, err := service.ReplaceAgentSkills(t.Context(), admin, agentID, domain.AgentSkillBindings{SkillIDs: []uuid.UUID{first.ID, first.ID}}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("duplicate err=%v", err)
	}
	if _, err := service.ReplaceAgentSkills(t.Context(), admin, agentID, domain.AgentSkillBindings{SkillIDs: []uuid.UUID{first.ID, second.ID}}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("budget err=%v", err)
	}
	bindings, err := service.ReplaceAgentSkills(t.Context(), admin, agentID, domain.AgentSkillBindings{SkillIDs: []uuid.UUID{first.ID}})
	if err != nil || len(bindings.SkillIDs) != 1 || bindings.SkillIDs[0] != first.ID {
		t.Fatalf("bindings=%+v err=%v", bindings, err)
	}
}

func TestImportSkillStoresCanonicalMetadataAndBlocksMember(t *testing.T) {
	service, repository, _, admin := skillServiceFixture(t)
	member := admin
	member.Role = domain.RoleMember
	command := skillImportCommand("guide.md", "---\nname: Guide\ndescription: Shared guidance.\n---\nDo the work.")
	if _, err := service.ImportSkill(t.Context(), member, command); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member err=%v", err)
	}
	skill, err := service.ImportSkill(t.Context(), admin, command)
	if err != nil || skill.Name != "Guide" || repository.skills[skill.ID].Checksum != skill.Checksum {
		t.Fatalf("skill=%+v err=%v", skill, err)
	}
}

func TestReplaceAgentSkillsAcceptsExactLimitsAndEmptyList(t *testing.T) {
	service, repository, agentID, admin := skillServiceFixture(t)
	ids := make([]uuid.UUID, 0, domain.MaxAgentSkills+1)
	for index := 0; index < domain.MaxAgentSkills+1; index++ {
		skill := validSkill("Skill "+string(rune('A'+index)), "instruction")
		repository.skills[skill.ID] = skill
		ids = append(ids, skill.ID)
	}
	if bindings, err := service.ReplaceAgentSkills(t.Context(), admin, agentID, domain.AgentSkillBindings{SkillIDs: ids[:domain.MaxAgentSkills]}); err != nil || len(bindings.SkillIDs) != domain.MaxAgentSkills {
		t.Fatalf("exact count bindings=%+v err=%v", bindings, err)
	}
	if _, err := service.ReplaceAgentSkills(t.Context(), admin, agentID, domain.AgentSkillBindings{SkillIDs: ids}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("over count err=%v", err)
	}
	exact := validSkill("Exact budget", strings.Repeat("x", domain.MaxAgentSkillBytes))
	repository.skills[exact.ID] = exact
	if _, err := service.ReplaceAgentSkills(t.Context(), admin, agentID, domain.AgentSkillBindings{SkillIDs: []uuid.UUID{exact.ID}}); err != nil {
		t.Fatalf("exact byte budget: %v", err)
	}
	bindings, err := service.ReplaceAgentSkills(t.Context(), admin, agentID, domain.AgentSkillBindings{})
	if err != nil || len(bindings.SkillIDs) != 0 {
		t.Fatalf("empty bindings=%+v err=%v", bindings, err)
	}
}

func skillImportCommand(filename, content string) inbound.SkillImportCommand {
	return inbound.SkillImportCommand{Filename: filename, Content: []byte(content)}
}

func skillServiceFixture(t *testing.T) (*Service, *memorySkillRepository, uuid.UUID, domain.Principal) {
	t.Helper()
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	userID, providerID, agentID := uuid.New(), uuid.New(), uuid.New()
	catalog := fakes.NewCatalogStore()
	if err := catalog.CreateProvider(t.Context(), domain.Provider{ID: providerID, Name: "Provider", Kind: domain.ProviderGemini, Status: domain.ConnectionUnchecked, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.CreateAgent(t.Context(), domain.Agent{ID: agentID, Name: "Agent", ProviderID: providerID, Model: "model", MaxIterations: 8, TimeoutSeconds: 120, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	repository := &memorySkillRepository{skills: map[uuid.UUID]domain.Skill{}, bindings: map[uuid.UUID]domain.AgentSkillBindings{}}
	service, err := NewService(Dependencies{Skills: repository, Agents: catalog, Tx: catalog, Clock: fakes.Clock{Time: now}, IDs: fakes.IDs{}})
	if err != nil {
		t.Fatal(err)
	}
	return service, repository, agentID, domain.Principal{Kind: domain.PrincipalUser, UserID: userID, Role: domain.RoleAdmin}
}

type memorySkillRepository struct {
	skills   map[uuid.UUID]domain.Skill
	bindings map[uuid.UUID]domain.AgentSkillBindings
}

func (*memorySkillRepository) LockSkills(context.Context) error { return nil }
func (r *memorySkillRepository) CreateSkill(_ context.Context, skill domain.Skill) error {
	for _, saved := range r.skills {
		if strings.EqualFold(saved.Name, skill.Name) {
			return domain.ErrConflict
		}
	}
	r.skills[skill.ID] = skill
	return nil
}
func (r *memorySkillRepository) GetSkill(_ context.Context, id uuid.UUID) (domain.Skill, error) {
	skill, ok := r.skills[id]
	if !ok {
		return domain.Skill{}, domain.ErrNotFound
	}
	skill.SourceFileAvailable = len(skill.SourceFile) > 0
	return skill, nil
}
func (r *memorySkillRepository) GetSkillSourceFile(_ context.Context, id uuid.UUID) ([]byte, error) {
	skill, ok := r.skills[id]
	if !ok || len(skill.SourceFile) == 0 {
		return nil, domain.ErrNotFound
	}
	return skill.SourceFile, nil
}
func (r *memorySkillRepository) ListSkills(context.Context, domain.PageOptions) ([]domain.Skill, error) {
	result := make([]domain.Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		result = append(result, skill)
	}
	return result, nil
}
func (r *memorySkillRepository) DeleteSkill(_ context.Context, id uuid.UUID) error {
	if _, ok := r.skills[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.skills, id)
	return nil
}
func (r *memorySkillRepository) GetAgentSkillBindings(_ context.Context, agentID uuid.UUID) (domain.AgentSkillBindings, error) {
	return r.bindings[agentID], nil
}
func (r *memorySkillRepository) ReplaceAgentSkillBindings(_ context.Context, agentID uuid.UUID, bindings domain.AgentSkillBindings) error {
	r.bindings[agentID] = domain.AgentSkillBindings{SkillIDs: append([]uuid.UUID(nil), bindings.SkillIDs...)}
	return nil
}
func (r *memorySkillRepository) ResolveAgentSkills(_ context.Context, agentID uuid.UUID) ([]domain.Skill, error) {
	result := make([]domain.Skill, 0, len(r.bindings[agentID].SkillIDs))
	for _, id := range r.bindings[agentID].SkillIDs {
		result = append(result, r.skills[id])
	}
	return result, nil
}

func TestDownloadSkillReturnsTheUploadOrTheStoredInstructions(t *testing.T) {
	service, repository, _, admin := skillServiceFixture(t)
	payload := zipPayload(t, []zipEntry{{name: "pack/SKILL.md", content: "# Pack\n\nFollow the guide."}, {name: "pack/reference.txt", content: "kept"}})
	imported, err := service.ImportSkill(t.Context(), admin, inbound.SkillImportCommand{Filename: "pack.zip", Content: payload})
	if err != nil || !imported.SourceFileAvailable {
		t.Fatalf("imported=%+v err=%v", imported, err)
	}
	member := admin
	member.Role = domain.RoleMember
	file, err := service.DownloadSkill(t.Context(), member, imported.ID)
	if err != nil || file.Filename != "pack.zip" || !bytes.Equal(file.Content, payload) {
		t.Fatalf("file=%q len=%d err=%v", file.Filename, len(file.Content), err)
	}

	legacy := validSkill("Legacy", "# Legacy")
	legacy.SourceType, legacy.SourceFilename = domain.SkillSourceZIP, "legacy-pack.zip"
	repository.skills[legacy.ID] = legacy
	file, err = service.DownloadSkill(t.Context(), member, legacy.ID)
	if err != nil || file.Filename != "legacy-pack.md" || string(file.Content) != "# Legacy" {
		t.Fatalf("legacy file=%+v err=%v", file, err)
	}
	if _, err = service.DownloadSkill(t.Context(), member, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing err=%v", err)
	}
	if _, err = service.DownloadSkill(t.Context(), domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: uuid.New()}, legacy.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("API key err=%v", err)
	}
}
