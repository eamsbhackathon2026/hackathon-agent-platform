import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { clearAuthSession, setAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { ConnectionsPage } from "./connections-page";

const provider = { id: "provider-1", name: "Primary Gemini", kind: "gemini" as const, base_url: null, default_model: "gemini-fast", api_key_hint: "1234", status: "ok" as const, last_error: null, last_checked_at: null, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };
const user = { id: "user-1", email: "admin@example.test", name: "Admin", role: "admin" as const, status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" };

afterEach(() => { clearAuthSession(); vi.restoreAllMocks(); });
describe("ConnectionsPage", () => {
  it("keeps a new connection model empty and disables credential autofill", async () => {
    setAuthSession("token", user); server.use(http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [], next_cursor: null })));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); render(<QueryClientProvider client={client}><MemoryRouter><ConnectionsPage /></MemoryRouter></QueryClientProvider>);
    expect(screen.getByRole("heading", { name: "Model Connections" })).toBeInTheDocument();
    fireEvent.click(await screen.findByRole("button", { name: "Add connection" }));
    expect(screen.getByLabelText("Default model")).toHaveValue("");
    expect(screen.getByLabelText("Default model")).toHaveAttribute("autocomplete", "off");
    expect(screen.getByLabelText("API key")).toHaveAttribute("autocomplete", "new-password");
  });
  it("returns to agent creation with the new provider selected", async () => {
    setAuthSession("token", user);
    const created = { ...provider, id: "10000000-0000-4000-8000-000000000001" };
    server.use(http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [], next_cursor: null })), http.post(apiUrl("/v1/providers"), () => jsonResponse(created, 201)));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const LocationMarker = () => { const location = useLocation(); return <p>route:{location.pathname}{location.search}</p>; };
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={["/connections?returnTo=/agents/new"]}><Routes><Route path="/connections" element={<ConnectionsPage />} /><Route path="*" element={<LocationMarker />} /></Routes></MemoryRouter></QueryClientProvider>);
    fireEvent.click(await screen.findByRole("button", { name: "Add connection" }));
    fireEvent.change(screen.getByLabelText("Connection name"), { target: { value: "Primary Gemini" } });
    fireEvent.change(screen.getByLabelText("Default model"), { target: { value: "gemini-2.5-flash" } });
    fireEvent.change(screen.getByLabelText("API key"), { target: { value: "test-key" } });
    fireEvent.click(screen.getByRole("button", { name: "Save connection" }));
    expect(await screen.findByText(`route:/agents/new?provider=${created.id}`)).toBeInTheDocument();
  });
  it("creates GreenNode with its managed endpoint and curated model catalog", async () => {
    setAuthSession("token", user);
    let requestBody: unknown;
    const created = { ...provider, id: "20000000-0000-4000-8000-000000000002", name: "GreenNode", kind: "greennode" as const, default_model: "z-ai/glm-5.2-hackathon" };
    server.use(
      http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [], next_cursor: null })),
      http.post(apiUrl("/v1/providers"), async ({ request }) => { requestBody = await request.json(); return jsonResponse(created, 201); }),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter><ConnectionsPage /></MemoryRouter></QueryClientProvider>);
    fireEvent.click(await screen.findByRole("button", { name: "Add connection" }));
    fireEvent.change(screen.getByLabelText("Service type"), { target: { value: "greennode" } });
    expect(screen.queryByLabelText("Server address")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Default model")).toHaveValue("z-ai/glm-5.2-hackathon");
    expect(screen.getByRole("img", { name: "Z.ai" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Qwen logo" })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Connection name"), { target: { value: "GreenNode" } });
    fireEvent.change(screen.getByLabelText("API key"), { target: { value: "test-key" } });
    fireEvent.click(screen.getByRole("button", { name: "Save connection" }));
    await waitFor(() => expect(requestBody).toEqual({ name: "GreenNode", kind: "greennode", base_url: null, default_model: "z-ai/glm-5.2-hackathon", api_key: "test-key" }));
  });
  it("shows the assistants that block deletion", async () => {
    setAuthSession("token", user); vi.spyOn(window, "confirm").mockReturnValue(true); server.use(http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [provider], next_cursor: null })), http.delete("*/v1/providers/:providerId", () => jsonResponse({ type: "about:blank", title: "In use", status: 409, detail: "", code: "conflict", fields: [], related_agents: [{ id: "agent-1", name: "Orders assistant" }] }, 409)));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); render(<QueryClientProvider client={client}><MemoryRouter><ConnectionsPage /></MemoryRouter></QueryClientProvider>);
    fireEvent.click(await screen.findByRole("button", { name: "Delete" }));
    expect(await screen.findByRole("link", { name: "Orders assistant" })).toHaveAttribute("href", "/agents/agent-1");
  });
  it("loads models for the exact connection being edited", async () => {
    setAuthSession("token", user); server.use(http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [provider], next_cursor: null })), http.get("*/v1/providers/:providerId/models", ({ params }) => jsonResponse({ items: [{ id: `${params.providerId}-fast`, display_name: "Gemini Fast" }], next_cursor: null })));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); render(<QueryClientProvider client={client}><MemoryRouter><ConnectionsPage /></MemoryRouter></QueryClientProvider>);
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    const modelPicker = await screen.findByRole("combobox", { name: "Default model" });
    expect(modelPicker).toHaveTextContent("gemini-fast");
    fireEvent.click(modelPicker);
    const search = screen.getByPlaceholderText("Search models...");
    fireEvent.change(search, { target: { value: "Gemini Fast" } });
    expect(await screen.findByRole("option", { name: /Gemini Fast/ })).toBeInTheDocument();
  });
  it("keeps the saved API key masked until the user chooses to replace it", async () => {
    setAuthSession("token", user);
    let requestBody: Record<string, unknown> | undefined;
    server.use(
      http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [provider], next_cursor: null })),
      http.get("*/v1/providers/:providerId/models", () => jsonResponse({ items: [{ id: "gemini-fast", display_name: "Gemini Fast" }], next_cursor: null })),
      http.patch("*/v1/providers/:providerId", async ({ request }) => { requestBody = await request.json() as Record<string, unknown>; return jsonResponse(provider); }),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter><ConnectionsPage /></MemoryRouter></QueryClientProvider>);
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    expect(screen.getByRole("button", { name: "Update API key" })).toHaveTextContent("••••••••••••");
    expect(screen.queryByLabelText("API key")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Save connection" }));
    await waitFor(() => expect(requestBody).toBeDefined());
    expect(requestBody).not.toHaveProperty("api_key");
  });
  it("reveals a fresh input and sends only the replacement API key", async () => {
    setAuthSession("token", user);
    let requestBody: Record<string, unknown> | undefined;
    server.use(
      http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [provider], next_cursor: null })),
      http.get("*/v1/providers/:providerId/models", () => jsonResponse({ items: [{ id: "gemini-fast", display_name: "Gemini Fast" }], next_cursor: null })),
      http.patch("*/v1/providers/:providerId", async ({ request }) => { requestBody = await request.json() as Record<string, unknown>; return jsonResponse(provider); }),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter><ConnectionsPage /></MemoryRouter></QueryClientProvider>);
    fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
    fireEvent.click(screen.getByRole("button", { name: "Update API key" }));
    const input = screen.getByLabelText("API key");
    expect(input).toHaveAttribute("placeholder", "Enter a new API key");
    fireEvent.change(input, { target: { value: "replacement-key" } });
    fireEvent.click(screen.getByRole("button", { name: "Save connection" }));
    await waitFor(() => expect(requestBody).toMatchObject({ api_key: "replacement-key" }));
  });
});
