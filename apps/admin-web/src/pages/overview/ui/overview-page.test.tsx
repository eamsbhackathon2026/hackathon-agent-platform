import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";

import { OverviewPage } from "./overview-page";

const emptyTotals = { requests: 0, succeeded: 0, failed: 0, cancelled: 0, success_rate: null, sessions: 0, avg_duration_ms: null, p95_duration_ms: null, processing_units: 0 };

const report = {
  range: { from: "2026-09-10T00:00:00Z", to: "2026-09-17T00:00:00Z", time_zone: "Asia/Ho_Chi_Minh" },
  totals: { requests: 120, succeeded: 108, failed: 9, cancelled: 3, success_rate: 0.9, sessions: 42, avg_duration_ms: 1500, p95_duration_ms: 4200, processing_units: 98_765 },
  daily: [
    { date: "2026-09-15", playground: 4, api: 20, failed: 1 },
    { date: "2026-09-16", playground: 0, api: 0, failed: 0 },
  ],
  top_agents: [{ agent_id: "agent-1", agent_name: "Support bot", requests: 80, succeeded: 76, failed: 4, success_rate: 0.95, p95_duration_ms: 3900, processing_units: 60_000 }],
  top_errors: [{ error_code: "tool_failed", count: 6, last_seen_at: "2026-09-16T08:00:00Z", sample_run_id: "run-9" }],
  top_tools: [{ tool_name: "lookup_invoice", calls: 51, errors: 4, p95_duration_ms: 800 }],
  step_limit_hits: 2,
};

function renderPage(entry = "/overview") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter([{ path: "/overview", element: <OverviewPage /> }], { initialEntries: [entry] });
  render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>);
  return router;
}

describe("OverviewPage", () => {
  it("summarises the window and links every ranking back to the same period", async () => {
    const received = vi.fn();
    server.use(http.get(apiUrl("/v1/reports/overview"), ({ request }) => {
      received(Object.fromEntries(new URL(request.url).searchParams));
      return jsonResponse(report);
    }));

    renderPage();

    expect(await screen.findByText("120")).toBeInTheDocument();
    expect(screen.getByText("90%")).toBeInTheDocument();
    expect(screen.getByText("42 conversations")).toBeInTheDocument();
    expect(screen.getByText("Support bot")).toBeInTheDocument();
    expect(screen.getByText("lookup_invoice")).toBeInTheDocument();
    expect(screen.getByText(/2 requests ran out of steps/)).toBeInTheDocument();

    const link = screen.getByRole("link", { name: /View requests/ });
    expect(link.getAttribute("href")).toContain("agent=agent-1");
    expect(link.getAttribute("href")).toContain("from=");
    expect(screen.getByRole("link", { name: /View an example/ })).toHaveAttribute("href", "/activity/run-9");

    const failures = screen.getByRole("link", { name: "The tool did not complete" });
    expect(failures.getAttribute("href")).toContain("status=failed");
    expect(failures.getAttribute("href")).toContain("from=");

    const query = received.mock.calls[0]?.[0] as { from: string; to: string; time_zone: string };
    expect(query.time_zone).toBeTruthy();
    expect(Date.parse(query.to) - Date.parse(query.from)).toBe(7 * 24 * 60 * 60 * 1000);
  });

  it("asks the server for the period named on the URL", async () => {
    const received = vi.fn();
    server.use(http.get(apiUrl("/v1/reports/overview"), ({ request }) => {
      received(Object.fromEntries(new URL(request.url).searchParams));
      return jsonResponse(report);
    }));

    renderPage("/overview?range=24h");

    expect(await screen.findByText("120")).toBeInTheDocument();
    const query = received.mock.calls[0]?.[0] as { from: string; to: string; time_zone: string };
    expect(Date.parse(query.to) - Date.parse(query.from)).toBe(24 * 60 * 60 * 1000);
  });

  it("keeps the chosen period on the URL so the view can be shared", async () => {
    server.use(http.get(apiUrl("/v1/reports/overview"), () => jsonResponse(report)));
    const router = renderPage();

    expect(await screen.findByText("120")).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Last 30 days" }), { button: 0, ctrlKey: false });
    expect(router.state.location.search).toContain("range=30d");
  });

  it("points a quiet workspace at the next step instead of an empty chart", async () => {
    server.use(http.get(apiUrl("/v1/reports/overview"), () => jsonResponse({ ...report, totals: emptyTotals, daily: [], top_agents: [], top_errors: [], top_tools: [], step_limit_hits: 0 })));

    renderPage();

    expect(await screen.findByText(/No activity in last 7 days/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Open Playground/ })).toHaveAttribute("href", "/playground");
  });
});
