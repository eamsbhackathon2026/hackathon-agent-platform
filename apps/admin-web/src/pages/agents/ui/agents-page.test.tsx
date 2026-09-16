import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it } from "vitest";
import { clearAuthSession, setAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { AgentsPage } from "./agents-page";

afterEach(() => clearAuthSession());
describe("AgentsPage", () => {
  it("does not show the empty state when loading fails", async () => {
    server.use(http.get(apiUrl("/v1/agents"), () => jsonResponse({ status: 500 }, 500)));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); render(<QueryClientProvider client={client}><MemoryRouter><AgentsPage /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText(/Unable to load assistants/)).toBeInTheDocument();
    expect(screen.queryByText("No assistants yet")).not.toBeInTheDocument();
  });
  it("does not offer a create action to a member when the list is empty", async () => {
    setAuthSession("token", { id: "member-1", email: "member@example.test", name: "Member", role: "member", status: "active", must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" });
    server.use(http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); render(<QueryClientProvider client={client}><MemoryRouter><AgentsPage /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText("Contact an administrator to create the first assistant.")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Create assistant/ })).not.toBeInTheDocument();
  });
});
