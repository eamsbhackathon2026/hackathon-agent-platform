import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";

import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";

import { FirstRunSetupPage } from "./first-run-setup-page";

function renderSetup() {
  const router = createMemoryRouter([{ path: "/setup", element: <FirstRunSetupPage /> }, { path: "/login", element: <p>Sign-in page</p> }, { path: "/agents", element: <p>Assistants page</p> }], { initialEntries: ["/setup"] });
  const query = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={query}><RouterProvider router={router} /></QueryClientProvider>);
}

describe("FirstRunSetupPage", () => {
  it("hides setup when an account already exists", async () => {
    server.use(http.get(apiUrl("/v1/auth/config"), () => jsonResponse({ signup_allowed: false })));
    renderSetup();
    expect(await screen.findByText("Sign-in page")).toBeInTheDocument();
  });

  it("submits only name, email, and password when setup is allowed", async () => {
    let requestBody: Record<string, unknown> = {};
    server.use(
      http.get(apiUrl("/v1/auth/config"), () => jsonResponse({ signup_allowed: true })),
      http.post(apiUrl("/v1/auth/register"), async ({ request }) => {
        requestBody = await request.json() as Record<string, unknown>;
        return jsonResponse({ access_token: "token", expires_in: 900, token_type: "Bearer", me: { user: { id: "u", email: "owner@example.test", name: "Owner", role: "owner", status: "active", must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" } } }, 201);
      }),
    );
    renderSetup();
    fireEvent.change(await screen.findByLabelText("Your name"), { target: { value: "Owner" } });
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "owner@example.test" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "strong-pass-123" } });
    fireEvent.click(screen.getByRole("button", { name: "Complete setup" }));
    expect(await screen.findByText("Assistants page")).toBeInTheDocument();
    await waitFor(() => expect(requestBody).toEqual({ name: "Owner", email: "owner@example.test", password: "strong-pass-123" }));
  });

  it("explains when another person has just completed setup", async () => {
    server.use(
      http.get(apiUrl("/v1/auth/config"), () => jsonResponse({ signup_allowed: true })),
      http.post(apiUrl("/v1/auth/register"), () => jsonResponse({ type: "about:blank", title: "Conflict", status: 409, detail: "", code: "conflict", fields: [] }, 409)),
    );
    renderSetup();
    fireEvent.change(await screen.findByLabelText("Your name"), { target: { value: "Second" } });
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "second@example.test" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "strong-pass-123" } });
    fireEvent.click(screen.getByRole("button", { name: "Complete setup" }));
    expect(await screen.findByText("Another person has already completed setup. Continue to sign in.")).toBeInTheDocument();
  });
});
