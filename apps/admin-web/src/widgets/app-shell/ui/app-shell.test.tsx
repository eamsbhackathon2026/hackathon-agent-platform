import { fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, describe, expect, it } from "vitest";

import { clearAuthSession, setAuthSession } from "@/shared/api";
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

  it("hides conversation history from navigation but keeps its route title", () => {
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

    expect(screen.queryByRole("link", { name: "Conversation History" })).not.toBeInTheDocument();
    expect(within(screen.getByRole("banner")).getByRole("heading", { name: "Conversation History" })).toBeInTheDocument();
    expect(screen.getByText("Conversation detail")).toBeInTheDocument();
  });
});
