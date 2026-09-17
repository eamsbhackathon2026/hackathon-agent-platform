import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

import type { Conversation } from "@/entities/conversation";
import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { ConversationsPage } from "./conversations-page";

function conversation(id: string, title: string, summary?: Conversation["summary"]) {
  return { id, agent_id: "agent-1", source: "playground" as const, created_by_user_id: "user-1", created_by_api_key_id: null, external_key: null, title, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z", ...(summary ? { summary } : {}) };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter><ConversationsPage /></MemoryRouter></QueryClientProvider>);
}

describe("ConversationsPage", () => {
  it("appends the next cursor page instead of replacing the first page", async () => {
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })),
      http.get(apiUrl("/v1/sessions"), ({ request }) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        return cursor === "next" ? jsonResponse({ items: [conversation("2", "Trang hai")], next_cursor: null }) : jsonResponse({ items: [conversation("1", "Trang một")], next_cursor: "next" });
      }),
    );
    renderPage();
    await screen.findByText("Trang một");
    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(await screen.findByText("Trang hai")).toBeInTheDocument();
    expect(screen.getByText("Trang một")).toBeInTheDocument();
  });

  it("asks for the most recently updated conversations first", async () => {
    const requests = vi.fn();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })),
      http.get(apiUrl("/v1/sessions"), ({ request }) => {
        requests(Object.fromEntries(new URL(request.url).searchParams));
        return jsonResponse({ items: [conversation("1", "Trang một")], next_cursor: null });
      }),
    );
    renderPage();
    await screen.findByText("Trang một");
    expect(requests).toHaveBeenCalledWith({ limit: "25", sort: "updated_at" });
  });

  it("distinguishes query failure from an empty result", async () => {
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })),
      http.get(apiUrl("/v1/sessions"), () => jsonResponse({ type: "about:blank", title: "Lỗi", status: 500, detail: "", code: "internal", fields: [] }, 500)),
    );
    renderPage();
    expect(await screen.findByText("Something went wrong")).toBeInTheDocument();
    expect(screen.queryByText("No conversations match. Try chatting with an assistant.")).not.toBeInTheDocument();
  });

  it("summarizes each thread so a troubled one stands out before opening it", async () => {
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })),
      http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [conversation("1", "", {
        turn_count: 6, failed_turn_count: 2, latest_run_id: "run-6", latest_run_status: "failed",
        usage: { input_tokens: 1000, output_tokens: null }, processing_ms: 4200,
        first_message: "Kiểm tra số dư", last_message: "Không tìm thấy tài khoản", last_message_role: "assistant",
      })], next_cursor: null })),
    );
    renderPage();
    expect(await screen.findByRole("link", { name: "Kiểm tra số dư" })).toHaveAttribute("href", "/conversations/1");
    expect(screen.getByText("Không tìm thấy tài khoản")).toBeInTheDocument();
    expect(screen.getByText("6 turns")).toBeInTheDocument();
    expect(screen.getByText("2 failed")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
    expect(screen.getByText("1,000 processing units")).toBeInTheDocument();
    expect(screen.getByText("Processing 4.2 sec")).toBeInTheDocument();
  });

  it("filters by latest activity time and can clear the filters", async () => {
    const requests = vi.fn();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })),
      http.get(apiUrl("/v1/sessions"), ({ request }) => {
        const params = Object.fromEntries(new URL(request.url).searchParams);
        requests(params);
        return jsonResponse({ items: params.from ? [] : [conversation("1", "Trang một")], next_cursor: null });
      }),
    );
    renderPage();
    await screen.findByText("Trang một");
    fireEvent.change(screen.getByLabelText("Active from"), { target: { value: "2026-09-16T08:30" } });
    expect(await screen.findByText("No conversations match these filters.")).toBeInTheDocument();
    expect(requests).toHaveBeenLastCalledWith({ limit: "25", sort: "updated_at", from: new Date("2026-09-16T08:30").toISOString() });
    fireEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(await screen.findByText("Trang một")).toBeInTheDocument();
    expect(screen.getByLabelText("Active from")).toHaveValue("");
  });

  it("marks a thread without requests instead of leaving the status blank", async () => {
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })),
      http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [conversation("1", "New chat", { turn_count: 0, failed_turn_count: 0, latest_run_id: null, latest_run_status: null, usage: { input_tokens: null, output_tokens: null }, processing_ms: 0, first_message: null, last_message: null, last_message_role: null })], next_cursor: null })),
    );
    renderPage();
    expect(await screen.findByText("No requests")).toBeInTheDocument();
    expect(screen.getByText("0 turns")).toBeInTheDocument();
    expect(screen.getByText("Usage not reported")).toBeInTheDocument();
  });

  it("catches a reversed date range before asking the server", async () => {
    const requests = vi.fn();
    server.use(
      http.get(apiUrl("/v1/agents"), () => jsonResponse({ items: [], next_cursor: null })),
      http.get(apiUrl("/v1/sessions"), ({ request }) => { requests(Object.fromEntries(new URL(request.url).searchParams)); return jsonResponse({ items: [], next_cursor: null }); }),
    );
    renderPage();
    fireEvent.change(await screen.findByLabelText("Active from"), { target: { value: "2026-09-16T10:00" } });
    fireEvent.change(screen.getByLabelText("Active until"), { target: { value: "2026-09-16T09:00" } });
    expect(await screen.findByRole("alert")).toHaveTextContent("must be earlier");
    expect(requests).not.toHaveBeenCalledWith(expect.objectContaining({ to: expect.any(String) }));
  });
});
