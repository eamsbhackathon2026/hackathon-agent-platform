import { fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, describe, expect, it } from "vitest";

import { clearAuthSession, setAuthSession } from "@/shared/api";
import { usePageHeader } from "@/shared/lib";
import { AppShell } from "./app-shell";

const user = {
  id: "user-1",
  email: "admin@ap.com",
  name: "Admin",
  role: "owner" as const,
  status: "active" as const,
  must_change_password: false,
  last_login_at: null,
  created_at: "2026-09-15T00:00:00Z",
};

afterEach(() => clearAuthSession());

describe("AppShell", () => {
  it("moves the current user into a sidebar account menu", async () => {
    setAuthSession("token", user);
    render(
      <MemoryRouter initialEntries={["/agents"]}>
        <Routes>
          <Route element={<AppShell />}>
            <Route path="/agents" element={<p>Assistant list</p>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    const account = screen.getByRole("group", { name: "Current user" });
    expect(within(account).getByText("Admin")).toBeInTheDocument();
    expect(within(account).getByText("Owner")).toBeInTheDocument();
    expect(within(screen.getByRole("banner")).queryByText("Admin")).not.toBeInTheDocument();

    fireEvent.pointerDown(within(account).getByRole("button", { name: "Open account menu for Admin" }), { button: 0, ctrlKey: false });
    expect(await screen.findByRole("menuitem", { name: "Change password" })).toHaveAttribute("href", "/change-password");
    expect(screen.getByRole("menuitem", { name: "Sign out" })).toBeInTheDocument();
  });

  it("lists conversation history in navigation and titles its detail route", () => {
    setAuthSession("token", user);
    render(
      <MemoryRouter initialEntries={["/conversations/session-1"]}>
        <Routes>
          <Route element={<AppShell />}>
            <Route path="/conversations/:sessionId" element={<p>Conversation detail</p>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getAllByRole("link", { name: "Conversation History" }).length).toBeGreaterThan(0);
    expect(within(screen.getByRole("banner")).getByRole("heading", { name: "Conversation History" })).toBeInTheDocument();
    expect(screen.getByText("Conversation detail")).toBeInTheDocument();
  });

  it("titles a page from its navigation entry", () => {
    setAuthSession("token", user);
    renderAt("/agents", <p>Assistant list</p>);

    const banner = within(screen.getByRole("banner"));
    expect(banner.getByRole("heading", { name: "Agent Hub" })).toBeInTheDocument();
    expect(banner.getByText("Create an assistant for each workflow and test it when it is ready.")).toBeInTheDocument();
  });

  it("lets a detail page replace the title its navigation entry supplies", () => {
    setAuthSession("token", user);
    function DetailPage() {
      usePageHeader({ title: "Support bot", description: "Update how this assistant works." });
      return <p>Assistant detail</p>;
    }
    renderAt("/agents/agent-1", <DetailPage />, "/agents/:agentId");

    const banner = within(screen.getByRole("banner"));
    expect(banner.getByRole("heading", { name: "Support bot" })).toBeInTheDocument();
    expect(banner.getByText("Update how this assistant works.")).toBeInTheDocument();
    expect(banner.queryByRole("heading", { name: "Agent Hub" })).not.toBeInTheDocument();
  });
});

function renderAt(entry: string, element: React.ReactElement, path = entry) {
  render(
    <MemoryRouter initialEntries={[entry]}>
      <Routes>
        <Route element={<AppShell />}>
          <Route path={path} element={element} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}
