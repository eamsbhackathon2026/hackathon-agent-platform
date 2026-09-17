import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { HttpResponse, http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { conversationKeys } from "@/entities/conversation";
import { clearAuthSession, setAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";

import { PlaygroundPage } from "./playground-page";

const agent = {
  id: "agent-1", name: "Test assistant", description: "", provider_id: "provider-1", model: "model",
  system_prompt: "", temperature: null, max_output_tokens: null, max_iterations: 8, timeout_seconds: 120,
  created_by: "user-1", archived_at: null, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z",
  ready: true, readiness_error: null,
};

const user = {
  id: "user-1", email: "user@example.test", name: "User", role: "admin" as const,
  status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z",
};

function conversation(id: string, agentId = agent.id, source: "playground" | "api" = "playground", ownerId: string | null = user.id, title = "Conversation") {
  return { id, agent_id: agentId, source, created_by_user_id: ownerId, created_by_api_key_id: source === "api" ? "key-1" : null, external_key: null, title, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T01:00:00Z" };
}

function renderPage(entry: string | string[] = "/playground?agent=agent-1", initialIndex?: number) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const entries = typeof entry === "string" ? [entry] : entry;
  const router = createMemoryRouter([
    { path: "/playground", element: <PlaygroundPage /> },
    { path: "/activity", element: <p>Activity page</p> },
  ], { initialEntries: entries, ...(initialIndex === undefined ? {} : { initialIndex }) });
  render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>);
  return { client, router };
}

const emptyHistory = http.get("*/v1/sessions/:sessionId/messages", () => jsonResponse({ items: [], next_cursor: null }));

function sseResponse(...events: object[]) {
  const encoder = new TextEncoder();
  return new HttpResponse(new ReadableStream({
    start(controller) {
      for (const event of events) controller.enqueue(encoder.encode(`event: ${(event as { type: string }).type}\ndata: ${JSON.stringify(event)}\n\n`));
      controller.close();
    },
  }), { headers: { "Content-Type": "text/event-stream" } });
}

function rawSseResponse(data: string) {
  return new HttpResponse(`data: ${data}\n\n`, { headers: { "Content-Type": "text/event-stream" } });
}

beforeEach(() => {
  setAuthSession("token", user);
  server.use(
    http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [], next_cursor: null })),
    http.get("*/v1/sessions/:sessionId", ({ params }) => jsonResponse(conversation(String(params.sessionId)))),
  );
});

afterEach(() => clearAuthSession());

