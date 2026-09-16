import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearAuthSession, setAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { jsonResponse } from "@/test/typed-handlers";
import { ActivityDetailPage } from "./activity-detail-page";

const baseUser = { id: "user-1", email: "user@example.test", name: "User", status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" };
const run = {
  id: "run-1", agent_id: "agent-1", session_id: "session-1", mode: "stream" as const, status: "succeeded" as const,
  source: "playground" as const, triggered_by_user_id: "user-1", triggered_by_api_key_id: null,
  input: { message: "Hello" }, output: "Result", error: null, iterations: 1,
  usage: { input_tokens: 2, output_tokens: 3 }, metadata: {}, cancel_requested_at: null, queued_at: null,
  started_at: "2026-09-15T00:00:00Z", finished_at: "2026-09-15T00:00:01Z", created_at: "2026-09-15T00:00:00Z",
};

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter([{ path: "/activity/:runId", element: <ActivityDetailPage /> }], { initialEntries: ["/activity/run-1"] });
  render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>);
  return client;
}

afterEach(clearAuthSession);
beforeEach(() => {
  server.use(http.get("*/v1/sessions/:sessionId/messages", () => jsonResponse({ items: [], next_cursor: null })));
});

describe("ActivityDetailPage", () => {
  it("does not request or show delivery controls to a member", async () => {
    setAuthSession("token", { ...baseUser, role: "member" });
    const deliveryRequest = vi.fn();
    server.use(
      http.get("*/v1/runs/:runId", () => jsonResponse(run)),
      http.get("*/v1/runs/:runId/spans", () => jsonResponse({ items: [], next_cursor: null })),
      http.get("*/v1/runs/:runId/webhook-deliveries", () => { deliveryRequest(); return jsonResponse({ items: [], next_cursor: null }); }),
    );
    renderPage();
    expect(await screen.findAllByText("Result")).toHaveLength(2);
    expect(screen.queryByText("Deliver results to another system")).not.toBeInTheDocument();
    await waitFor(() => expect(deliveryRequest).not.toHaveBeenCalled());
  });

  it("shows delivery history to an administrator", async () => {
    setAuthSession("token", { ...baseUser, role: "admin" });
    server.use(
      http.get("*/v1/runs/:runId", () => jsonResponse(run)),
      http.get("*/v1/runs/:runId/spans", () => jsonResponse({ items: [], next_cursor: null })),
      http.get("*/v1/runs/:runId/webhook-deliveries", () => jsonResponse({ items: [], next_cursor: null })),
    );
    renderPage();
    expect(await screen.findByText("Deliver results to another system")).toBeInTheDocument();
  });

  it("shows a retry action when the detail query fails", async () => {
    setAuthSession("token", { ...baseUser, role: "member" });
    server.use(http.get("*/v1/runs/:runId", () => jsonResponse({ type: "about:blank", title: "Error", status: 500, detail: "", code: "internal", fields: [] }, 500)));
    renderPage();
    expect(await screen.findByText("Something went wrong")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument();
  });

  it("links interrupted processing back to the same assistant", async () => {
    setAuthSession("token", { ...baseUser, role: "member" });
    server.use(
      http.get("*/v1/runs/:runId", () => jsonResponse({ ...run, status: "failed", error: { code: "interrupted", message: "Interrupted" } })),
      http.get("*/v1/runs/:runId/spans", () => jsonResponse({ items: [], next_cursor: null })),
    );
    renderPage();
    expect(await screen.findByRole("link", { name: "Open playground" })).toHaveAttribute("href", "/playground?agent=agent-1&session=session-1");
  });

  it.each([
    ["another user's playground run", { triggered_by_user_id: "user-2" }],
    ["an API run", { source: "api" as const, triggered_by_user_id: null, triggered_by_api_key_id: "key-1" }],
  ])("does not continue the session for %s", async (_label, overrides) => {
    setAuthSession("token", { ...baseUser, role: "admin" });
    server.use(
      http.get("*/v1/runs/:runId", () => jsonResponse({ ...run, ...overrides, status: "failed", error: { code: "interrupted", message: "Interrupted" } })),
      http.get("*/v1/runs/:runId/spans", () => jsonResponse({ items: [], next_cursor: null })),
      http.get("*/v1/runs/:runId/webhook-deliveries", () => jsonResponse({ items: [], next_cursor: null })),
    );
    renderPage();
    expect(await screen.findByRole("link", { name: "Open playground" })).toHaveAttribute("href", "/playground?agent=agent-1");
  });

  it("explains that steps arrive after an active run instead of spinning forever", async () => {
    setAuthSession("token", { ...baseUser, role: "member" });
    const spansRequest = vi.fn();
    server.use(
      http.get("*/v1/runs/:runId", () => jsonResponse({ ...run, status: "running", finished_at: null, output: null })),
      http.get("*/v1/runs/:runId/spans", () => { spansRequest(); return jsonResponse({ items: [], next_cursor: null }); }),
    );
    renderPage();
    expect(await screen.findByText(/Timing becomes available/)).toBeInTheDocument();
    expect(screen.getByText("Live updates")).toBeInTheDocument();
    expect(spansRequest).not.toHaveBeenCalled();
  });

  it("loads deliveries only after a managed run becomes terminal", async () => {
    setAuthSession("token", { ...baseUser, role: "admin" });
    let runRequestCount = 0;
    const deliveryRequest = vi.fn();
    server.use(
      http.get("*/v1/runs/:runId", () => {
        runRequestCount += 1;
        return jsonResponse(runRequestCount === 1 ? { ...run, status: "running", finished_at: null } : run);
      }),
      http.get("*/v1/runs/:runId/spans", () => jsonResponse({ items: [], next_cursor: null })),
      http.get("*/v1/runs/:runId/webhook-deliveries", () => {
        deliveryRequest();
        return jsonResponse({ items: [], next_cursor: null });
      }),
    );
    const client = renderPage();
    expect(await screen.findByText(/Timing becomes available/)).toBeInTheDocument();
    expect(deliveryRequest).not.toHaveBeenCalled();

    await client.invalidateQueries({ queryKey: ["runs", "run-1"] });

    await waitFor(() => expect(deliveryRequest).toHaveBeenCalledOnce());
    expect(await screen.findByText("Deliver results to another system")).toBeInTheDocument();
  });

  it("cancels an in-flight message snapshot and reloads it when an active run becomes terminal", async () => {
    setAuthSession("token", { ...baseUser, role: "member" });
    let runRequestCount = 0;
    let messageRequestCount = 0;
    const requestMessage = { id: "request", session_id: "session-1", run_id: "run-1", seq: 1, role: "user", content: "Hello", tool_calls: [], tool_call_id: null, tool_name: null, is_error: false, created_at: run.created_at };
    const answerMessage = { ...requestMessage, id: "answer", seq: 2, role: "assistant", content: "Final saved answer" };
    server.use(
      http.get("*/v1/runs/:runId", () => {
        runRequestCount += 1;
        return jsonResponse(runRequestCount === 1 ? { ...run, status: "running", finished_at: null, output: null } : run);
      }),
      http.get("*/v1/sessions/:sessionId/messages", async ({ request }) => {
        messageRequestCount += 1;
        if (messageRequestCount === 1) {
          await new Promise<void>((resolve) => request.signal.addEventListener("abort", () => resolve(), { once: true }));
          return jsonResponse({ items: [requestMessage], next_cursor: null });
        }
        return jsonResponse({ items: [requestMessage, answerMessage], next_cursor: null });
      }),
      http.get("*/v1/runs/:runId/spans", () => jsonResponse({ items: [], next_cursor: null })),
    );
    const client = renderPage();
    expect(await screen.findByText("Live updates")).toBeInTheDocument();
    await waitFor(() => expect(messageRequestCount).toBe(1));

    await client.invalidateQueries({ queryKey: ["runs", "run-1"] });

    expect(await screen.findByText("Final saved answer")).toBeInTheDocument();
    expect(messageRequestCount).toBe(2);
  });

  it("shows the observable decision, tool inputs, and observation in order", async () => {
    setAuthSession("token", { ...baseUser, role: "member" });
    server.use(
      http.get("*/v1/runs/:runId", () => jsonResponse({ ...run, iterations: 2 })),
      http.get("*/v1/sessions/:sessionId/messages", () => jsonResponse({ items: [
        { id: "request", session_id: "session-1", run_id: "run-1", seq: 1, role: "user", content: "Check Hue weather", tool_calls: [], tool_call_id: null, tool_name: null, is_error: false, created_at: run.created_at },
        { id: "decision", session_id: "session-1", run_id: "run-1", seq: 2, role: "assistant", content: "I will check the latest weather.", tool_calls: [{ id: "call-1", name: "weather", arguments: { city: "Hue" } }], tool_call_id: null, tool_name: null, is_error: false, created_at: run.created_at },
        { id: "observation", session_id: "session-1", run_id: "run-1", seq: 3, role: "tool", content: "Sunny", tool_calls: [], tool_call_id: "call-1", tool_name: "weather", is_error: false, created_at: run.created_at },
        { id: "answer", session_id: "session-1", run_id: "run-1", seq: 4, role: "assistant", content: "It is sunny.", tool_calls: [], tool_call_id: null, tool_name: null, is_error: false, created_at: run.created_at },
      ], next_cursor: null })),
      http.get("*/v1/runs/:runId/spans", () => jsonResponse({ items: [
        { id: "llm-1", run_id: "run-1", parent_span_id: null, kind: "llm_call", name: "llm.generate", status: "ok", model: "gemini", tool_name: null, usage: { input_tokens: 2, output_tokens: 1 }, started_at: run.started_at, ended_at: run.finished_at, duration_ms: 500, attributes: { iteration: 1, finish_reason: "STOP" }, error_message: null },
        { id: "tool-1", run_id: "run-1", parent_span_id: null, kind: "tool_call", name: "tool.execute", status: "ok", model: null, tool_name: "weather", usage: { input_tokens: null, output_tokens: null }, started_at: run.started_at, ended_at: run.finished_at, duration_ms: 100, attributes: { call_id: "call-1", result_truncated: false }, error_message: null },
        { id: "llm-2", run_id: "run-1", parent_span_id: null, kind: "llm_call", name: "llm.generate", status: "ok", model: "gemini", tool_name: null, usage: { input_tokens: 3, output_tokens: 2 }, started_at: run.started_at, ended_at: run.finished_at, duration_ms: 400, attributes: { iteration: 2, finish_reason: "stop" }, error_message: null },
      ], next_cursor: null })),
    );

    renderPage();

    expect(await screen.findByText("Gather information with tools")).toBeInTheDocument();
    expect(screen.getByText("Produce the answer")).toBeInTheDocument();
    expect(screen.getByText("I will check the latest weather.")).toBeInTheDocument();
    expect(screen.getAllByText("weather").length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole("button", { name: "View inputs and observation for weather" }));
    expect(await screen.findByText(/"city": "Hue"/)).toBeInTheDocument();
    expect(screen.getByText("Sunny")).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "View model signal" })[0]!);
    expect(screen.getByText(/STOP · tool call emitted/i)).toBeInTheDocument();
  });

  it("does not leave an unfinished tool action waiting after the run ends", async () => {
    setAuthSession("token", { ...baseUser, role: "member" });
    server.use(
      http.get("*/v1/runs/:runId", () => jsonResponse(run)),
      http.get("*/v1/sessions/:sessionId/messages", () => jsonResponse({ items: [
        { id: "request", session_id: "session-1", run_id: "run-1", seq: 1, role: "user", content: "Check it", tool_calls: [], tool_call_id: null, tool_name: null, is_error: false, created_at: run.created_at },
        { id: "decision", session_id: "session-1", run_id: "run-1", seq: 2, role: "assistant", content: "", tool_calls: [{ id: "call-1", name: "weather", arguments: {} }], tool_call_id: null, tool_name: null, is_error: false, created_at: run.created_at },
      ], next_cursor: null })),
      http.get("*/v1/runs/:runId/spans", () => jsonResponse({ items: [], next_cursor: null })),
    );

    renderPage();

    expect(await screen.findByText("No observation")).toBeInTheDocument();
    expect(screen.queryByText("Waiting")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "View inputs and observation for weather" }));
    expect(screen.getByText("No observation was saved before this activity ended.")).toBeInTheDocument();
  });
});
