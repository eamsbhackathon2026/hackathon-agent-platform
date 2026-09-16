package http_test

import (
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

func TestIdentityMemberPasswordAndImmediatePermissions(t *testing.T) {
	f := newIdentityFixture(t, false)
	_, owner := f.owner(t)
	ownerHeaders := identityBearer(owner.AccessToken)
	created := decodeIdentity[gen.MemberCreated](t, f.request(t, "POST", "/v1/members", `{"name":"Member","email":"member@example.com","role":"member"}`, ownerHeaders, 201))
	if created.TemporaryPassword == "" || !created.Member.MustChangePassword {
		t.Fatal("missing temporary credential")
	}
	login := decodeIdentity[gen.TokenResponse](t, f.request(t, "POST", "/v1/auth/login", identityJSON(t, map[string]string{"email": "member@example.com", "password": created.TemporaryPassword}), nil, 200))
	memberHeaders := identityBearer(login.AccessToken)
	me := decodeIdentity[gen.Me](t, f.request(t, "GET", "/v1/me", "", memberHeaders, 200))
	if !me.User.MustChangePassword {
		t.Fatal("temporary password flag not exposed")
	}
	f.request(t, "POST", "/v1/api-keys", `{"name":"Key","scopes":["runs:read"]}`, memberHeaders, 403)
	f.request(t, "POST", "/v1/me/password", identityJSON(t, map[string]string{"current_password": created.TemporaryPassword, "new_password": "new-password12345"}), memberHeaders, 204)
	me = decodeIdentity[gen.Me](t, f.request(t, "GET", "/v1/me", "", memberHeaders, 200))
	if me.User.MustChangePassword {
		t.Fatal("password flag remains set")
	}
	f.request(t, "POST", "/v1/api-keys", `{"name":"Key","scopes":["runs:read"]}`, memberHeaders, 403)
	memberPath := "/v1/members/" + created.Member.Id.String()
	f.request(t, "PATCH", memberPath, `{"role":"admin"}`, ownerHeaders, 200)
	f.request(t, "GET", "/v1/api-keys", "", memberHeaders, 200)
	f.request(t, "PATCH", memberPath, `{"role":"member"}`, ownerHeaders, 200)
	f.request(t, "GET", "/v1/api-keys", "", memberHeaders, 403)
	f.request(t, "PATCH", memberPath, `{"status":"disabled"}`, ownerHeaders, 200)
	f.request(t, "GET", "/v1/me", "", memberHeaders, 401)
	f.request(t, "POST", "/v1/auth/login", `{"email":"member@example.com","password":"new-password12345"}`, nil, 401)
}

func TestIdentityAPIKeysNeverListSecrets(t *testing.T) {
	f := newIdentityFixture(t, true)
	_, owner := f.owner(t)
	headers := identityBearer(owner.AccessToken)
	created := decodeIdentity[gen.ApiKeyCreated](t, f.request(t, "POST", "/v1/api-keys", `{"name":"Automation","scopes":["runs:write","runs:read"]}`, headers, 201))
	if !strings.HasPrefix(created.Key, "apk_") || !strings.HasPrefix(created.WebhookSecret, "whsec_") {
		t.Fatal("missing one-time secrets")
	}
	list := f.request(t, "GET", "/v1/api-keys", "", headers, 200)
	page := decodeIdentity[gen.ApiKeyPage](t, list)
	if len(page.Items) != 1 || page.Items[0].Id != created.ApiKey.Id {
		t.Fatal("created key missing from list")
	}
	for _, secret := range []string{created.Key, created.WebhookSecret, "key_hash", "webhook_secret_ciphertext"} {
		if strings.Contains(list.Body.String(), secret) {
			t.Fatal("list leaks credentials")
		}
	}
	keyHeaders := map[string]string{"X-API-Key": created.Key}
	f.request(t, "GET", "/v1/me", "", keyHeaders, 403)
	f.request(t, "GET", "/v1/me", "", map[string]string{"Authorization": "Bearer " + owner.AccessToken, "X-API-Key": created.Key}, 400)
	f.request(t, "DELETE", "/v1/api-keys/"+created.ApiKey.Id.String(), "", headers, 204)
	f.request(t, "GET", "/v1/me", "", keyHeaders, 401)
	for _, secret := range []string{created.Key, created.WebhookSecret, owner.AccessToken} {
		if strings.Contains(f.logs.String(), secret) {
			t.Fatal("log leaks credentials")
		}
	}
}