describe("PlaygroundPage", () => {
  it("renders streamed text and links to processing details", async () => {
    const receivedBody = vi.fn();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      emptyHistory,
      http.post("*/v1/agents/:agentId/runs/stream", async ({ request }) => {
        receivedBody(await request.json());
        return sseResponse(
          { type: "run.started", run_id: "run-1", session_id: "session-1" },
          { type: "message.delta", text: "Hello" },
          { type: "run.completed", run: { output: "Hello there" } },
        );
      }),
    );
    const { client, router } = renderPage();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByText("Hello there")).toBeInTheDocument();
    expect(receivedBody).toHaveBeenCalledWith({ input: { message: "Hello" } });
    await waitFor(() => expect(router.state.location.search).toBe("?agent=agent-1&session=session-1"));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: conversationKeys.all });
    expect(screen.getByRole("link", { name: "View processing steps" })).toHaveAttribute("href", "/activity/run-1");
  });

  it("continues subsequent messages in the session returned by the stream", async () => {
    const bodies: unknown[] = [];
    let requestCount = 0;
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      emptyHistory,
      http.post("*/v1/agents/:agentId/runs/stream", async ({ request }) => {
        bodies.push(await request.json());
        requestCount += 1;
        return sseResponse(
          { type: "run.started", run_id: `run-${requestCount}`, session_id: "session-kept" },
          { type: "run.completed", run: { output: `Reply ${requestCount}` } },
        );
      }),
    );
    const { router } = renderPage();
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "First question" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await screen.findByText("Reply 1");
    await waitFor(() => expect(router.state.location.search).toBe("?agent=agent-1&session=session-kept"));
    await waitFor(() => expect(screen.getByLabelText("Message")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Follow-up question" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await screen.findByText("Reply 2");
    expect(bodies).toEqual([
      { input: { message: "First question" } },
      { input: { message: "Follow-up question" }, session_id: "session-kept" },
    ]);
  });

  it("offers the persisted result when a stream disconnects early", async () => {
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      emptyHistory,
      http.post("*/v1/agents/:agentId/runs/stream", () => sseResponse(
        { type: "run.started", run_id: "run-lost", session_id: "session-1" },
        { type: "message.delta", text: "Partial response" },
      )),
    );
    renderPage();
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByRole("link", { name: "view result" })).toHaveAttribute("href", "/activity/run-lost");
  });

  it("shows a useful Problem action even when streaming never starts", async () => {
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      http.post("*/v1/agents/:agentId/runs/stream", () => jsonResponse({ type: "about:blank", title: "Missing connection", status: 400, detail: "The assistant has no AI connection.", code: "provider_not_configured", fields: [] }, 400)),
    );
    renderPage();
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByText("The assistant has no AI connection.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Add connection" })).toHaveAttribute("href", "/connections");
    expect(screen.queryByRole("link", { name: "View processing steps" })).not.toBeInTheDocument();
  });

  it("offers login when refresh cannot recover an expired session", async () => {
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      http.post("*/v1/agents/:agentId/runs/stream", () => jsonResponse({ type: "about:blank", title: "Session expired", status: 401, detail: "Sign in again to continue.", code: "unauthenticated", fields: [] }, 401)),
      http.post(apiUrl("/v1/auth/refresh"), () => HttpResponse.json({}, { status: 401 })),
    );
    renderPage();
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByText("Sign in again to continue.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
  });

  it.each([
    ["network rejection", () => HttpResponse.error()],
    ["missing response body", () => new HttpResponse(null, { status: 200, headers: { "Content-Type": "text/event-stream" } })],
    ["malformed first event", () => rawSseResponse("not-json")],
  ])("turns %s before run.started into a retryable request error", async (_name, response) => {
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      http.post("*/v1/agents/:agentId/runs/stream", response),
    );
    renderPage();
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByText("Unable to start processing. Check the connection and try again.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument();
    expect(screen.queryByText("Connection lost")).not.toBeInTheDocument();
  });

  it("does not duplicate optimistic or completed messages after history refetch", async () => {
    const storedMessages = [
      { id: "message-user", session_id: "session-dedup", run_id: "run-dedup", seq: 1, role: "user", content: "Hello", tool_calls: [], tool_call_id: null, tool_name: null, is_error: false, created_at: new Date().toISOString() },
      { id: "message-assistant", session_id: "session-dedup", run_id: "run-dedup", seq: 2, role: "assistant", content: "A single reply", tool_calls: [], tool_call_id: null, tool_name: null, is_error: false, created_at: new Date().toISOString() },
    ];
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      http.get("*/v1/sessions/:sessionId/messages", () => jsonResponse({ items: storedMessages, next_cursor: null })),
      http.post("*/v1/agents/:agentId/runs/stream", () => sseResponse(
        { type: "run.started", run_id: "run-dedup", session_id: "session-dedup" },
        { type: "run.completed", run: { output: "A single reply" } },
      )),
    );
    renderPage();
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await screen.findByText("A single reply");
    expect(screen.getAllByText("Hello")).toHaveLength(1);
    expect(screen.getAllByText("A single reply")).toHaveLength(1);
  });

  it("drops the old session when selecting another assistant", async () => {
    Object.defineProperty(HTMLElement.prototype, "scrollIntoView", { configurable: true, value: vi.fn() });
    const secondAgent = { ...agent, id: "agent-2", name: "Second assistant" };
    const receivedBody = vi.fn();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent, secondAgent], next_cursor: null })),
      http.get("*/v1/sessions/:sessionId/messages", () => jsonResponse({ items: [{ id: "old", session_id: "old-session", run_id: null, seq: 1, role: "user", content: "Old message", tool_calls: [], tool_call_id: null, tool_name: null, is_error: false, created_at: "2026-09-15T00:00:00Z" }], next_cursor: null })),
      http.post("*/v1/agents/:agentId/runs/stream", async ({ request }) => {
        receivedBody(await request.json());
        return sseResponse({ type: "run.started", run_id: "run-new", session_id: "new-session" }, { type: "run.completed", run: { output: "New reply" } });
      }),
    );
    renderPage("/playground?agent=agent-1&session=old-session");
    await screen.findByText("Old message");
    const picker = screen.getByRole("combobox", { name: "Choose assistant" });
    fireEvent.keyDown(picker, { key: "ArrowDown" });
    fireEvent.click(await screen.findByRole("option", { name: "Second assistant" }));
    expect(screen.queryByText("Old message")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Start again" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await screen.findByText("New reply");
    expect(receivedBody).toHaveBeenCalledWith({ input: { message: "Start again" } });
  });

  it("aborts the stream and requests cancellation after receiving an identifier", async () => {
    const cancelled = vi.fn();
    const encoder = new TextEncoder();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      emptyHistory,
      http.post("*/v1/agents/:agentId/runs/stream", () => new HttpResponse(new ReadableStream({
        start(controller) {
          controller.enqueue(encoder.encode(`data: ${JSON.stringify({ type: "run.started", run_id: "run-stop", session_id: "session-1" })}\n\n`));
          controller.enqueue(encoder.encode(`data: ${JSON.stringify({ type: "message.delta", text: "Answering" })}\n\n`));
        },
      }), { headers: { "Content-Type": "text/event-stream" } })),
      http.post("*/v1/runs/:runId/cancel", ({ params }) => {
        cancelled(params.runId);
        return jsonResponse({ id: params.runId, status: "cancelled" });
      }),
    );
    renderPage();
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Please stop" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await screen.findByText("Answering");
    fireEvent.click(screen.getByRole("button", { name: "Stop" }));
    await waitFor(() => expect(cancelled).toHaveBeenCalledTimes(1));
    expect(cancelled).toHaveBeenCalledWith("run-stop");
  });

  it("normalizes a direct session URL to the persisted assistant", async () => {
    const secondAgent = { ...agent, id: "agent-2", name: "Second assistant" };
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent, secondAgent], next_cursor: null })),
      http.get("*/v1/sessions/:sessionId", () => jsonResponse(conversation("session-1", secondAgent.id))),
      emptyHistory,
    );

    const { router } = renderPage("/playground?agent=wrong-agent&session=session-1");

    await waitFor(() => expect(router.state.location.search).toBe("?agent=agent-2&session=session-1"));
    expect(screen.getByRole("combobox", { name: "Choose assistant" })).toHaveTextContent("Second assistant");
    expect(screen.getByLabelText("Message")).toBeEnabled();
  });

  it.each([
    ["an API conversation", conversation("session-1", agent.id, "api", null)],
    ["another user's conversation", conversation("session-1", agent.id, "playground", "user-2")],
  ])("prevents continuing %s in Playground", async (_name, session) => {
    const streamed = vi.fn();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      http.get("*/v1/sessions/:sessionId", () => jsonResponse(session)),
      emptyHistory,
      http.post("*/v1/agents/:agentId/runs/stream", () => { streamed(); return sseResponse(); }),
    );
    renderPage("/playground?agent=agent-1&session=session-1");

    expect(await screen.findByText("This conversation can’t be continued in Playground")).toBeInTheDocument();
    expect(screen.getByLabelText("Message")).toBeDisabled();
    expect(screen.getByRole("link", { name: "View conversation history" })).toHaveAttribute("href", "/conversations/session-1");
    expect(streamed).not.toHaveBeenCalled();
  });

  it("switches and clears conversations through the rail while idle", async () => {
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [conversation("session-2")], next_cursor: null })),
      http.get("*/v1/sessions/:sessionId/messages", ({ params }) => jsonResponse({ items: [{ id: `message-${String(params.sessionId)}`, session_id: String(params.sessionId), run_id: null, seq: 1, role: "user", content: String(params.sessionId), tool_calls: [], tool_call_id: null, tool_name: null, is_error: false, created_at: "2026-09-15T00:00:00Z" }], next_cursor: null })),
    );
    const { router } = renderPage("/playground?agent=agent-1&session=session-1");
    expect(await screen.findByText("session-1")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Open recent chats" }));
    fireEvent.click(await screen.findByRole("button", { name: "Conversation, Test assistant" }));
    await waitFor(() => expect(router.state.location.search).toBe("?agent=agent-1&session=session-2"));
    expect(await screen.findByText("session-2")).toBeInTheDocument();
    expect(screen.queryByText("session-1")).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Discard this draft" } });
    fireEvent.click(screen.getByRole("button", { name: "Open recent chats" }));
    fireEvent.click(await screen.findByRole("button", { name: "New chat" }));
    await waitFor(() => expect(router.state.location.search).toBe("?agent=agent-1"));
    expect(screen.queryByText("session-2")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Message")).toHaveValue("");
  });

  it("follows browser back and forward while idle", async () => {
    const secondAgent = { ...agent, id: "agent-2", name: "Second assistant" };
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent, secondAgent], next_cursor: null })),
      http.get("*/v1/sessions/:sessionId", ({ params }) => jsonResponse(conversation(String(params.sessionId), params.sessionId === "session-2" ? secondAgent.id : agent.id))),
      http.get("*/v1/sessions/:sessionId/messages", ({ params }) => jsonResponse({ items: [{ id: `message-${String(params.sessionId)}`, session_id: String(params.sessionId), run_id: null, seq: 1, role: "user", content: String(params.sessionId), tool_calls: [], tool_call_id: null, tool_name: null, is_error: false, created_at: "2026-09-15T00:00:00Z" }], next_cursor: null })),
    );
    const entries = ["/playground?agent=agent-1&session=session-1", "/playground?agent=agent-2&session=session-2"];
    const { router } = renderPage(entries, 1);
    expect(await screen.findByText("session-2")).toBeInTheDocument();

    await router.navigate(-1);
    expect(await screen.findByText("session-1")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Choose assistant" })).toHaveTextContent("Test assistant");
    await router.navigate(1);
    expect(await screen.findByText("session-2")).toBeInTheDocument();
  });

  it("keeps the stream when a context switch is dismissed", async () => {
    const encoder = new TextEncoder();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [conversation("session-live", agent.id, "playground", user.id, "Live conversation"), conversation("session-2", agent.id, "playground", user.id, "Other conversation")], next_cursor: null })),
      emptyHistory,
      http.post("*/v1/agents/:agentId/runs/stream", () => new HttpResponse(new ReadableStream({ start(controller) {
        controller.enqueue(encoder.encode(`data: ${JSON.stringify({ type: "run.started", run_id: "run-live", session_id: "session-live" })}\n\n`));
        controller.enqueue(encoder.encode(`data: ${JSON.stringify({ type: "message.delta", text: "Still answering" })}\n\n`));
      } }), { headers: { "Content-Type": "text/event-stream" } })),
    );
    const { router } = renderPage();
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByText("Still answering")).toBeInTheDocument();
    await waitFor(() => expect(router.state.location.search).toBe("?agent=agent-1&session=session-live"));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Open recent chats" }));
    const liveItem = (await screen.findByRole("button", { name: /^Live conversation,/ })).closest("li");
    fireEvent.contextMenu(liveItem as HTMLElement);
    expect(await screen.findByRole("menuitem", { name: "Delete Live conversation" })).toHaveAttribute("data-disabled");
    fireEvent.keyDown(document, { key: "Escape" });
    fireEvent.click(await screen.findByRole("button", { name: "New chat" }));
    expect(await screen.findByRole("alertdialog", { name: "Stop the current response?" })).toBeInTheDocument();
    expect(router.state.location.search).toBe("?agent=agent-1&session=session-live");
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.getByRole("alertdialog", { name: "Stop the current response?" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Keep waiting" }));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("button", { name: "Open recent chats" })).toHaveFocus());
    expect(screen.getByRole("button", { name: "Stop" })).toBeInTheDocument();
    expect(screen.getByText("Still answering")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Open recent chats" }));
    fireEvent.click(await screen.findByRole("button", { name: "Other conversation, Test assistant" }));
    expect(await screen.findByRole("alertdialog", { name: "Stop the current response?" })).toBeInTheDocument();
    expect(router.state.location.search).toBe("?agent=agent-1&session=session-live");
    fireEvent.click(screen.getByRole("button", { name: "Keep waiting" }));
  });

  it("cancels the active run before applying a confirmed switch", async () => {
    const encoder = new TextEncoder();
    const cancelled = vi.fn();
    let finishCancellation!: () => void;
    const cancellationGate = new Promise<void>((resolve) => { finishCancellation = resolve; });
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [], next_cursor: null })),
      emptyHistory,
      http.post("*/v1/agents/:agentId/runs/stream", () => new HttpResponse(new ReadableStream({ start(controller) {
        controller.enqueue(encoder.encode(`data: ${JSON.stringify({ type: "run.started", run_id: "run-live", session_id: "session-live" })}\n\n`));
      } }), { headers: { "Content-Type": "text/event-stream" } })),
      http.post("*/v1/runs/:runId/cancel", async ({ params }) => {
        cancelled(params.runId);
        await cancellationGate;
        return jsonResponse({ id: String(params.runId), status: "cancelled" });
      }),
    );
    const { router } = renderPage();
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(router.state.location.search).toContain("session=session-live"));
    fireEvent.click(screen.getByRole("button", { name: "Open recent chats" }));
    fireEvent.click(await screen.findByRole("button", { name: "New chat" }));
    fireEvent.click(await screen.findByRole("button", { name: "Stop and switch" }));

    await waitFor(() => expect(cancelled).toHaveBeenCalledWith("run-live"));
    expect(router.state.location.search).toBe("?agent=agent-1&session=session-live");
    finishCancellation();
    await waitFor(() => expect(router.state.location.search).toBe("?agent=agent-1"));
    await waitFor(() => expect(screen.getByRole("button", { name: "Open recent chats" })).toHaveFocus());
  });

  it("switches without a cancel request when no run identifier was received", async () => {
    const secondAgent = { ...agent, id: "agent-2", name: "Second assistant" };
    const cancelled = vi.fn();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent, secondAgent], next_cursor: null })),
      http.post("*/v1/agents/:agentId/runs/stream", () => new HttpResponse(new ReadableStream({ start() {} }), { headers: { "Content-Type": "text/event-stream" } })),
      http.post("*/v1/runs/:runId/cancel", () => { cancelled(); return jsonResponse({ id: "run", status: "cancelled" }); }),
    );
    const { router } = renderPage();
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByRole("button", { name: "Stop" })).toBeInTheDocument();
    const picker = screen.getByRole("combobox", { name: "Choose assistant" });
    fireEvent.keyDown(picker, { key: "ArrowDown" });
    fireEvent.click(await screen.findByRole("option", { name: "Second assistant" }));
    fireEvent.click(await screen.findByRole("button", { name: "Stop and switch" }));

    await waitFor(() => expect(router.state.location.search).toBe("?agent=agent-2"));
    expect(cancelled).not.toHaveBeenCalled();
  });

  it("blocks browser navigation while streaming and proceeds after confirmation", async () => {
    const encoder = new TextEncoder();
    const cancelled = vi.fn();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      emptyHistory,
      http.post("*/v1/agents/:agentId/runs/stream", () => new HttpResponse(new ReadableStream({ start(controller) {
        controller.enqueue(encoder.encode(`data: ${JSON.stringify({ type: "run.started", run_id: "run-live", session_id: "session-current" })}\n\n`));
        controller.enqueue(encoder.encode(`data: ${JSON.stringify({ type: "message.delta", text: "Working" })}\n\n`));
      } }), { headers: { "Content-Type": "text/event-stream" } })),
      http.post("*/v1/runs/:runId/cancel", ({ params }) => { cancelled(params.runId); return jsonResponse({ id: String(params.runId), status: "cancelled" }); }),
    );
    const { router } = renderPage(["/activity", "/playground?agent=agent-1&session=session-current"], 1);
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByText("Working")).toBeInTheDocument();

    void router.navigate(-1);
    expect(await screen.findByRole("alertdialog", { name: "Stop the current response?" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/playground");
    fireEvent.click(screen.getByRole("button", { name: "Stop and switch" }));
    await waitFor(() => expect(router.state.location.pathname).toBe("/activity"));
    expect(cancelled).toHaveBeenCalledWith("run-live");
  });

  it("switches after a failed cancellation and warns that the activity may continue", async () => {
    const encoder = new TextEncoder();
    const secondAgent = { ...agent, id: "agent-2", name: "Second assistant" };
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent, secondAgent], next_cursor: null })),
      emptyHistory,
      http.post("*/v1/agents/:agentId/runs/stream", () => new HttpResponse(new ReadableStream({ start(controller) {
        controller.enqueue(encoder.encode(`data: ${JSON.stringify({ type: "run.started", run_id: "run-live", session_id: "session-live" })}\n\n`));
      } }), { headers: { "Content-Type": "text/event-stream" } })),
      http.post("*/v1/runs/:runId/cancel", () => jsonResponse({ type: "about:blank", title: "Error", status: 500, detail: "", code: "internal", fields: [] }, 500)),
    );
    const { router } = renderPage();
    await screen.findByText("Test assistant");
    fireEvent.change(screen.getByLabelText("Message"), { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(router.state.location.search).toContain("session=session-live"));
    const picker = screen.getByRole("combobox", { name: "Choose assistant" });
    fireEvent.keyDown(picker, { key: "ArrowDown" });
    fireEvent.click(await screen.findByRole("option", { name: "Second assistant" }));
    fireEvent.click(await screen.findByRole("button", { name: "Stop and switch" }));

    await waitFor(() => expect(router.state.location.search).toBe("?agent=agent-2"));
    expect(await screen.findByText("The activity may still be running because cancellation could not be confirmed.")).toBeInTheDocument();
  });

  it("clears an active deleted session without changing the assistant", async () => {
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [agent], next_cursor: null })),
      http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [conversation("session-1")], next_cursor: null })),
      emptyHistory,
      http.delete("*/v1/sessions/:sessionId", () => new HttpResponse(null, { status: 204 })),
    );
    const { router } = renderPage("/playground?agent=agent-1&session=session-1");
    await screen.findByText("Test assistant");
    fireEvent.click(screen.getByRole("button", { name: "Open recent chats" }));
    const item = (await screen.findByRole("button", { name: /^Conversation,/ })).closest("li");
    fireEvent.contextMenu(item as HTMLElement);
    fireEvent.click(await screen.findByRole("menuitem", { name: "Delete Conversation" }));
    fireEvent.click(await screen.findByRole("button", { name: "Delete conversation" }));

    await waitFor(() => expect(router.state.location.search).toBe("?agent=agent-1"));
  });
});
