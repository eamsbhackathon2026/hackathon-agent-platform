package http

import (
	"net/http"
	"time"
)

func (h *IdentityHandler) refreshCookie(value string, expires time.Time) *string {
	maxAge := h.expiresIn(expires)
	if value == "" {
		maxAge = -1
		expires = time.Unix(1, 0).UTC()
	}
	// #nosec G124 -- Secure is disabled only by explicit APP_ENV=dev wiring; cookie flags are tested.
	cookie := (&http.Cookie{Name: "refresh_token", Value: value, Path: "/v1/auth", HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: maxAge}).String()
	return &cookie
}
