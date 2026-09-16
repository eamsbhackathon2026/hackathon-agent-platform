import { render, screen } from "@testing-library/react";
import { http } from "msw";
import { createMemoryRouter, Outlet, RouterProvider } from "react-router";
import { beforeEach, describe, expect, it } from "vitest";

import { clearAuthSession, setAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";

import { RequireAuth } from "./require-auth";
import { RequireWorkspaceAdmin } from "./require-workspace-admin";

const member = { id: "u", email: "member@example.test", name: "Member", role: "member" as const, status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" };

describe("route guards", () => {
  beforeEach(() => clearAuthSession());

  it("returns to sign in with returnTo when refresh fails", async () => {
    server.use(http.post(apiUrl("/v1/auth/refresh"), () => jsonResponse({ code: "unauthenticated" }, 401)));
    const router = createMemoryRouter([{ Component: RequireAuth, children: [{ path: "/activity", element: <p>Activity</p> }] }, { path: "/login", element: <p>Sign in again</p> }], { initialEntries: ["/activity"] });
    render(<RouterProvider router={router} />);
    expect(await screen.findByText("Sign in again")).toBeInTheDocument();
    expect(router.state.location.search).toContain("returnTo=%2Factivity");
  });

  it("shows the destination page when refresh restores the session", async () => {
    server.use(http.post(apiUrl("/v1/auth/refresh"), () => jsonResponse({ access_token: "restored", expires_in: 900, token_type: "Bearer", me: { user: member } })));
    const router = createMemoryRouter([{ Component: RequireAuth, children: [{ path: "/activity", element: <p>Restored activity</p> }] }], { initialEntries: ["/activity"] });
    render(<RouterProvider router={router} />);
    expect(await screen.findByText("Restored activity")).toBeInTheDocument();
  });

  it("shows an explanation instead of an admin page to members", async () => {
    setAuthSession("token", member);
    const router = createMemoryRouter([{ Component: RequireAuth, children: [{ element: <Outlet />, children: [{ Component: RequireWorkspaceAdmin, children: [{ path: "/members", element: <p>Member list</p> }] }] }] }], { initialEntries: ["/members"] });
    render(<RouterProvider router={router} />);
    expect(await screen.findByText("You do not have access to this page")).toBeInTheDocument();
    expect(screen.queryByText("Member list")).not.toBeInTheDocument();
  });
});
