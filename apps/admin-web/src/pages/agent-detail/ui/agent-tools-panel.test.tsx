import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import { http } from "msw";
import { describe, expect, it } from "vitest";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { AgentToolsPanel } from "./agent-tools-panel";

const tool = (id: string, slug: string, connectionId: string | null) => ({ id, kind: "http" as const, slug, display_name: `Tool ${slug}`, description: "", method: "GET" as const, url_template: `/${slug}`, connection_id: connectionId, params: [], public_headers: {}, secret_header_names: [], timeout_seconds: 15, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" });
const connection = (id: string, name: string) => ({ id, slug: id, display_name: name, base_url: "http://example.test", public_headers: {}, secret_header_names: [], created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" });

function renderPanel(props: { readOnly?: boolean } = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}><AgentToolsPanel agentId="agent-1" {...props} /></QueryClientProvider>);
}

function mockEmptyServersAndBindings(toolIds: string[] = []) {
  server.use(
    http.get(apiUrl("/v1/mcp-servers"), () => jsonResponse({ items: [], next_cursor: null })),
    http.get("*/v1/agents/:agentId/tools", () => jsonResponse({ tool_ids: toolIds, mcp_server_ids: [] })),
  );
}

describe("AgentToolsPanel", () => {
  it("shows current bindings without write controls in read-only mode", async () => {
    const orders = tool("tool-1", "orders", null);
    server.use(http.get(apiUrl("/v1/tools"), () => jsonResponse({ items: [orders], next_cursor: null })), http.get(apiUrl("/v1/api-connections"), () => jsonResponse({ items: [], next_cursor: null })));
    mockEmptyServersAndBindings([orders.id]);
    renderPanel({ readOnly: true });
    const checkbox = await screen.findByRole("checkbox"); expect(checkbox).toBeChecked(); expect(checkbox).toBeDisabled(); expect(screen.queryByRole("button", { name: "Save tool list" })).not.toBeInTheDocument();
  });

  it("lists every tool even when the server splits them across pages", async () => {
    // The server's default page is 20; a picker that stops there hides most tools.
    const all = Array.from({ length: 43 }, (_, index) => tool(`tool-${index}`, `tool_${String(index).padStart(2, "0")}`, "conn-a"));
    server.use(
      http.get(apiUrl("/v1/tools"), ({ request }) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        return cursor === "page-2" ? jsonResponse({ items: all.slice(20), next_cursor: null }) : jsonResponse({ items: all.slice(0, 20), next_cursor: "page-2" });
      }),
      http.get(apiUrl("/v1/api-connections"), () => jsonResponse({ items: [connection("conn-a", "Giao dịch")], next_cursor: null })),
    );
    mockEmptyServersAndBindings();
    renderPanel();
    expect(await screen.findByRole("button", { name: "Save tool list" })).toBeInTheDocument();
    expect(screen.getAllByRole("checkbox")).toHaveLength(43);
    expect(screen.getByText("0 of 43 selected.", { exact: false })).toBeInTheDocument();
  });

  it("groups tools by the connection they call and shows the short name", async () => {
    server.use(
      http.get(apiUrl("/v1/tools"), () => jsonResponse({ items: [tool("t1", "precheck_transfer", "conn-risk"), tool("t2", "get_customer", "conn-customer"), tool("t3", "ping", null)], next_cursor: null })),
      http.get(apiUrl("/v1/api-connections"), () => jsonResponse({ items: [connection("conn-risk", "Chấm điểm rủi ro"), connection("conn-customer", "Hồ sơ khách hàng")], next_cursor: null })),
    );
    mockEmptyServersAndBindings();
    renderPanel();
    const risk = await screen.findByRole("region", { name: "Chấm điểm rủi ro" });
    expect(within(risk).getByText("precheck_transfer")).toBeInTheDocument();
    expect(within(risk).getByText("GET /precheck_transfer", { exact: false })).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "Hồ sơ khách hàng" })).getByText("get_customer")).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "Direct URL" })).getByText("ping")).toBeInTheDocument();
  });
});
