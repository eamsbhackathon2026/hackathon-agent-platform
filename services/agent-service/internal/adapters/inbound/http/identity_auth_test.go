package http_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

func TestIdentityRegistrationLoginAndCookies(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(map[bool]string{false: "dev", true: "secure"}[secure], func(t *testing.T) {
			f := newIdentityFixture(t, secure)
			config := decodeIdentity[gen.AuthConfig](t, f.request(t, "GET", "/v1/auth/config", "", nil, 200))
			if !config.SignupAllowed {
				t.Fatal("first-owner signup disabled")
			}
			r, owner := f.owner(t)
			c := identityCookie(t, r)
			if c.Secure != secure || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/v1/auth" || c.MaxAge != 30*24*60*60 || !c.Expires.Equal(f.clock.Time.Add(30*24*time.Hour)) {
				t.Fatalf("invalid cookie flags: %+v", c)
			}
			if owner.Me.User.Role != gen.RoleOwner || owner.ExpiresIn != 900 {
				t.Fatal("invalid first-owner session")
			}
			config = decodeIdentity[gen.AuthConfig](t, f.request(t, "GET", "/v1/auth/config", "", nil, 200))
			if config.SignupAllowed {
				t.Fatal("signup remains open")
			}
			f.request(t, "POST", "/v1/auth/register", `{"name":"Other","email":"other@example.com","password":"password12345"}`, nil, 403)
			login := decodeIdentity[gen.TokenResponse](t, f.request(t, "POST", "/v1/auth/login", `{"email":"owner@example.com","password":"password12345"}`, nil, 200))
			me := decodeIdentity[gen.Me](t, f.request(t, "GET", "/v1/me", "", identityBearer(login.AccessToken), 200))
			if me.User.Id != owner.Me.User.Id {
				t.Fatal("wrong current user")
			}
			for _, email := range []string{"owner@example.com", "missing@example.com"} {
				f.request(t, "POST", "/v1/auth/login", identityJSON(t, map[string]string{"email": email, "password": "incorrect-password"}), nil, 401)
			}
			for _, secret := range []string{"password12345", owner.AccessToken, c.Value} {
				if strings.Contains(f.logs.String(), secret) {
					t.Fatal("credential leaked to logs")
				}
			}
		})
	}
}

func TestIdentityRefreshReuseLogoutAndExpiry(t *testing.T) {
	f := newIdentityFixture(t, true)
	r, _ := f.owner(t)
	first := identityCookie(t, r)
	headers := map[string]string{"Cookie": "refresh_token=" + first.Value, "Origin": "http://localhost:5173"}
	next := identityCookie(t, f.request(t, "POST", "/v1/auth/refresh", "", headers, 200))
	if next.Value == first.Value || !next.Expires.Equal(first.Expires) {
		t.Fatal("refresh did not rotate with absolute expiry")
	}
	f.request(t, "POST", "/v1/auth/refresh", "", headers, 401)
	f.request(t, "POST", "/v1/auth/refresh", "", map[string]string{"Cookie": "refresh_token=" + next.Value}, 401)
	login := f.request(t, "POST", "/v1/auth/login", `{"email":"owner@example.com","password":"password12345"}`, nil, 200)
	headers = map[string]string{"Cookie": "refresh_token=" + identityCookie(t, login).Value}
	deleted := identityCookie(t, f.request(t, "POST", "/v1/auth/logout", "", headers, 204))
	if deleted.Value != "" || deleted.MaxAge != -1 || !deleted.Expires.Before(f.clock.Time) || deleted.Path != "/v1/auth" || !deleted.Secure || !deleted.HttpOnly {
		t.Fatal("logout did not expire cookie")
	}
	f.request(t, "POST", "/v1/auth/logout", "", headers, 204)
	f.request(t, "POST", "/v1/auth/logout", "", nil, 204)
	f.request(t, "POST", "/v1/auth/refresh", "", headers, 401)
	f.request(t, "POST", "/v1/auth/refresh", "", nil, 401)
	login = f.request(t, "POST", "/v1/auth/login", `{"email":"owner@example.com","password":"password12345"}`, nil, 200)
	f.clock.Time = f.clock.Time.Add(30 * 24 * time.Hour)
	f.request(t, "POST", "/v1/auth/refresh", "", map[string]string{"Cookie": "refresh_token=" + identityCookie(t, login).Value}, 401)
	f.request(t, "GET", "/v1/me", "", identityBearer(decodeIdentity[gen.TokenResponse](t, login).AccessToken), 401)
}
