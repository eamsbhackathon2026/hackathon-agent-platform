package http

import (
	"context"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// ListMembers returns a page of workspace members.
func (h *IdentityHandler) ListMembers(ctx context.Context, r gen.ListMembersRequestObject) (gen.ListMembersResponseObject, error) {
	page, err := h.members.ListMembers(ctx, requestFrom(ctx).principal, pageRequest(r.Params.Limit, r.Params.Cursor))
	if err != nil {
		return nil, err
	}
	items := make([]gen.Member, len(page.Items))
	for i, u := range page.Items {
		items[i] = userDTO(u)
	}
	return gen.ListMembers200JSONResponse{Items: items, NextCursor: nullableValue(page.NextCursor)}, nil
}

// CreateMember returns the generated temporary password once.
func (h *IdentityHandler) CreateMember(ctx context.Context, r gen.CreateMemberRequestObject) (gen.CreateMemberResponseObject, error) {
	if r.Body == nil {
		return nil, domain.ErrValidation
	}
	member, err := h.members.CreateMember(ctx, requestFrom(ctx).principal, inbound.MemberCreateCommand{Email: string(r.Body.Email), Name: r.Body.Name, Role: domain.Role(r.Body.Role)})
	if err != nil {
		return nil, err
	}
	return gen.CreateMember201JSONResponse{Member: userDTO(member.Member), TemporaryPassword: member.TemporaryPassword}, nil
}

// UpdateMember updates account status and role through the policy service.
func (h *IdentityHandler) UpdateMember(ctx context.Context, r gen.UpdateMemberRequestObject) (gen.UpdateMemberResponseObject, error) {
	if r.Body == nil {
		return nil, domain.ErrValidation
	}
	changes := domain.MemberChanges{Name: r.Body.Name}
	if r.Body.Role != nil {
		v := domain.Role(*r.Body.Role)
		changes.Role = &v
	}
	if r.Body.Status != nil {
		v := domain.UserStatus(*r.Body.Status)
		changes.Status = &v
	}
	member, err := h.members.UpdateMember(ctx, requestFrom(ctx).principal, r.UserId, changes)
	if err != nil {
		return nil, err
	}
	return gen.UpdateMember200JSONResponse(userDTO(member)), nil
}
