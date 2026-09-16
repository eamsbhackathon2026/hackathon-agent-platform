import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { IntegrationsPage } from "./integrations-page";

describe("IntegrationsPage", () => {
  it("does not show the empty key state when loading fails", async () => {
    server.use(http.get(apiUrl("/v1/api-keys"), () => jsonResponse({ status: 500 }, 500)), http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); render(<QueryClientProvider client={client}><MemoryRouter><IntegrationsPage /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText(/Unable to load API access data/)).toBeInTheDocument();
    expect(screen.queryByText("No access keys yet")).not.toBeInTheDocument();
  });
  it("does not create a key before an assistant is selected", async () => {
    const createRequest = vi.fn(); server.use(http.get(apiUrl("/v1/api-keys"), () => jsonResponse({ items: [], next_cursor: null })), http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })), http.post(apiUrl("/v1/api-keys"), () => { createRequest(); return jsonResponse({}, 500); }));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); render(<QueryClientProvider client={client}><MemoryRouter><IntegrationsPage /></MemoryRouter></QueryClientProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Create key" })); const dialog = screen.getByRole("dialog", { name: "New access key" }); const name = within(dialog).getByLabelText("Key name"); fireEvent.change(name, { target: { value: "Website" } }); fireEvent.submit(name.closest("form")!);
    expect(createRequest).not.toHaveBeenCalled();
  });
  it("opens the access key form from the integration guide", () => {
    server.use(http.get(apiUrl("/v1/api-keys"), () => jsonResponse({ items: [], next_cursor: null })), http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); render(<QueryClientProvider client={client}><MemoryRouter><IntegrationsPage /></MemoryRouter></QueryClientProvider>);
    fireEvent.click(screen.getByRole("button", { name: "View integration guide" }));
    fireEvent.click(screen.getByRole("button", { name: "Create access key" }));
    expect(screen.getByRole("dialog", { name: "New access key" })).toBeInTheDocument();
  });
});
