package http_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	httpadapter "agent-platform/services/agent-service/internal/adapters/inbound/http"
	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/adapters/outbound/security"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/services/catalog"
	"agent-platform/services/agent-service/internal/platform/netguard"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"
)

type catalogResolver struct{}

func (catalogResolver) FromAccessToken(_ context.Context, token string) (domain.Principal, error) {
	return domain.Principal{Kind: domain.PrincipalUser, UserID: uuid.MustParse("00000000-0000-4000-8000-000000000001"), Role: domain.Role(token)}, nil
}
func (catalogResolver) FromAPIKey(context.Context, string) (domain.Principal, error) {
	return domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: uuid.MustParse("00000000-0000-4000-8000-000000000002"), Scopes: []string{"runs:read", "runs:write"}}, nil
}

type catalogFixture struct {
	identityFixture
	store   *fakes.CatalogStore
	llm     *fakes.ScriptedLLM
	factory *fakes.FakeFactory
	cipher  *security.AESGCMSecretCipher
}

func newCatalogFixture(t *testing.T) catalogFixture {
	t.Helper()
	db := fakes.NewCatalogStore()
	clock := &fakes.Clock{Time: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)}
	cipher, err := security.NewAESGCMSecretCipher(bytes.Repeat([]byte{42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	guard, err := netguard.New(netguard.Config{Development: true})
	if err != nil {
		t.Fatal(err)
	}
	llm := &fakes.ScriptedLLM{Models: []string{"model-a"}}
	factory := &fakes.FakeFactory{Client: llm}
	service, err := catalog.NewService(catalog.Dependencies{Providers: db, Agents: db, Tx: db, Cipher: cipher, Factory: factory, URLs: guard, Clock: clock, IDs: fakes.IDs{}})
	if err != nil {
		t.Fatal(err)
	}
	logs := &bytes.Buffer{}
	handler, err := httpadapter.NewRouter(&httpadapter.Handler{SystemHandler: httpadapter.NewSystemHandler(nil), CatalogHandler: httpadapter.NewCatalogHandler(service, service)}, httpadapter.RouterOptions{Resolver: catalogResolver{}, Logger: slog.New(slog.NewJSONHandler(logs, nil))})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := gen.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	spec.Servers = nil
	contract, err := legacy.NewRouter(spec)
	if err != nil {
		t.Fatal(err)
	}
	return catalogFixture{identityFixture{handler, contract, clock, logs}, db, llm, factory, cipher}
}
func (f catalogFixture) provider(t *testing.T) gen.Provider {
	t.Helper()
	return decodeIdentity[gen.Provider](t, f.request(t, "POST", "/v1/providers", `{"name":"Local","kind":"openai_compatible","base_url":"http://127.0.0.1:8081/v1","api_key":"test-secret-1234"}`, identityBearer("owner"), 201))
}
func (f catalogFixture) agent(t *testing.T, id uuid.UUID) gen.Agent {
	t.Helper()
	return decodeIdentity[gen.Agent](t, f.request(t, "POST", "/v1/agents", identityJSON(t, map[string]any{"name": "Helper", "provider_id": id, "model": "model-a", "system_prompt": "Help"}), identityBearer("owner"), 201))
}
