import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { delay, http } from "msw";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

import { clearAuthSession, setAuthSession } from "@/shared/api";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";

import { ConversationRail } from "./conversation-rail";

const user = { id: "user-1", email: "user@example.test", name: "User", role: "admin" as const, status: "active" as const, must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" };
const agent = { id: "agent-1", name: "Support assistant", description: "", provider_id: "provider-1", model: "test-model", system_prompt: "", temperature: 0.5, max_output_tokens: 2048, max_iterations: 8, timeout_seconds: 120, status: "active" as const, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z", readiness: "ready" as const, readiness_error: null };

function conversation(id: string, title: string, agentId = agent.id) {
  return { id, agent_id: agentId, source: "playground" as const, created_by_user_id: user.id, created_by_api_key_id: null, external_key: null, title, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T01:00:00Z" };
}

function renderRail(overrides: Partial<React.ComponentProps<typeof ConversationRail>> = {}) {
  const props = { activeSessionId: "session-1", disabled: false, onNew: vi.fn(), onSelect: vi.fn(), onActiveDeleted: vi.fn(), ...overrides };
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter><ConversationRail {...props} /></MemoryRouter></QueryClientProvider>);
  return { client, props };
}

function rightClickItem(title: string) {
  const item = screen.getByRole("button", { name: new RegExp(`^${title},`) }).closest("li");
  fireEvent.contextMenu(item as HTMLElement);
}

async function openItemMenu(title: string) {
  rightClickItem(title);
  return await screen.findByRole("menuitem", { name: `Delete ${title}` });
}

function useAgentHandler(items = [agent]) {
  server.use(http.get(apiUrl("/v1/agents"), () => jsonResponse({ items, next_cursor: null })));
}

async function openRail() {
  fireEvent.click(screen.getByRole("button", { name: "Open recent chats" }));
  expect(await screen.findByRole("dialog", { name: "Recent conversations" })).toHaveClass("overscroll-contain");
}

afterEach(() => { clearAuthSession(); vi.unstubAllGlobals(); });

describe("ConversationRail", () => {
  it("appends cursor pages, marks the active chat, and closes after selection", async () => {
    setAuthSession("token", user);
    useAgentHandler();
    const requests = vi.fn();
    server.use(http.get(apiUrl("/v1/sessions"), ({ request }) => {
      const query = Object.fromEntries(new URL(request.url).searchParams);
      requests(query);
      return query.cursor === "next"
        ? jsonResponse({ items: [conversation("session-2", "Second chat")], next_cursor: null })
        : jsonResponse({ items: [conversation("session-1", "First chat")], next_cursor: "next" });
    }));
    const onSelect = vi.fn();
    renderRail({ onSelect });

    await openRail();
    const active = await screen.findByRole("button", { current: "page" });
    expect(active).toHaveAttribute("aria-current", "page");
    expect(requests).toHaveBeenCalledWith({ limit: "25", scope: "mine", source: "playground", sort: "updated_at" });
    expect(screen.getByRole("link", { name: "View workspace conversations" })).toHaveAttribute("href", "/conversations");

    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(await screen.findByText("Second chat")).toBeInTheDocument();
    expect(screen.getByText("First chat")).toBeInTheDocument();
    expect(requests).toHaveBeenLastCalledWith({ limit: "25", cursor: "next", scope: "mine", source: "playground", sort: "updated_at" });

    fireEvent.click(screen.getByRole("button", { name: "Second chat, Support assistant" }));
    expect(onSelect).toHaveBeenCalledWith({ agentId: "agent-1", sessionId: "session-2" });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("uses readable fallbacks for members and closes after New chat", async () => {
    setAuthSession("token", { ...user, role: "member" });
    useAgentHandler([]);
    server.use(http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [conversation("session-1", "", "archived-agent")], next_cursor: null })));
    const onNew = vi.fn();
    renderRail({ onNew });

    await openRail();
    expect(await screen.findByText("Untitled conversation")).toBeInTheDocument();
    expect(screen.getByText("Archived assistant")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "View workspace conversations" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "New chat" }));
    expect(onNew).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("shows the workspace history link for owners", async () => {
    setAuthSession("token", { ...user, role: "owner" });
    useAgentHandler();
    server.use(http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [], next_cursor: null })));
    renderRail();

    await openRail();
    expect(await screen.findByRole("link", { name: "View workspace conversations" })).toHaveAttribute("href", "/conversations");
  });

  it("blocks context-changing actions while a response is streaming", async () => {
    setAuthSession("token", user);
    useAgentHandler();
    server.use(http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [conversation("session-1", "First chat")], next_cursor: null })));
    const onNew = vi.fn();
    const onSelect = vi.fn();
    renderRail({ disabled: true, onNew, onSelect });

    await openRail();
    expect(await screen.findByText(/Stop or wait for the response/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "New chat" })).toBeDisabled();
    expect(screen.getByRole("button", { current: "page" })).toBeDisabled();
    rightClickItem("First chat");
    const deleteItem = await screen.findByRole("menuitem", { name: "Delete First chat" });
    expect(deleteItem).toHaveAttribute("data-disabled");
    expect(await screen.findByText("Wait for the response to finish.")).toBeInTheDocument();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.getByRole("link", { name: "View workspace conversations" })).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(screen.getByRole("button", { current: "page" }));
    expect(onNew).not.toHaveBeenCalled();
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("distinguishes loading, empty, and retryable error states", async () => {
    setAuthSession("token", user);
    useAgentHandler();
    let fail = true;
    server.use(http.get(apiUrl("/v1/sessions"), async () => {
      if (fail) {
        await delay(20);
        return jsonResponse({ type: "about:blank", title: "Error", status: 500, detail: "", code: "internal", fields: [] }, 500);
      }
      return jsonResponse({ items: [], next_cursor: null });
    }));
    renderRail();

    await openRail();
    expect(screen.getByRole("status", { name: "Loading recent conversations" })).toBeInTheDocument();
    expect(await screen.findByText("Unable to load recent conversations")).toBeInTheDocument();
    fail = false;
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("No conversations yet")).toBeInTheDocument();
  });

  it("calls the active deletion callback only for the active item", async () => {
    setAuthSession("token", user);
    useAgentHandler();
    server.use(
      http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [conversation("session-1", "First chat"), conversation("session-2", "Second chat")], next_cursor: null })),
      http.delete("*/v1/sessions/:sessionId", () => new Response(null, { status: 204 })),
    );
    const onActiveDeleted = vi.fn();
    const { props } = renderRail({ onActiveDeleted });

    await openRail();
    await screen.findByText("First chat");
    fireEvent.click(await openItemMenu("Second chat"));
    fireEvent.click(await screen.findByRole("button", { name: "Delete conversation" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(onActiveDeleted).not.toHaveBeenCalled();
    expect(props.onSelect).not.toHaveBeenCalled();

    fireEvent.click(await openItemMenu("First chat"));
    fireEvent.click(await screen.findByRole("button", { name: "Delete conversation" }));
    await waitFor(() => expect(onActiveDeleted).toHaveBeenCalledTimes(1));
  });

  it("mounts one desktop content tree and sends one recent request", async () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true, media: "(min-width: 1280px)", onchange: null, addEventListener: vi.fn(), removeEventListener: vi.fn(), addListener: vi.fn(), removeListener: vi.fn(), dispatchEvent: vi.fn() })));
    setAuthSession("token", user);
    useAgentHandler();
    const requested = vi.fn();
    server.use(http.get(apiUrl("/v1/sessions"), () => { requested(); return jsonResponse({ items: [conversation("session-1", "First chat")], next_cursor: null }); }));

    renderRail();

    expect(await screen.findByRole("complementary", { name: "Recent conversations" })).toHaveClass("w-68");
    expect(await screen.findByText("First chat")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Open recent chats" })).not.toBeInTheDocument();
    expect(requested).toHaveBeenCalledTimes(1);
  });

  it("resets an open sheet after crossing the desktop breakpoint", async () => {
    let desktop = false;
    const listeners = new Set<() => void>();
    vi.stubGlobal("matchMedia", vi.fn(() => ({
      get matches() { return desktop; },
      media: "(min-width: 1280px)", onchange: null,
      addEventListener: vi.fn((_event: string, listener: () => void) => listeners.add(listener)),
      removeEventListener: vi.fn((_event: string, listener: () => void) => listeners.delete(listener)),
      addListener: vi.fn(), removeListener: vi.fn(), dispatchEvent: vi.fn(),
    })));
    setAuthSession("token", user);
    useAgentHandler();
    server.use(http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [conversation("session-1", "First chat")], next_cursor: null })));
    renderRail();

    await openRail();
    act(() => { desktop = true; listeners.forEach((listener) => listener()); });
    expect(await screen.findByRole("complementary", { name: "Recent conversations" })).toBeInTheDocument();
    expect(screen.queryByRole("dialog", { name: "Recent conversations" })).not.toBeInTheDocument();

    act(() => { desktop = false; listeners.forEach((listener) => listener()); });
    expect(await screen.findByRole("button", { name: "Open recent chats" })).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("dialog", { name: "Recent conversations" })).not.toBeInTheDocument();
  });
});
