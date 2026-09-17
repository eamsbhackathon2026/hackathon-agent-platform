import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { http } from "msw";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { ConversationsPage } from "./conversations-page";

function conversation(id: string, title: string) {
  return { id, agent_id: "agent-1", source: "playground" as const, created_by_user_id: "user-1", created_by_api_key_id: null, external_key: null, title, created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };
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
});
