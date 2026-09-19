import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { AgentForm } from "./agent-form";

const greenNode = { id: "123e4567-e89b-12d3-a456-426614174000", name: "GreenNode", kind: "greennode" as const, base_url: null, default_model: "qwen/qwen3.6-flash", api_key_hint: "1234", status: "ok" as const, last_error: null, last_checked_at: null, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };

describe("AgentForm", () => {
  it("points to connection setup and preserves the return destination", async () => {
    server.use(http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [], next_cursor: null })));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter><AgentForm onSubmit={() => undefined} /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText("No model connections yet")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Add connection" })).toHaveAttribute("href", "/connections?returnTo=/agents/new");
  });
  it("shows provider loading failures separately and retries", async () => {
    let failed = true;
    server.use(http.get(apiUrl("/v1/providers"), () => failed ? jsonResponse({ status: 500 }, 500) : jsonResponse({ items: [], next_cursor: null })));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter><AgentForm onSubmit={() => undefined} /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText("Unable to load model connections")).toBeInTheDocument();
    failed = false; fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("No model connections yet")).toBeInTheDocument();
  });
  it("does not offer a write action in a read-only empty state", async () => {
    server.use(http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [], next_cursor: null })));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter><AgentForm readOnly onSubmit={() => undefined} /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText("This assistant has no available model connection.")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Add connection" })).not.toBeInTheDocument();
  });
  it("shows GreenNode models with brand logos without remote discovery", async () => {
    server.use(http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [greenNode], next_cursor: null })));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter><AgentForm preferredProviderId={greenNode.id} onSubmit={() => undefined} /></MemoryRouter></QueryClientProvider>);
    const modelPicker = await screen.findByRole("combobox", { name: "AI model" });
    expect(modelPicker).toHaveTextContent("qwen/qwen3.6-flash");
    fireEvent.click(modelPicker);
    expect(screen.getByPlaceholderText("Search models...")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Z.ai" })).toBeInTheDocument();
    expect(screen.getAllByRole("img", { name: "Qwen logo" })).toHaveLength(2);
  });
  it("offers clear conversation-capacity suggestions and accepts exact values", async () => {
    server.use(http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [greenNode], next_cursor: null })));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const onSubmit = vi.fn();
    render(<QueryClientProvider client={client}><MemoryRouter><AgentForm preferredProviderId={greenNode.id} onSubmit={onSubmit} /></MemoryRouter></QueryClientProvider>);
    await screen.findByRole("combobox", { name: "AI model" });
    fireEvent.click(screen.getByRole("button", { name: "Advanced options" }));
    const capacity = screen.getByRole("spinbutton", { name: /Conversation capacity/ });
    expect(capacity).toHaveValue(32768);
    fireEvent.change(capacity, { target: { value: "50000" } });
    expect(capacity).toHaveValue(50000);
    fireEvent.change(screen.getByRole("textbox", { name: "Assistant name" }), { target: { value: "Exact capacity" } });
    fireEvent.click(screen.getByRole("button", { name: "Save assistant" }));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ context_window_tokens: 50000 }), expect.anything()));
    // A capacity of 50,000 with the default 2,048 reply reserve leaves 42,952 and
    // starts summarizing at 32,214 — the point of showing it is that neither number
    // is guessable from the field.
    expect(screen.getByText(/Leaves/)).toHaveTextContent("42,952 tokens");
    expect(screen.getByText(/Leaves/)).toHaveTextContent("32,214 tokens");
  });

  it("shows the response-budget validation error", async () => {
    server.use(http.get(apiUrl("/v1/providers"), () => jsonResponse({ items: [greenNode], next_cursor: null })));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter><AgentForm preferredProviderId={greenNode.id} onSubmit={() => undefined} /></MemoryRouter></QueryClientProvider>);
    await screen.findByRole("combobox", { name: "AI model" });
    fireEvent.click(screen.getByRole("button", { name: "Advanced options" }));
    fireEvent.change(screen.getByRole("spinbutton", { name: "Maximum response length" }), { target: { value: "30000" } });
    fireEvent.click(screen.getByRole("button", { name: "Save assistant" }));
    expect(await screen.findByText(/leave room for instructions/)).toBeInTheDocument();
  });
});
