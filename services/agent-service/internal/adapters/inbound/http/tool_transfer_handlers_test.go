package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	httpadapter "agent-platform/services/agent-service/internal/adapters/inbound/http"
	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/adapters/outbound/security"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/core/services/tooling"
	"agent-platform/services/agent-service/internal/platform/netguard"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
	"github.com/getkin/kin-openapi/routers/legacy"
)

type unusedToolInvoker struct{}

func (unusedToolInvoker) Invoke(context.Context, domain.HTTPTool, map[string]string, json.RawMessage) (domain.HTTPToolInvocation, error) {
	return domain.HTTPToolInvocation{}, errors.New("not used")
}

func (unusedToolInvoker) Connect(context.Context, domain.MCPServer, map[string]string) (outbound.MCPSession, error) {
	return nil, errors.New("not used")
}

func newToolTransferFixture(t *testing.T) identityFixture {
	t.Helper()
	db := fakes.NewCatalogStore()
	cipher, err := security.NewAESGCMSecretCipher(bytes.Repeat([]byte{42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	guard, err := netguard.New(netguard.Config{Development: true})
	if err != nil {
		t.Fatal(err)
	}
	clock := &fakes.Clock{Time: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)}
	service, err := tooling.NewService(tooling.Dependencies{Tools: db, Connections: db, MCPServers: db, Bindings: db, Agents: db, Tx: db, Cipher: cipher, HTTP: unusedToolInvoker{}, MCP: unusedToolInvoker{}, URLs: guard, Clock: clock, IDs: fakes.IDs{}})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := httpadapter.NewRouter(&httpadapter.Handler{SystemHandler: httpadapter.NewSystemHandler(nil), ToolingHandler: httpadapter.NewToolingHandler(service, service, service, service, service)}, httpadapter.RouterOptions{Resolver: catalogResolver{}, Logger: slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))})
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
	return identityFixture{handler: handler, contract: contract, clock: clock}
}

func TestToolTransferEndpoints(t *testing.T) {
	f := newToolTransferFixture(t)
	f.request(t, "POST", "/v1/api-connections", `{"slug":"orders","display_name":"Orders","base_url":"http://127.0.0.1:9000","secret_headers":{"Authorization":"Bearer hidden"}}`, identityBearer("admin"), 201)
	connections := decodeIdentity[gen.ApiConnectionPage](t, f.request(t, "GET", "/v1/api-connections", "", identityBearer("member"), 200))
	f.request(t, "POST", "/v1/tools", identityJSON(t, map[string]any{"slug": "lookup", "display_name": "Lookup", "method": "GET", "url_template": "/items/{id}", "connection_id": connections.Items[0].Id, "params": []map[string]any{{"name": "id", "type": "string", "required": true, "in": "path", "description": ""}}}), identityBearer("admin"), 201)

	exported := f.request(t, "GET", "/v1/tools/export", "", identityBearer("member"), 200)
	if strings.Contains(exported.Body.String(), "Bearer hidden") {
		t.Fatalf("export leaked a secret: %s", exported.Body.String())
	}
	bundle := decodeIdentity[gen.ToolBundle](t, exported)
	if bundle.Format != "agent-platform.tools" || len(bundle.Tools) != 1 || len(bundle.Connections) != 1 || bundle.Tools[0].ConnectionSlug.MustGet() != "orders" {
		t.Fatalf("bundle=%s", exported.Body.String())
	}

	f.request(t, "POST", "/v1/tools/import/preview", exported.Body.String(), identityBearer("member"), 403)
	preview := decodeIdentity[gen.ToolImportPreview](t, f.request(t, "POST", "/v1/tools/import/preview", exported.Body.String(), identityBearer("admin"), 200))
	if len(preview.Items) != 2 || preview.Items[0].Status != gen.ToolImportItemStatusConflict || preview.Items[1].Status != gen.ToolImportItemStatusConflict {
		t.Fatalf("preview=%+v", preview)
	}

	request := func(action string) string {
		return identityJSON(t, map[string]any{"bundle": bundle, "decisions": []map[string]string{{"kind": "connection", "slug": "orders", "action": action}, {"kind": "tool", "slug": "lookup", "action": action}}})
	}
	missing := decodeIdentity[gen.Problem](t, f.request(t, "POST", "/v1/tools/import", identityJSON(t, map[string]any{"bundle": bundle, "decisions": []any{}}), identityBearer("admin"), 400))
	if len(missing.Fields) != 2 {
		t.Fatalf("problem=%+v", missing)
	}
	result := decodeIdentity[gen.ToolImportResult](t, f.request(t, "POST", "/v1/tools/import", request("overwrite"), identityBearer("admin"), 200))
	if result.Overwritten != 2 || len(result.NeedsSecrets) != 0 {
		t.Fatalf("result=%+v", result)
	}
	f.request(t, "POST", "/v1/tools/import", `{"bundle":{"format":"other","version":1,"connections":[],"tools":[]},"decisions":[]}`, identityBearer("admin"), 400)
}
