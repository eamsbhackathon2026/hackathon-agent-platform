package http

import (
	"context"
	"errors"
	"io"
	"mime/multipart"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

const maxSkillUploadBytes = 2 * 1024 * 1024

// SkillHandler translates Skill Hub and assistant binding APIs to use cases.
type SkillHandler struct {
	skills   inbound.SkillUseCase
	bindings inbound.AgentSkillBindingUseCase
}

// NewSkillHandler creates the strict HTTP adapter for Skill Hub.
func NewSkillHandler(skills inbound.SkillUseCase, bindings inbound.AgentSkillBindingUseCase) *SkillHandler {
	return &SkillHandler{skills: skills, bindings: bindings}
}

// ListSkills handles cursor-paginated retrieval of the shared skill library.
func (h *SkillHandler) ListSkills(ctx context.Context, request gen.ListSkillsRequestObject) (gen.ListSkillsResponseObject, error) {
	page, err := h.skills.ListSkills(ctx, requestFrom(ctx).principal, pageRequest(request.Params.Limit, request.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]gen.SkillSummary, len(page.Items))
	for index, skill := range page.Items {
		items[index] = skillSummaryDTO(skill)
	}
	return gen.ListSkills200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}

// ImportSkill accepts exactly one bounded file part named file.
func (h *SkillHandler) ImportSkill(ctx context.Context, request gen.ImportSkillRequestObject) (gen.ImportSkillResponseObject, error) {
	filename, content, err := skillUpload(request.Body)
	if err != nil {
		return nil, err
	}
	skill, err := h.skills.ImportSkill(ctx, requestFrom(ctx).principal, inbound.SkillImportCommand{Filename: filename, Content: content})
	if err != nil {
		return nil, err
	}
	return gen.ImportSkill201JSONResponse(skillDTO(skill)), nil
}

// GetSkill returns one skill including canonical Markdown content.
func (h *SkillHandler) GetSkill(ctx context.Context, request gen.GetSkillRequestObject) (gen.GetSkillResponseObject, error) {
	skill, err := h.skills.GetSkill(ctx, requestFrom(ctx).principal, request.SkillId)
	if err != nil {
		return nil, err
	}
	return gen.GetSkill200JSONResponse(skillDTO(skill)), nil
}

// DeleteSkill removes one skill and all bindings through database cascades.
func (h *SkillHandler) DeleteSkill(ctx context.Context, request gen.DeleteSkillRequestObject) (gen.DeleteSkillResponseObject, error) {
	err := h.skills.DeleteSkill(ctx, requestFrom(ctx).principal, request.SkillId)
	return gen.DeleteSkill204Response{}, err
}

// GetAgentSkills returns skills enabled for an assistant.
func (h *SkillHandler) GetAgentSkills(ctx context.Context, request gen.GetAgentSkillsRequestObject) (gen.GetAgentSkillsResponseObject, error) {
	bindings, err := h.bindings.GetAgentSkills(ctx, requestFrom(ctx).principal, request.AgentId)
	if err != nil {
		return nil, err
	}
	return gen.GetAgentSkills200JSONResponse(skillBindingsDTO(bindings)), nil
}

// ReplaceAgentSkills atomically replaces all enabled skills for an assistant.
func (h *SkillHandler) ReplaceAgentSkills(ctx context.Context, request gen.ReplaceAgentSkillsRequestObject) (gen.ReplaceAgentSkillsResponseObject, error) {
	if request.Body == nil {
		return nil, domain.ErrValidation
	}
	bindings, err := h.bindings.ReplaceAgentSkills(ctx, requestFrom(ctx).principal, request.AgentId, domain.AgentSkillBindings{SkillIDs: request.Body.SkillIds})
	if err != nil {
		return nil, err
	}
	return gen.ReplaceAgentSkills200JSONResponse(skillBindingsDTO(bindings)), nil
}

func skillUpload(reader *multipart.Reader) (string, []byte, error) {
	if reader == nil {
		return "", nil, domain.Invalid("file", "Hãy chọn một file skill để tải lên.")
	}
	var filename string
	var content []byte
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", nil, domain.Invalid("file", "Không thể đọc dữ liệu tải lên.")
		}
		if part.FormName() != "file" || part.FileName() == "" || filename != "" {
			_ = part.Close()
			return "", nil, domain.Invalid("file", "Yêu cầu phải chứa đúng một file skill.")
		}
		payload, readErr := io.ReadAll(io.LimitReader(part, maxSkillUploadBytes+1))
		closeErr := part.Close()
		if readErr != nil || closeErr != nil || len(payload) > maxSkillUploadBytes {
			return "", nil, domain.Invalid("file", "File skill không được vượt quá 2 MiB.")
		}
		filename, content = part.FileName(), payload
	}
	if filename == "" {
		return "", nil, domain.Invalid("file", "Hãy chọn một file skill để tải lên.")
	}
	return filename, content, nil
}
