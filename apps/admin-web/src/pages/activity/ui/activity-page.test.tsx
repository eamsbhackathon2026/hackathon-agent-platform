import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { ActivityPage } from "./activity-page";

const agent = {
  id: "agent-1", name: "Research assistant", description: "", provider_id: "provider-1", model: "model",
  system_prompt: "", temperature: null, max_output_tokens: null, max_iterations: 8, timeout_seconds: 120,
  created_by: "user-1", archived_at: null, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z",
  ready: true, readiness_error: null,
};

const run = {
  id: "run-1", agent_id: "agent-1", session_id: "session-1", mode: "stream" as const, status: "succeeded" as const,
  source: "playground" as const, triggered_by_user_id: "user-1", triggered_by_api_key_id: null,
  input: { message: "Compare the latest research" }, output: "Done", error: null, iterations: 3,
  usage: { input_tokens: 20, output_tokens: 5 }, metadata: {}, cancel_requested_at: null, queued_at: null,
  started_at: "2026-09-15T00:00:00Z", finished_at: "2026-09-15T00:00:02Z", created_at: "2026-09-15T00:00:00Z",
};

describe("ActivityPage", () => {
  it("honors the assistant deep link and makes the agent loop easy to open", async () => {
    const received = vi.fn();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      http.get(apiUrl("/v1/runs"), ({ request }) => {
        received(Object.fromEntries(new URL(request.url).searchParams));
        return jsonResponse({ items: [run], next_cursor: null });
      }),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const router = createMemoryRouter([{ path: "/activity", element: <ActivityPage /> }], { initialEntries: ["/activity?agent=agent-1"] });

    render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>);

    expect(await screen.findByText("Compare the latest research")).toBeInTheDocument();
    expect(screen.getByText("3 model passes")).toBeInTheDocument();
    expect(screen.getByText("25 processing units")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View loop" })).toHaveAttribute("href", "/activity/run-1");
    expect(received).toHaveBeenCalledWith({ limit: "25", agent_id: "agent-1" });
  });
});

describe("ActivityPage deep links", () => {
  it("starts from the window a report handed it", async () => {
    const received = vi.fn();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      http.get(apiUrl("/v1/runs"), ({ request }) => {
        received(Object.fromEntries(new URL(request.url).searchParams));
        return jsonResponse({ items: [run], next_cursor: null });
      }),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const entry = "/activity?agent=agent-1&status=failed&from=2026-09-10T00%3A00%3A00.000Z&to=2026-09-17T00%3A00%3A00.000Z";
    const router = createMemoryRouter([{ path: "/activity", element: <ActivityPage /> }], { initialEntries: [entry] });

    render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>);

    expect(await screen.findByText("Compare the latest research")).toBeInTheDocument();
    expect(received).toHaveBeenCalledWith({
      limit: "25",
      agent_id: "agent-1",
      status: "failed",
      from: "2026-09-10T00:00:00.000Z",
      to: "2026-09-17T00:00:00.000Z",
    });
  });

  it("ignores a status the API would reject instead of forwarding it", async () => {
    const received = vi.fn();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      http.get(apiUrl("/v1/runs"), ({ request }) => {
        received(Object.fromEntries(new URL(request.url).searchParams));
        return jsonResponse({ items: [run], next_cursor: null });
      }),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const router = createMemoryRouter([{ path: "/activity", element: <ActivityPage /> }], { initialEntries: ["/activity?status=constructor"] });

    render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>);

    expect(await screen.findByText("Compare the latest research")).toBeInTheDocument();
    expect(received).toHaveBeenCalledWith({ limit: "25" });
  });
});
