import { fireEvent, render, screen } from "@testing-library/react";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { clearAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";

import { LoginForm } from "./login-form";

describe("LoginForm", () => {
  it("validates email and password before submitting", async () => {
    const router = createMemoryRouter([{ path: "/login", element: <LoginForm /> }], { initialEntries: ["/login"] });
    render(<RouterProvider router={router} />);
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByText("Enter a valid email address.")).toBeInTheDocument();
    expect(screen.getByText("Enter your password.")).toBeInTheDocument();
  });

  it("navigates to password change when required by the server", async () => {
    clearAuthSession();
    server.use(http.post(apiUrl("/v1/auth/login"), () => jsonResponse({ access_token: "token", expires_in: 900, token_type: "Bearer", me: { user: { id: "u", email: "member@example.test", name: "Member", role: "member", status: "active", must_change_password: true, last_login_at: null, created_at: "2026-09-15T00:00:00Z" } } })));
    const router = createMemoryRouter([{ path: "/login", element: <LoginForm /> }, { path: "/change-password", element: <p>Change password now</p> }], { initialEntries: ["/login"] });
    render(<RouterProvider router={router} />);
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "member@example.test" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "password" } });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByText("Change password now")).toBeInTheDocument();
  });

  it("does not reveal whether an account exists after invalid credentials", async () => {
    server.use(http.post(apiUrl("/v1/auth/login"), () => jsonResponse({ type: "about:blank", title: "Invalid credentials", status: 401, detail: "", code: "unauthenticated", fields: [] }, 401)));
    const router = createMemoryRouter([{ path: "/login", element: <LoginForm /> }], { initialEntries: ["/login"] });
    render(<RouterProvider router={router} />);
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "unknown@example.test" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "wrong" } });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByText("The email or password is incorrect. Please try again.")).toBeInTheDocument();
  });
});
