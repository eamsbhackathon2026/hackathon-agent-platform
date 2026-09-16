import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { afterEach, describe, expect, it } from "vitest";
import { clearAuthSession, setAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { ToolEditPage } from "./tool-edit-page";

const user = { id: "user-1", email: "user@example.test", name: "User", role: "admin" as const, status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" };
const connection = { id: "connection-1", slug: "commerce", display_name: "Commerce API", base_url: "https://api.example.test/v1", public_headers: {}, secret_header_names: ["Authorization"], created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };
const tool = { id: "tool-1", connection_id: connection.id, kind: "http" as const, slug: "orders", display_name: "Orders", description: "Lists orders", method: "POST" as const, url_template: "/orders", params: [], public_headers: {}, secret_header_names: [], timeout_seconds: 15, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };

// Renders the page under real routes so navigation after saving can be observed.
function Probe() { const location = useLocation(); return <p data-testid="location">{location.pathname}{location.search}</p>; }
function renderAt(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><Routes><Route path="/tools/new" element={<ToolEditPage />} /><Route path="/tools/:toolId/edit" element={<ToolEditPage />} /><Route path="/tools" element={<Probe />} /><Route path="/agents/new" element={<Probe />} /></Routes></MemoryRouter></QueryClientProvider>);
}

afterEach(() => clearAuthSession());
describe("ToolEditPage", () => {
  it("keeps the page read-only for a member", async () => {
    setAuthSession("token", { ...user, role: "member" }); renderAt("/tools/new");
    expect(await screen.findByText("Only administrators can edit tools. Contact an administrator for help.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save tool" })).not.toBeInTheDocument();
  });

  it("creates a tool and returns to the list with the new tool selected", async () => {
    setAuthSession("token", user); let created: unknown;
    server.use(
      http.get(apiUrl("/v1/api-connections"), () => jsonResponse({ items: [connection], next_cursor: null })),
      http.post(apiUrl("/v1/tools"), async ({ request }) => { created = await request.json(); return jsonResponse(tool, 201); }),
    );
    renderAt("/tools/new?returnTo=/agents/new");
    expect(await screen.findByRole("heading", { name: "Create HTTP tool" })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Display name"), { target: { value: "Orders" } });
    fireEvent.change(screen.getByLabelText("Short name"), { target: { value: "orders" } });
    const selector = screen.getByLabelText("API connection");
    await waitFor(() => expect(selector).toHaveTextContent("Commerce API"));
    fireEvent.change(selector, { target: { value: connection.id } });
    fireEvent.change(screen.getByLabelText("Operation path"), { target: { value: "/orders" } });
    // The description is multi-line: a tool's purpose is what the model reads.
    const description = screen.getByLabelText("Description");
    expect(description.tagName).toBe("TEXTAREA");
    fireEvent.change(description, { target: { value: "Lists orders\nfor one customer" } });
    // The connection summary resolves the address the request will hit.
    expect(screen.getByRole("group", { name: "Connection details" })).toHaveTextContent("https://api.example.test/v1/orders");
    fireEvent.click(screen.getByRole("button", { name: "Save tool" }));
    await waitFor(() => expect(screen.getByTestId("location")).toHaveTextContent("/tools?created=tool-1&returnTo=%2Fagents%2Fnew"));
    expect(created).toMatchObject({ slug: "orders", display_name: "Orders", description: "Lists orders\nfor one customer", method: "GET", url_template: "/orders", connection_id: connection.id });
  });

  it("loads an existing tool, saves changes, and returns to the list", async () => {
    setAuthSession("token", user); let patched: unknown;
    server.use(
      http.get(apiUrl("/v1/api-connections"), () => jsonResponse({ items: [connection], next_cursor: null })),
      http.get("*/v1/tools/:toolId", () => jsonResponse(tool)),
      http.patch("*/v1/tools/:toolId", async ({ request }) => { patched = await request.json(); return jsonResponse({ ...tool, display_name: "Orders v2" }); }),
    );
    renderAt(`/tools/${tool.id}/edit`);
    expect(await screen.findByRole("heading", { name: "Edit tool" })).toBeInTheDocument();
    expect(screen.getByLabelText("Display name")).toHaveValue("Orders");
    expect(screen.getByLabelText("Description")).toHaveValue("Lists orders");
    fireEvent.change(screen.getByLabelText("Display name"), { target: { value: "Orders v2" } });
    fireEvent.click(screen.getByRole("button", { name: "Save tool" }));
    await waitFor(() => expect(screen.getByTestId("location")).toHaveTextContent("/tools"));
    expect(patched).toMatchObject({ display_name: "Orders v2", method: "POST", connection_id: connection.id });
  });

  it("explains when the tool cannot be loaded", async () => {
    setAuthSession("token", user);
    server.use(http.get(apiUrl("/v1/api-connections"), () => jsonResponse({ items: [], next_cursor: null })), http.get("*/v1/tools/:toolId", () => jsonResponse({ status: 404 }, 404)));
    renderAt("/tools/missing/edit");
    expect(await screen.findByText("Unable to load this tool. It may have been deleted.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to tools" })).toHaveAttribute("href", "/tools");
  });
});
