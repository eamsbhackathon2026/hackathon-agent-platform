package http

import (
	"time"

	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// IdentityHandler translates the API contract to identity use cases.
type IdentityHandler struct {
	auth          inbound.AuthUseCase
	members       inbound.MemberUseCase
	keys          inbound.APIKeyUseCase
	clock         outbound.Clock
	secureCookies bool
}

// NewIdentityHandler shares the same clock as token issuance.
func NewIdentityHandler(auth inbound.AuthUseCase, members inbound.MemberUseCase, keys inbound.APIKeyUseCase, clock outbound.Clock, secureCookies bool) *IdentityHandler {
	return &IdentityHandler{auth: auth, members: members, keys: keys, clock: clock, secureCookies: secureCookies}
}

func (h *IdentityHandler) expiresIn(expires time.Time) int {
	seconds := int(expires.Sub(h.clock.Now()).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}
