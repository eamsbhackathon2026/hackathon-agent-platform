import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

import { clearAuthSession, setAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { ConversationDetailPage } from "./conversation-detail-page";

const user = { id: "user-1", email: "user@example.test", name: "User", role: "member" as const, status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" };
const session = { id: "session-1", agent_id: "agent-1", source: "playground" as const, created_by_user_id: "user-1", created_by_api_key_id: null, external_key: null, title: "Balance check", created_at: "2026-09-17T10:00:00Z", updated_at: "2026-09-17T10:05:00Z" };

function run(id: string, overrides: Record<string, unknown> = {}) {
  return {
    id, agent_id: "agent-1", session_id: "session-1", mode: "stream" as const, status: "succeeded" as const, source: "playground" as const,
    triggered_by_user_id: "user-1", triggered_by_api_key_id: null, input: { message: "Hi" }, output: "Done", error: null, iterations: 2,
    usage: { input_tokens: 10, output_tokens: 5 }, metadata: {}, cancel_requested_at: null, queued_at: null,
    started_at: "2026-09-17T10:00:01Z", finished_at: "2026-09-17T10:00:04Z", created_at: "2026-09-17T10:00:00Z", ...overrides,
  };
}

function message(seq: number, runId: string, role: "user" | "assistant" | "tool", content: string, createdAt: string, toolCalls: { id: string; name: string; arguments: Record<string, unknown> }[] = []) {
  return { id: `m-${seq}`, session_id: "session-1", run_id: runId, seq, role, content, tool_calls: toolCalls, tool_call_id: role === "tool" ? "call-1" : null, tool_name: role === "tool" ? "lookup" : null, is_error: false, created_at: createdAt };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter([{ path: "/conversations/:sessionId", element: <ConversationDetailPage /> }], { initialEntries: ["/conversations/session-1"] });
  render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>);
}

afterEach(clearAuthSession);

describe("ConversationDetailPage", () => {
  it("shows each turn with its timing, status and a link to the full activity", async () => {
    setAuthSession("token", user);
    const runQuery = vi.fn();
    const stepRequests = vi.fn();
    server.use(
      http.get("*/v1/sessions/:sessionId", () => jsonResponse(session)),
      http.get(apiUrl("/v1/runs"), ({ request }) => {
        runQuery(Object.fromEntries(new URL(request.url).searchParams));
        return jsonResponse({ items: [run("run-2", { status: "failed", output: null, error: { code: "loop_detected", message: "loop" }, usage: { input_tokens: null, output_tokens: null }, started_at: "2026-09-17T10:05:01Z", finished_at: "2026-09-17T10:05:02Z", created_at: "2026-09-17T10:05:00Z" }), run("run-1")], next_cursor: null });
      }),
      http.get("*/v1/sessions/:sessionId/messages", () => jsonResponse({ items: [
        message(1, "run-1", "user", "What is my balance?", "2026-09-17T10:00:00Z"),
        message(2, "run-1", "assistant", "", "2026-09-17T10:00:02Z", [{ id: "call-1", name: "lookup", arguments: {} }]),
        message(3, "run-1", "tool", "{\"balance\":10}", "2026-09-17T10:00:03Z"),
        message(4, "run-1", "assistant", "Your balance is **10**", "2026-09-17T10:00:04Z"),
        message(5, "run-2", "user", "And yesterday?", "2026-09-17T10:05:00Z"),
      ], next_cursor: null })),
      http.get("*/v1/runs/:runId/spans", () => { stepRequests(); return jsonResponse({ items: [], next_cursor: null }); }),
    );
    renderPage();

    const turns = await screen.findAllByRole("listitem", { name: /Turn \d/ });
    expect(turns).toHaveLength(2);
    expect(runQuery).toHaveBeenCalledWith({ limit: "100", session_id: "session-1" });
    const first = within(turns[0]!);
    expect(first.getByText("What is my balance?")).toBeInTheDocument();
    expect(first.getByText("10", { selector: "strong" })).toBeInTheDocument();
    expect(first.getByText(/responded in 4\.0 sec/)).toBeInTheDocument();
    expect(first.getByText("Completed")).toBeInTheDocument();
    expect(first.getByText("2 model passes · 15 processing units")).toBeInTheDocument();
    expect(first.getByRole("link", { name: /Open details/ })).toHaveAttribute("href", "/activity/run-1");
    expect(first.queryByText("Request")).not.toBeInTheDocument();

    const second = within(turns[1]!);
    expect(second.getByText("Failed")).toBeInTheDocument();
    expect(second.getByText("No answer was saved for this turn. It ended after 2.0 sec.")).toBeInTheDocument();
    expect(second.getByText("The assistant stopped after repeating the same action")).toBeInTheDocument();

    expect(screen.getByText("1 failed")).toBeInTheDocument();
    expect(screen.getByText("15")).toBeInTheDocument();
    expect(stepRequests).not.toHaveBeenCalled();

    fireEvent.click(first.getByRole("button", { name: "Agent loop" }));
    expect(await first.findByText("Request")).toBeInTheDocument();
    expect(first.getByText("Gather information with tools")).toBeInTheDocument();
    expect(stepRequests).toHaveBeenCalledTimes(1);
  });

  it("keeps the messages readable when turn statuses cannot load", async () => {
    setAuthSession("token", user);
    server.use(
      http.get("*/v1/sessions/:sessionId", () => jsonResponse(session)),
      http.get(apiUrl("/v1/runs"), () => jsonResponse({ type: "about:blank", title: "Lỗi", status: 500, detail: "", code: "internal", fields: [] }, 500)),
      http.get("*/v1/sessions/:sessionId/messages", () => jsonResponse({ items: [message(1, "run-1", "user", "Hello there", "2026-09-17T10:00:00Z")], next_cursor: null })),
    );
    renderPage();
    expect(await screen.findByText("Unable to load the status of each turn")).toBeInTheDocument();
    expect(await screen.findByText("Hello there")).toBeInTheDocument();
  });

  it("invites the owner to continue an empty conversation", async () => {
    setAuthSession("token", user);
    server.use(
      http.get("*/v1/sessions/:sessionId", () => jsonResponse(session)),
      http.get(apiUrl("/v1/runs"), () => jsonResponse({ items: [], next_cursor: null })),
      http.get("*/v1/sessions/:sessionId/messages", () => jsonResponse({ items: [], next_cursor: null })),
    );
    renderPage();
    expect(await screen.findByText(/No messages in this conversation yet\. Continue it in the Playground/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Continue conversation" })).toHaveAttribute("href", "/playground?agent=agent-1&session=session-1");
  });

  it("reads the answer once more when the last turn finishes", async () => {
    setAuthSession("token", user);
    let runCalls = 0;
    let finished = false;
    const question = message(1, "run-1", "user", "Still there?", "2026-09-17T10:00:00Z");
    server.use(
      http.get("*/v1/sessions/:sessionId", () => jsonResponse(session)),
      // The finished status arrives late, so a message poll starts before it and still misses the answer.
      http.get(apiUrl("/v1/runs"), async () => {
        runCalls += 1;
        if (runCalls === 1) return jsonResponse({ items: [run("run-1", { status: "running", output: null, finished_at: null })], next_cursor: null });
        await new Promise((resolve) => setTimeout(resolve, 150));
        finished = true;
        return jsonResponse({ items: [run("run-1")], next_cursor: null });
      }),
      http.get("*/v1/sessions/:sessionId/messages", async () => {
        const answered = finished;
        await new Promise((resolve) => setTimeout(resolve, 100));
        return jsonResponse({ items: answered ? [question, message(2, "run-1", "assistant", "Yes, done", "2026-09-17T10:00:03Z")] : [question], next_cursor: null });
      }),
    );
    renderPage();
    expect(await screen.findByText("Waiting for the answer…")).toBeInTheDocument();
    expect(await screen.findByText("Yes, done", {}, { timeout: 5_000 })).toBeInTheDocument();
    expect(screen.queryByText(/No answer was saved/)).not.toBeInTheDocument();
  }, 10_000);

  it("stops loading more turn statuses after a page fails", async () => {
    setAuthSession("token", user);
    const pageRequests = vi.fn();
    server.use(
      http.get("*/v1/sessions/:sessionId", () => jsonResponse(session)),
      http.get(apiUrl("/v1/runs"), ({ request }) => {
        if (!new URL(request.url).searchParams.get("cursor")) return jsonResponse({ items: [run("run-1")], next_cursor: "next" });
        pageRequests();
        return jsonResponse({ type: "about:blank", title: "Lỗi", status: 400, detail: "", code: "validation_failed", fields: [] }, 400);
      }),
      http.get("*/v1/sessions/:sessionId/messages", () => jsonResponse({ items: [], next_cursor: null })),
    );
    renderPage();
    expect(await screen.findByText("Unable to load the status of each turn")).toBeInTheDocument();
    await new Promise((resolve) => setTimeout(resolve, 200));
    expect(pageRequests).toHaveBeenCalledTimes(1);
  });
});
