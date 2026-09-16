package greennode

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestManagedBaseURLUsesGreenNodeAPIEndpoint(t *testing.T) {
	const want = "https://maas-llm-aiplatform-hcm.api.vngcloud.vn/v1"
	if BaseURL != want {
		t.Fatalf("BaseURL = %q, want %q", BaseURL, want)
	}
}

func TestListModelsAuthenticatesAndReturnsCuratedCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected request: %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"object":"list","data":[{"id":"unrelated/model"}]}`)
	}))
	defer server.Close()

	client, err := New(context.Background(), domain.ProviderConnection{Kind: domain.ProviderGreenNode, BaseURL: server.URL + "/v1", APIKey: "test-key"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"z-ai/glm-5.2-hackathon", "qwen/qwen3.6-flash"}
	if fmt.Sprint(models) != fmt.Sprint(want) {
		t.Fatalf("models = %v, want %v", models, want)
	}
	models[0] = "mutated"
	again, err := client.ListModels(context.Background())
	if err != nil || again[0] != want[0] {
		t.Fatalf("catalog must be copied: %v %v", again, err)
	}
}
