import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { beforeEach, describe, expect, it } from "vitest";

import { clearAuthSession, setAuthSession } from "@/shared/api";

import { HomeRedirect } from "./home-redirect";

const base = { id: "u", email: "person@example.test", name: "Person", status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" };

function renderHome() {
  const router = createMemoryRouter([
    { index: true, Component: HomeRedirect },
    { path: "/overview", element: <p>Workspace overview</p> },
    { path: "/agents", element: <p>Assistant list</p> },
  ], { initialEntries: ["/"] });
  render(<RouterProvider router={router} />);
}

describe("HomeRedirect", () => {
  beforeEach(() => clearAuthSession());

  it("sends an administrator to the workspace overview", async () => {
    setAuthSession("token", { ...base, role: "admin" });
    renderHome();
    expect(await screen.findByText("Workspace overview")).toBeInTheDocument();
  });

  it("sends a member to their assistants, because reporting is not theirs to see", async () => {
    setAuthSession("token", { ...base, role: "member" });
    renderHome();
    expect(await screen.findByText("Assistant list")).toBeInTheDocument();
  });
});
