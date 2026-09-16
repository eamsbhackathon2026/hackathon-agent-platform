import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it } from "vitest";
import { clearAuthSession, setAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { MembersPage } from "./members-page";

const base = { status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" };
afterEach(() => clearAuthSession());
describe("MembersPage", () => {
  it("does not show the empty member state when loading fails", async () => {
    setAuthSession("token", { ...base, id: "admin-1", email: "actor@example.test", name: "Actor", role: "admin" });
    server.use(http.get(apiUrl("/v1/members"), () => jsonResponse({ status: 500 }, 500)));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); render(<QueryClientProvider client={client}><MemoryRouter><MembersPage /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText(/Unable to load members/)).toBeInTheDocument();
    expect(screen.queryByText("No other members yet")).not.toBeInTheDocument();
  });
  it("prevents an admin from assigning or changing elevated accounts", async () => {
    const actor = { ...base, id: "admin-1", email: "actor@example.test", name: "Actor", role: "admin" as const }; setAuthSession("token", actor);
    server.use(http.get(apiUrl("/v1/members"), () => jsonResponse({ items: [{ ...base, id: "admin-2", email: "admin@example.test", name: "Other Admin", role: "admin" }, { ...base, id: "member-1", email: "member@example.test", name: "Member", role: "member" }], next_cursor: null })));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); render(<QueryClientProvider client={client}><MemoryRouter><MembersPage /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByLabelText("Role for Other Admin")).toBeDisabled(); expect(screen.getByLabelText("Role for Member")).toBeEnabled();
    const memberList = screen.getByRole("region", { name: "Member list" });
    expect(within(memberList).getByText("Workspace members")).toBeInTheDocument();
    expect(within(memberList).getByText("2 people can access this workspace")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Add member" })); const dialog = screen.getByRole("dialog", { name: "New member" }); const role = within(dialog).getByLabelText("Role"); expect(within(role).queryByRole("option", { name: "Administrator" })).not.toBeInTheDocument();
  });
});
