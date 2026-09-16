package http

import (
	"context"
	"time"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// GetAuthConfig reports whether the first owner can be created.
func (h *IdentityHandler) GetAuthConfig(ctx context.Context, _ gen.GetAuthConfigRequestObject) (gen.GetAuthConfigResponseObject, error) {
	allowed, err := h.auth.SignupAllowed(ctx)
	return gen.GetAuthConfig200JSONResponse{SignupAllowed: allowed}, err
}

// Register creates the first owner and returns session credentials.
func (h *IdentityHandler) Register(ctx context.Context, r gen.RegisterRequestObject) (gen.RegisterResponseObject, error) {
	if r.Body == nil {
		return nil, domain.ErrValidation
	}
	s, err := h.auth.Register(ctx, inbound.RegisterCommand{Name: r.Body.Name, Email: string(r.Body.Email), Password: stringValue(r.Body.Password)})
	if err != nil {
		return nil, err
	}
	return gen.Register201JSONResponse{Body: h.tokenResponse(s), Headers: gen.Register201ResponseHeaders{SetCookie: h.refreshCookie(s.RefreshToken, s.RefreshExpiresAt)}}, nil
}

// Login starts a session after credential verification.
func (h *IdentityHandler) Login(ctx context.Context, r gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	if r.Body == nil {
		return nil, domain.ErrValidation
	}
	s, err := h.auth.Login(ctx, inbound.LoginCommand{Email: string(r.Body.Email), Password: stringValue(r.Body.Password), IP: requestFrom(ctx).ip})
	if err != nil {
		return nil, err
	}
	return gen.Login200JSONResponse{Body: h.tokenResponse(s), Headers: gen.Login200ResponseHeaders{SetCookie: h.refreshCookie(s.RefreshToken, s.RefreshExpiresAt)}}, nil
}

// Refresh rotates the refresh cookie and issues an access token.
func (h *IdentityHandler) Refresh(ctx context.Context, _ gen.RefreshRequestObject) (gen.RefreshResponseObject, error) {
	s, err := h.auth.Refresh(ctx, requestFrom(ctx).refreshToken)
	if err != nil {
		return nil, err
	}
	return gen.Refresh200JSONResponse{Body: h.tokenResponse(s), Headers: gen.Refresh200ResponseHeaders{SetCookie: h.refreshCookie(s.RefreshToken, s.RefreshExpiresAt)}}, nil
}

// Logout revokes the refresh family and expires its cookie.
func (h *IdentityHandler) Logout(ctx context.Context, _ gen.LogoutRequestObject) (gen.LogoutResponseObject, error) {
	if err := h.auth.Logout(ctx, requestFrom(ctx).refreshToken); err != nil {
		return nil, err
	}
	return gen.Logout204Response{Headers: gen.Logout204ResponseHeaders{SetCookie: h.refreshCookie("", time.Time{})}}, nil
}

// GetMe returns the current account without secrets.
func (h *IdentityHandler) GetMe(ctx context.Context, _ gen.GetMeRequestObject) (gen.GetMeResponseObject, error) {
	me, err := h.auth.GetMe(ctx, requestFrom(ctx).principal)
	if err != nil {
		return nil, err
	}
	return gen.GetMe200JSONResponse{User: userDTO(me.User)}, nil
}

// ChangePassword updates the password and ends other refresh sessions.
func (h *IdentityHandler) ChangePassword(ctx context.Context, r gen.ChangePasswordRequestObject) (gen.ChangePasswordResponseObject, error) {
	if r.Body == nil {
		return nil, domain.ErrValidation
	}
	err := h.auth.ChangePassword(ctx, requestFrom(ctx).principal, inbound.ChangePasswordCommand{CurrentPassword: stringValue(r.Body.CurrentPassword), NewPassword: stringValue(r.Body.NewPassword)})
	return gen.ChangePassword204Response{}, err
}

func (h *IdentityHandler) tokenResponse(s inbound.AuthSession) gen.TokenResponse {
	return gen.TokenResponse{AccessToken: s.AccessToken, TokenType: gen.Bearer, ExpiresIn: h.expiresIn(s.AccessExpiresAt), Me: gen.Me{User: userDTO(s.Me.User)}}
}

func stringValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
