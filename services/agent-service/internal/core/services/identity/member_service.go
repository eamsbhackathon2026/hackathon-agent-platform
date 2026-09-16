package identity

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"context"
	"github.com/google/uuid"
	"strings"
)

// ListMembers lists members for authorized workspace managers.
func (s *Service) ListMembers(ctx context.Context, p domain.Principal, page inbound.PageRequest) (inbound.MemberPage, error) {
	var out inbound.MemberPage
	if err := domain.Authorize(p, domain.ActionMembersRead, nil); err != nil {
		return out, err
	}
	options, err := pageOptions(page)
	if err != nil {
		return out, err
	}
	out.Items, err = s.Users.ListUsers(ctx, options)
	if err != nil {
		return out, err
	}
	if len(out.Items) == options.Limit {
		out.Items = out.Items[:options.Limit-1]
		last := out.Items[len(out.Items)-1]
		out.NextCursor = encodeCursor(domain.PageCursor{ID: last.ID, CreatedAt: last.CreatedAt})
	}
	return out, nil
}

// CreateMember creates a member with a one-time temporary password.
func (s *Service) CreateMember(ctx context.Context, p domain.Principal, c inbound.MemberCreateCommand) (inbound.MemberCreated, error) {
	var out inbound.MemberCreated
	if err := domain.Authorize(p, domain.ActionMembersWrite, nil); err != nil {
		return out, err
	}
	if c.Role == "" {
		c.Role = domain.RoleMember
	}
	if !domain.ValidRole(c.Role) {
		return out, domain.Invalid("role", "Vai trò không hợp lệ.")
	}
	if c.Role != domain.RoleMember {
		if err := domain.Authorize(p, domain.ActionRolesWrite, nil); err != nil {
			return out, err
		}
	}
	email, err := domain.NormalizeEmail(c.Email)
	if err != nil {
		return out, err
	}
	if err = domain.ValidateName("name", c.Name); err != nil {
		return out, err
	}
	password, err := s.Random.Base62(20)
	if err != nil {
		return out, err
	}
	hash, err := s.Passwords.Hash(ctx, password)
	if err != nil {
		return out, err
	}
	id, err := s.IDs.NewID()
	if err != nil {
		return out, err
	}
	u := domain.User{ID: id, Email: email, Name: strings.TrimSpace(c.Name), PasswordHash: hash, Role: c.Role, Status: domain.UserActive, MustChangePassword: true, CreatedAt: s.Clock.Now()}
	err = s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if e := s.Users.LockIdentity(ctx); e != nil {
			return e
		}
		return s.Users.CreateUser(ctx, u)
	})
	if err != nil {
		return out, err
	}
	return inbound.MemberCreated{Member: u, TemporaryPassword: password}, nil
}

// UpdateMember serializes membership changes and protects the last active owner.
func (s *Service) UpdateMember(ctx context.Context, p domain.Principal, id uuid.UUID, c domain.MemberChanges) (domain.User, error) {
	var out domain.User
	if err := domain.Authorize(p, domain.ActionMembersWrite, nil); err != nil {
		return out, err
	}
	if c.Name != nil {
		if err := domain.ValidateName("name", *c.Name); err != nil {
			return out, err
		}
		name := strings.TrimSpace(*c.Name)
		c.Name = &name
	}
	if c.Role != nil && !domain.ValidRole(*c.Role) {
		return out, domain.Invalid("role", "Vai trò không hợp lệ.")
	}
	if c.Status != nil && *c.Status != domain.UserActive && *c.Status != domain.UserDisabled {
		return out, domain.Invalid("status", "Trạng thái không hợp lệ.")
	}
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.Users.LockIdentity(ctx); err != nil {
			return err
		}
		u, err := s.Users.GetUserForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if p.Role == domain.RoleAdmin && (u.Role != domain.RoleMember || (c.Role != nil && *c.Role != domain.RoleMember)) {
			return domain.ErrForbidden
		}
		removingOwner := u.Role == domain.RoleOwner && u.Status == domain.UserActive && ((c.Role != nil && *c.Role != domain.RoleOwner) || (c.Status != nil && *c.Status != domain.UserActive))
		if removingOwner {
			n, e := s.Users.CountActiveOwners(ctx)
			if e != nil {
				return e
			}
			if n <= 1 {
				return &domain.Error{Kind: domain.ErrConflict, Detail: "Cần ít nhất một chủ sở hữu đang hoạt động."}
			}
		}
		out, err = s.Users.UpdateMember(ctx, id, c)
		if err != nil {
			return err
		}
		if c.Status != nil && *c.Status == domain.UserDisabled {
			return s.RefreshTokens.RevokeUserRefreshTokens(ctx, id, uuid.Nil, s.Clock.Now())
		}
		return nil
	})
	return out, err
}
