import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { clearAuthSession, setAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { ToolsPage } from "./tools-page";

const user = { id: "user-1", email: "user@example.test", name: "User", role: "admin" as const, status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" };
const tool = { id: "tool-1", connection_id: null, kind: "http" as const, slug: "orders", display_name: "Orders", description: "", method: "POST" as const, url_template: "https://example.test/orders", params: [], public_headers: {}, secret_header_names: [], timeout_seconds: 15, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };
const connection = { id: "connection-1", slug: "commerce", display_name: "Commerce API", base_url: "https://api.example.test/v1", public_headers: {}, secret_header_names: ["Authorization"], created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };
const failingServer = { id: "server-1", slug: "crm", display_name: "Internal CRM", url: "https://crm.example.test/mcp", allowed_tools: null, secret_header_names: ["Authorization"], tools: [], status: "failing" as const, last_error: { code: "tool_failed" as const, message: "Unable to connect to the server" }, last_synced_at: null, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };

function handlers(method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE" = tool.method) { server.use(http.get(apiUrl("/v1/api-connections"), () => jsonResponse({ items: [connection], next_cursor: null })), http.get(apiUrl("/v1/tools"), () => jsonResponse({ items: [{ ...tool, method }], next_cursor: null })), http.get(apiUrl("/v1/mcp-servers"), () => jsonResponse({ items: [], next_cursor: null })), http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null }))); }
function renderPage(path = "/tools") { const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><ToolsPage /></MemoryRouter></QueryClientProvider>); }

afterEach(() => { clearAuthSession(); vi.restoreAllMocks(); });
describe("ToolsPage", () => {
  it("does not send a side-effecting test before explicit confirmation", async () => {
    setAuthSession("token", user); handlers(); const request = vi.fn(); server.use(http.post("*/v1/tools/:toolId/test", () => { request(); return jsonResponse({ ok: true, status_code: 200, body: "ok", truncated: false, duration_ms: 4, error: null }); })); vi.spyOn(window, "confirm").mockReturnValue(false); renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Test" })); fireEvent.click(screen.getByRole("button", { name: "Send test" }));
    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining("may change data")); expect(request).not.toHaveBeenCalled();
  });
  it("shows the complete test response", async () => {
    setAuthSession("token", user); handlers("GET"); server.use(http.post("*/v1/tools/:toolId/test", () => jsonResponse({ ok: false, status_code: 502, body: "upstream body", truncated: true, duration_ms: 8, error: { code: "tool_failed", message: "Server rejected the request" } }))); renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Test" })); fireEvent.click(screen.getByRole("button", { name: "Send test" }));
    expect(await screen.findByText("HTTP status: 502")).toBeInTheDocument(); expect(screen.getByText("upstream body")).toBeInTheDocument(); expect(screen.getByText("The response was truncated.")).toBeInTheDocument(); expect(screen.getByText("Error: Server rejected the request")).toBeInTheDocument();
  });
  it("keeps configuration read-only for a member", async () => {
    setAuthSession("token", { ...user, role: "member" }); handlers("GET"); renderPage();
    expect(await screen.findByText("Only administrators can edit tools. Contact an administrator for help.")).toBeInTheDocument(); expect(screen.queryByRole("button", { name: "Test" })).not.toBeInTheDocument(); expect(screen.queryByRole("link", { name: "Create tool" })).not.toBeInTheDocument(); fireEvent.mouseDown(screen.getByRole("tab", { name: "API connections" }), { button: 0 }); expect(screen.queryByRole("button", { name: "Connect API" })).not.toBeInTheDocument();
    expect(await screen.findByText("Commerce API")).toBeInTheDocument();
    expect(screen.getByText("Public headers:").parentElement).toHaveTextContent("Public headers: None");
    expect(screen.getByText("Secret headers:").parentElement).toHaveTextContent("Secret headers: Authorization");
  });
  it("labels each HTTP tool with its system above the name and its description below", async () => {
    setAuthSession("token", user); handlers("GET"); server.use(http.get(apiUrl("/v1/tools"), () => jsonResponse({ items: [{ ...tool, id: "tool-2", slug: "invoices", display_name: "Invoices", description: "List unpaid invoices for a customer", connection_id: connection.id, url_template: "/invoices" }, tool], next_cursor: null }))); renderPage();
    const invoices = await screen.findByText("Invoices"); const invoicesHeader = invoices.parentElement!;
    expect(invoicesHeader).toHaveTextContent("Commerce API"); expect(invoicesHeader.firstElementChild).toHaveTextContent("Commerce API"); expect(invoicesHeader).toHaveTextContent("List unpaid invoices for a customer"); expect(invoicesHeader).not.toHaveTextContent("/invoices");
    const ordersHeader = screen.getByText("Orders").parentElement!;
    expect(ordersHeader.firstElementChild).toHaveTextContent("Direct URL"); expect(ordersHeader).not.toHaveTextContent("https://example.test/orders");
  });
  it("sends admins to the dedicated pages to create and edit tools", async () => {
    setAuthSession("token", user); handlers("GET"); renderPage();
    expect(await screen.findByRole("link", { name: "Create tool" })).toHaveAttribute("href", "/tools/new");
    expect(await screen.findByRole("link", { name: "Edit" })).toHaveAttribute("href", `/tools/${tool.id}/edit`);
  });
  it("creates, updates, and deletes an API connection", async () => {
    setAuthSession("token", user); handlers("GET"); let created: unknown; let updated: unknown; let deleted = false;
    server.use(
      http.post(apiUrl("/v1/api-connections"), async ({ request }) => { created = await request.json(); return jsonResponse({ ...connection, display_name: "Payments API", slug: "payments", base_url: "https://payments.example.test/v1", secret_header_names: [] }, 201); }),
      http.patch("*/v1/api-connections/:apiConnectionId", async ({ request }) => { updated = await request.json(); return jsonResponse({ ...connection, display_name: "Commerce API updated" }); }),
      http.delete("*/v1/api-connections/:apiConnectionId", () => { deleted = true; return new Response(null, { status: 204 }); }),
    );
    vi.spyOn(window, "confirm").mockReturnValue(true); renderPage(); fireEvent.mouseDown(await screen.findByRole("tab", { name: "API connections" }), { button: 0 });
    fireEvent.click(screen.getByRole("button", { name: "Connect API" }));
    fireEvent.change(screen.getByLabelText("Display name"), { target: { value: "Payments API" } });
    fireEvent.change(screen.getByLabelText("Short name"), { target: { value: "payments" } });
    fireEvent.change(screen.getByLabelText("Base URL"), { target: { value: "https://payments.example.test/v1" } });
    fireEvent.click(screen.getByRole("button", { name: "Save connection" }));
    await waitFor(() => expect(created).toMatchObject({ display_name: "Payments API", slug: "payments", base_url: "https://payments.example.test/v1", public_headers: {} }));
    fireEvent.click(await screen.findByRole("button", { name: "Edit connection" }));
    fireEvent.change(screen.getByLabelText("Display name"), { target: { value: "Commerce API updated" } });
    fireEvent.click(screen.getByRole("button", { name: "Save connection" }));
    await waitFor(() => expect(updated).toMatchObject({ display_name: "Commerce API updated" }));
    fireEvent.click(await screen.findByRole("button", { name: "Delete" }));
    await waitFor(() => expect(deleted).toBe(true));
  });
  it("edits a connection without secret headers as an empty list", async () => {
    setAuthSession("token", user); handlers("GET"); server.use(http.get(apiUrl("/v1/api-connections"), () => jsonResponse({ items: [{ ...connection, secret_header_names: [] }], next_cursor: null }))); renderPage();
    fireEvent.mouseDown(await screen.findByRole("tab", { name: "API connections" }), { button: 0 });
    fireEvent.click(await screen.findByRole("button", { name: "Edit connection" }));
    expect(screen.getByText("Saved: None")).toBeInTheDocument();
  });
  it("shows an MCP failure reason and recovery actions to an admin", async () => {
    setAuthSession("token", user); server.use(http.get(apiUrl("/v1/api-connections"), () => jsonResponse({ items: [], next_cursor: null })), http.get(apiUrl("/v1/tools"), () => jsonResponse({ items: [], next_cursor: null })), http.get(apiUrl("/v1/mcp-servers"), () => jsonResponse({ items: [failingServer], next_cursor: null })), http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })));
    renderPage(); fireEvent.mouseDown(await screen.findByRole("tab", { name: "Tool servers" }), { button: 0 });
    expect(await screen.findByRole("alert")).toHaveTextContent("Unable to connect to the server");
    expect(screen.getByRole("button", { name: "Try refreshing" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Edit connection" })).toBeInTheDocument();
  });
  it("shows an MCP failure reason without mutation actions to a member", async () => {
    setAuthSession("token", { ...user, role: "member" }); server.use(http.get(apiUrl("/v1/api-connections"), () => jsonResponse({ items: [], next_cursor: null })), http.get(apiUrl("/v1/tools"), () => jsonResponse({ items: [], next_cursor: null })), http.get(apiUrl("/v1/mcp-servers"), () => jsonResponse({ items: [failingServer], next_cursor: null })), http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })));
    renderPage(); fireEvent.mouseDown(await screen.findByRole("tab", { name: "Tool servers" }), { button: 0 });
    expect(await screen.findByRole("alert")).toHaveTextContent("Unable to connect to the server");
    expect(screen.queryByRole("button", { name: "Try refreshing" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit connection" })).not.toBeInTheDocument();
  });
  it("offers to attach a just-created tool and recovers when assistants fail to load", async () => {
    setAuthSession("token", user); let agentsFail = true;
    server.use(
      http.get(apiUrl("/v1/tools"), () => jsonResponse({ items: [tool], next_cursor: null })),
      http.get(apiUrl("/v1/api-connections"), () => jsonResponse({ items: [], next_cursor: null })),
      http.get(apiUrl("/v1/mcp-servers"), () => jsonResponse({ items: [], next_cursor: null })),
      http.get(apiUrl("/v1/agents"), () => agentsFail ? jsonResponse({ status: 500 }, 500) : jsonResponse({ items: [{ id: "20000000-0000-4000-8000-000000000002", name: "Orders assistant", description: "", provider_id: "10000000-0000-4000-8000-000000000001", model: "gemini-fast", system_prompt: "", temperature: 0.5, max_output_tokens: 2048, max_iterations: 8, timeout_seconds: 120, status: "active", created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z", readiness: "ready", readiness_error: null }], next_cursor: null })),
    );
    renderPage(`/tools?created=${tool.id}`);
    expect(await screen.findByText(/Unable to load assistants/)).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "Choose assistant" })).not.toBeInTheDocument();
    agentsFail = false; fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("combobox", { name: "Choose assistant" })).toBeInTheDocument();
  });
});
