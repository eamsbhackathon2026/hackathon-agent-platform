package domain

import (
	"errors"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestPolicyMatrix(t *testing.T) {
	id, other := uuid.New(), uuid.New()
	actions := []Action{ActionMe, ActionChangePassword, ActionLogout, ActionMembersRead, ActionMembersWrite, ActionRolesWrite, ActionAPIKeysRead, ActionAPIKeysWrite, ActionWebhookDeliveries, ActionResourcesRead, ActionResourcesWrite, ActionRunsWrite, ActionRunsRead, ActionSessionsRead, ActionSpansRead, Action("unknown")}
	for _, role := range []Role{RoleOwner, RoleAdmin, RoleMember, Role("invalid")} {
		for _, own := range []*uuid.UUID{nil, &id, &other} {
			for _, action := range actions {
				p := Principal{Kind: PrincipalUser, UserID: id, Role: role}
				want := false
				if ValidRole(role) {
					switch action {
					case ActionMe, ActionChangePassword, ActionLogout, ActionResourcesRead, ActionRunsWrite:
						want = true
					case ActionMembersRead, ActionMembersWrite, ActionAPIKeysRead, ActionAPIKeysWrite, ActionWebhookDeliveries, ActionResourcesWrite:
						want = role != RoleMember
					case ActionRolesWrite:
						want = role == RoleOwner
					case ActionRunsRead, ActionSessionsRead, ActionSpansRead:
						want = role != RoleMember || (own != nil && *own == id)
					}
				}
				if got := Authorize(p, action, own) == nil; got != want {
					t.Errorf("role=%s action=%s own=%v got=%v want=%v", role, action, own, got, want)
				}
				p.MustChangePassword = true
				want = ValidRole(role) && (action == ActionMe || action == ActionChangePassword || action == ActionLogout)
				if got := Authorize(p, action, own) == nil; got != want {
					t.Errorf("forced password role=%s action=%s", role, action)
				}
			}
		}
	}
	for _, scopes := range [][]string{nil, {"runs:read"}, {"runs:write"}, {"runs:read", "runs:write"}, {"unknown"}} {
		for _, own := range []*uuid.UUID{nil, &id, &other} {
			for _, a := range actions {
				p := Principal{Kind: PrincipalAPIKey, APIKeyID: id, Scopes: scopes}
				want := false
				for _, scope := range scopes {
					want = want || (a == ActionRunsWrite && scope == "runs:write") || (a == ActionRunsRead && scope == "runs:read" && own != nil && *own == id)
				}
				if (Authorize(p, a, own) == nil) != want {
					t.Errorf("key scopes=%v action=%s", scopes, a)
				}
			}
		}
	}
	for _, p := range []Principal{{}, {Kind: "unknown"}, {Kind: PrincipalAPIKey}, {Kind: PrincipalUser, Role: RoleOwner}} {
		if !errors.Is(Authorize(p, ActionMe, nil), ErrForbidden) {
			t.Fatal("invalid principal authorized")
		}
	}
}
func TestValidation(t *testing.T) {
	email, err := NormalizeEmail("  Foo@Example.com ")
	if err != nil || email != "foo@example.com" {
		t.Fatal(email, err)
	}
	for _, v := range []string{"", "foo", "Name <foo@example.com>", "<foo@example.com>", strings.Repeat("x", 250) + "@e.com"} {
		if _, err = NormalizeEmail(v); err == nil {
			t.Errorf("accepted email %q", v)
		}
	}
	for _, v := range []string{"", "  ", strings.Repeat("a", 201), string([]byte{0xff})} {
		if ValidateName("name", v) == nil {
			t.Fatal("accepted name")
		}
	}
	if ValidateName("name", " Tên tổ chức ") != nil {
		t.Fatal("valid name rejected")
	}
	for _, v := range []string{"short", strings.Repeat("a", 1025), string([]byte{0xff})} {
		if ValidatePassword(v) == nil {
			t.Fatal("accepted password")
		}
	}
	if ValidatePassword(strings.Repeat("ệ", 10)) != nil {
		t.Fatal("unicode password rejected")
	}
	e := Invalid("name", "invalid")
	if !errors.Is(e, ErrValidation) || e.Error() != "validation failed" {
		t.Fatal(e)
	}
}
