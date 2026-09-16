package identity_test

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"fmt"
	"github.com/google/uuid"
	"sync"
	"testing"
)

func TestMembersPermissionsPasswordAndPagination(t *testing.T) {
	f := setup(t)
	owner := f.register(t, "owner@example.com")
	p := f.principal(t, owner)
	member, err := f.s.CreateMember(ctx, p, inbound.MemberCreateCommand{Email: "member@example.com", Name: "Member"})
	if err != nil {
		t.Fatal(err)
	}
	if !member.Member.MustChangePassword || len(member.TemporaryPassword) != 20 {
		t.Fatal("temporary password")
	}
	login, err := f.s.Login(ctx, inbound.LoginCommand{Email: member.Member.Email, Password: member.TemporaryPassword})
	if err != nil {
		t.Fatal(err)
	}
	mp := f.principal(t, login)
	_, err = f.s.ListMembers(ctx, mp, inbound.PageRequest{})
	requireError(t, err, domain.ErrForbidden)
	_, err = f.s.GetMe(ctx, mp)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.ChangePassword(ctx, mp, inbound.ChangePasswordCommand{CurrentPassword: member.TemporaryPassword, NewPassword: "member-password"}); err != nil {
		t.Fatal(err)
	}
	mp = f.principal(t, login)
	if mp.MustChangePassword {
		t.Fatal("flag retained")
	}
	_, err = f.s.CreateMember(ctx, mp, inbound.MemberCreateCommand{})
	requireError(t, err, domain.ErrForbidden)
	admin, err := f.s.CreateMember(ctx, p, inbound.MemberCreateCommand{Email: "admin@example.com", Name: "Admin", Role: domain.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	ap := domain.Principal{Kind: domain.PrincipalUser, UserID: admin.Member.ID, Role: domain.RoleAdmin}
	_, err = f.s.CreateMember(ctx, ap, inbound.MemberCreateCommand{Email: "admin2@example.com", Name: "Admin", Role: domain.RoleOwner})
	requireError(t, err, domain.ErrForbidden)
	name := "Changed"
	_, err = f.s.UpdateMember(ctx, ap, p.UserID, domain.MemberChanges{Name: &name})
	requireError(t, err, domain.ErrForbidden)
	_, err = f.s.UpdateMember(ctx, ap, ap.UserID, domain.MemberChanges{Name: &name})
	requireError(t, err, domain.ErrForbidden)
	updated, err := f.s.UpdateMember(ctx, ap, mp.UserID, domain.MemberChanges{Name: &name})
	if err != nil || updated.Name != name {
		t.Fatal(err)
	}
	page, err := f.s.ListMembers(ctx, p, inbound.PageRequest{Limit: 2})
	if err != nil || len(page.Items) != 2 || page.NextCursor == nil {
		t.Fatal(page, err)
	}
	next, err := f.s.ListMembers(ctx, p, inbound.PageRequest{Limit: 2, Cursor: *page.NextCursor})
	if err != nil || len(next.Items) != 1 || next.NextCursor != nil {
		t.Fatal(next, err)
	}
	all, err := f.s.ListMembers(ctx, p, inbound.PageRequest{})
	if err != nil || len(all.Items) != 3 {
		t.Fatal(err)
	}
	disabled := domain.UserDisabled
	_, err = f.s.UpdateMember(ctx, p, mp.UserID, domain.MemberChanges{Status: &disabled})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.s.FromAccessToken(ctx, login.AccessToken)
	requireError(t, err, domain.ErrUnauthenticated)
	_, err = f.s.Refresh(ctx, login.RefreshToken)
	requireError(t, err, domain.ErrUnauthenticated)
	_, err = f.s.Login(ctx, inbound.LoginCommand{Email: member.Member.Email, Password: "member-password"})
	requireError(t, err, domain.ErrUnauthenticated)
}
func TestOwnerGuardAndConcurrentMembership(t *testing.T) {
	f := setup(t)
	first := f.register(t, "owner@example.com")
	p := f.principal(t, first)
	role := domain.RoleMember
	_, err := f.s.UpdateMember(ctx, p, p.UserID, domain.MemberChanges{Role: &role})
	requireError(t, err, domain.ErrConflict)
	second, err := f.s.CreateMember(ctx, p, inbound.MemberCreateCommand{Email: "second@example.com", Name: "Second", Role: domain.RoleOwner})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range []uuid.UUID{p.UserID, second.Member.ID} {
		wg.Go(func() { _, e := f.s.UpdateMember(ctx, p, id, domain.MemberChanges{Role: &role}); results <- e })
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		} else {
			requireError(t, e, domain.ErrConflict)
		}
	}
	if success != 1 {
		t.Fatal(success)
	}
	count, err := f.db.CountActiveOwners(ctx)
	if err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
func TestConcurrentFirstOwnerOnly(t *testing.T) {
	f := setup(t)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			_, e := f.s.Register(ctx, inbound.RegisterCommand{Name: "Owner", Email: fmt.Sprintf("owner%d@example.com", i), Password: "password12345"})
			errs <- e
		})
	}
	wg.Wait()
	close(errs)
	success := 0
	for e := range errs {
		if e == nil {
			success++
		} else {
			requireError(t, e, domain.ErrForbidden)
		}
	}
	if success != 1 {
		t.Fatal(success)
	}
	count, err := f.db.CountUsers(ctx)
	if err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
