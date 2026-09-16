package http_test

import (
	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestCatalogConnectionResultsPersistAndSanitizeErrors(t *testing.T) {
	f := newCatalogFixture(t)
	p := f.provider(t)
	path := "/v1/providers/" + p.Id.String()
	owner := identityBearer("owner")
	success := decodeIdentity[gen.ProviderTestResult](t, f.request(t, "POST", path+"/test", "", owner, 200))
	if !success.Ok || !success.Code.IsNull() || success.Message == "" {
		t.Fatal("invalid successful test result")
	}
	saved, _ := f.store.GetProvider(context.Background(), p.Id)
	if saved.Status != domain.ConnectionOK || saved.LastCheckedAt == nil || saved.LastError != nil {
		t.Fatal("successful check was not persisted")
	}
	f.llm.ModelsError = fmt.Errorf("raw-upstream-secret-payload: %w", domain.ErrProviderAuth)
	failure := decodeIdentity[gen.ProviderTestResult](t, f.request(t, "POST", path+"/test", "", owner, 200))
	code, err := failure.Code.Get()
	if failure.Ok || err != nil || string(code) != "provider_auth_failed" || failure.Message == "" {
		t.Fatal("invalid failed test result")
	}
	saved, _ = f.store.GetProvider(context.Background(), p.Id)
	if saved.Status != domain.ConnectionFailing || saved.LastError == nil || saved.LastError.Code != "provider_auth_failed" {
		t.Fatal("failed check was not persisted")
	}
	for _, endpoint := range []string{path, path + "/models"} {
		status := 200
		if strings.HasSuffix(endpoint, "/models") {
			status = 502
		}
		r := f.request(t, "GET", endpoint, "", owner, status)
		if strings.Contains(r.Body.String(), "raw-upstream-secret-payload") {
			t.Fatal("upstream error leaked")
		}
	}
	if strings.Contains(saved.LastError.Message, "raw-upstream-secret-payload") || strings.Contains(f.logs.String(), "raw-upstream-secret-payload") {
		t.Fatal("raw error persisted or logged")
	}
}

func TestCatalogModelsSortDeduplicateAndPaginate(t *testing.T) {
	f := newCatalogFixture(t)
	p := f.provider(t)
	path := "/v1/providers/" + p.Id.String() + "/models"
	owner := identityBearer("owner")
	f.llm.Models = []string{"z", "a", "z", "b", "a", ""}
	first := decodeIdentity[gen.ModelPage](t, f.request(t, "GET", path+"?limit=2", "", owner, 200))
	cursor, err := first.NextCursor.Get()
	if err != nil || cursor == "" || len(first.Items) != 2 || first.Items[0].Id != "a" || first.Items[1].Id != "b" {
		t.Fatal("first model page not sorted/deduplicated")
	}
	last := decodeIdentity[gen.ModelPage](t, f.request(t, "GET", path+"?limit=2&cursor="+url.QueryEscape(cursor), "", owner, 200))
	if len(last.Items) != 1 || last.Items[0].Id != "z" || !last.NextCursor.IsNull() {
		t.Fatal("wrong final model page")
	}
	f.llm.ModelsError = domain.ErrModelsUnsupported
	f.request(t, "GET", path, "", owner, 501)
}

func TestCatalogConnectionGenerationFallbackRequiresConfiguredModel(t *testing.T) {
	f := newCatalogFixture(t)
	p := f.provider(t)
	path := "/v1/providers/" + p.Id.String()
	owner := identityBearer("owner")
	f.llm.ModelsError = domain.ErrModelsUnsupported
	failed := decodeIdentity[gen.ProviderTestResult](t, f.request(t, "POST", path+"/test", "", owner, 200))
	if failed.Ok || len(f.llm.Requests()) != 0 {
		t.Fatal("unsupported model listing reported success without a model")
	}
	f.request(t, "PATCH", path, `{"default_model":"manual-model"}`, owner, 200)
	ok := decodeIdentity[gen.ProviderTestResult](t, f.request(t, "POST", path+"/test", "", owner, 200))
	calls := f.llm.Requests()
	if !ok.Ok || len(calls) != 1 || calls[0].Model != "manual-model" || calls[0].MaxOutputTokens == nil || *calls[0].MaxOutputTokens != 1 {
		t.Fatal("fallback must request exactly one output token")
	}
}

func TestCatalogFactoryErrorsNeverExposeUpstreamDetails(t *testing.T) {
	f := newCatalogFixture(t)
	p := f.provider(t)
	f.factory.Err = &domain.Error{Kind: domain.ErrProviderAuth, Detail: "raw-secret-from-client-construction"}
	path := "/v1/providers/" + p.Id.String()
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"POST", path + "/test", 200},
		{"GET", path + "/models", 502},
	} {
		r := f.request(t, tc.method, tc.path, "", identityBearer("owner"), tc.status)
		if strings.Contains(r.Body.String(), "raw-secret-from-client-construction") {
			t.Fatal("client construction error leaked raw provider details")
		}
	}
}
