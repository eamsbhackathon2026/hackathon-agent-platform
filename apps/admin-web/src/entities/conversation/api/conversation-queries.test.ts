import { QueryClient } from "@tanstack/react-query";
import { http } from "msw";
import { describe, expect, it, vi } from "vitest";

import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { conversationKeys, conversationQueries } from "./conversation-queries";

const session = { id: "session-1", agent_id: "agent-1", source: "api" as const, created_by_user_id: null, created_by_api_key_id: "key-1", external_key: null, title: "Test", created_at: "2026-09-15T00:00:00Z", updated_at: "2026-09-15T00:00:00Z" };

describe("conversationQueries", () => {
  it("passes list filters and returns a page", async () => {
    const received = vi.fn();
    server.use(http.get(apiUrl("/v1/sessions"), ({ request }) => {
      received(Object.fromEntries(new URL(request.url).searchParams));
      return jsonResponse({ items: [session], next_cursor: null });
    }));
    const client = new QueryClient();
    const result = await client.fetchQuery(conversationQueries.list({ agentId: "agent-1", source: "api", cursor: "cursor-1" }));
    expect(result.items).toEqual([session]);
    expect(received).toHaveBeenCalledWith({ limit: "25", cursor: "cursor-1", agent_id: "agent-1", source: "api" });
    expect((await client.fetchQuery(conversationQueries.list({}))).items).toEqual([session]);
    expect(received).toHaveBeenLastCalledWith({ limit: "25" });
  });

  it("loads detail, messages, and infinite variants", async () => {
    const messageParams = vi.fn();
    server.use(
      http.get(apiUrl("/v1/sessions"), () => jsonResponse({ items: [session], next_cursor: null })),
      http.get("*/v1/sessions/:sessionId/messages", ({ request }) => {
        messageParams(Object.fromEntries(new URL(request.url).searchParams));
        return jsonResponse({ items: [], next_cursor: null });
      }),
      http.get("*/v1/sessions/:sessionId", () => jsonResponse(session)),
    );
    const client = new QueryClient();
    expect((await client.fetchQuery(conversationQueries.detail("session-1"))).id).toBe("session-1");
    expect(await client.fetchQuery(conversationQueries.messages("session-1"))).toEqual([]);
    expect(await client.fetchQuery(conversationQueries.runMessages("session-1", "run-1"))).toEqual([]);
    expect(messageParams).toHaveBeenLastCalledWith({ limit: "100", run_id: "run-1" });
    expect((await client.fetchInfiniteQuery(conversationQueries.infiniteList({ agentId: "agent-1", source: "api" }))).pages[0]?.items).toEqual([session]);
    expect((await client.fetchInfiniteQuery(conversationQueries.messagesInfinite("session-1"))).pages[0]?.items).toEqual([]);
  });

  it("requests only recent personal Playground conversations", async () => {
    const received = vi.fn();
    server.use(http.get(apiUrl("/v1/sessions"), ({ request }) => {
      received(Object.fromEntries(new URL(request.url).searchParams));
      return jsonResponse({ items: [{ ...session, source: "playground" as const }], next_cursor: "next" });
    }));
    const client = new QueryClient();

    const result = await client.fetchInfiniteQuery(conversationQueries.recentMine());

    expect(result.pages[0]?.items).toHaveLength(1);
    expect(received).toHaveBeenCalledWith({ limit: "25", scope: "mine", source: "playground", sort: "updated_at" });
    expect(conversationQueries.recentMine().queryKey).toEqual(conversationKeys.recentMine());
  });
});
