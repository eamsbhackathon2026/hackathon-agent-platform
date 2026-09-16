package http_test

import "testing"

func TestCatalogAuthorization(t *testing.T) {
	f := newCatalogFixture(t)
	p := f.provider(t)
	a := f.agent(t, p.Id)
	for _, path := range []string{"/v1/providers", "/v1/providers/" + p.Id.String(), "/v1/agents", "/v1/agents/" + a.Id.String()} {
		f.request(t, "GET", path, "", identityBearer("member"), 200)
		f.request(t, "GET", path, "", map[string]string{"X-API-Key": "fake"}, 403)
		f.request(t, "GET", path, "", nil, 401)
	}
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/v1/providers", `{"name":"Other","kind":"gemini","api_key":"test-key"}`},
		{"POST", "/v1/agents", `{"name":"Other","provider_id":"` + p.Id.String() + `","model":"m","system_prompt":"Help"}`},
		{"PATCH", "/v1/providers/" + p.Id.String(), `{"name":"Other"}`},
		{"PATCH", "/v1/agents/" + a.Id.String(), `{"name":"Other"}`},
		{"DELETE", "/v1/providers/" + p.Id.String(), ""},
		{"DELETE", "/v1/agents/" + a.Id.String(), ""},
		{"POST", "/v1/providers/" + p.Id.String() + "/test", ""},
		{"GET", "/v1/providers/" + p.Id.String() + "/models", ""},
	} {
		for _, headers := range []map[string]string{identityBearer("member"), {"X-API-Key": "fake"}} {
			f.request(t, tc.method, tc.path, tc.body, headers, 403)
		}
	}
}

func TestCatalogRequestValidation(t *testing.T) {
	f := newCatalogFixture(t)
	p := f.provider(t)
	a := f.agent(t, p.Id)
	for _, tc := range []struct{ name, method, path, body string }{
		{"malformed", "POST", "/v1/providers", `{"name":`},
		{"missing name", "POST", "/v1/providers", `{"kind":"gemini"}`},
		{"unknown field", "POST", "/v1/providers", `{"name":"Other","kind":"gemini","secret":"x"}`},
		{"missing base URL", "POST", "/v1/providers", `{"name":"Other","kind":"openai_compatible","api_key":"test-key"}`},
		{"invalid UUID", "GET", "/v1/providers/invalid", ""},
		{"zero limit", "GET", "/v1/providers?limit=0", ""},
		{"large limit", "GET", "/v1/agents?limit=101", ""},
		{"bad cursor", "GET", "/v1/agents?cursor=invalid", ""},
		{"bad model cursor", "GET", "/v1/providers/" + p.Id.String() + "/models?cursor=%%%", ""},
		{"invalid session scope", "GET", "/v1/sessions?scope=workspace", ""},
		{"invalid session sort", "GET", "/v1/sessions?sort=recent", ""},
		{"null name", "PATCH", "/v1/providers/" + p.Id.String(), `{"name":null}`},
		{"empty patch", "PATCH", "/v1/agents/" + a.Id.String(), `{}`},
		{"iterations", "PATCH", "/v1/agents/" + a.Id.String(), `{"max_iterations":26}`},
		{"timeout", "PATCH", "/v1/agents/" + a.Id.String(), `{"timeout_seconds":9}`},
		{"temperature", "PATCH", "/v1/agents/" + a.Id.String(), `{"temperature":3}`},
		{"zero tokens", "PATCH", "/v1/agents/" + a.Id.String(), `{"max_output_tokens":0}`},
	} {
		t.Run(tc.name, func(t *testing.T) { f.request(t, tc.method, tc.path, tc.body, identityBearer("owner"), 400) })
	}
}
