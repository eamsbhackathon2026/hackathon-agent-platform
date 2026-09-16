import { QueryClient } from "@tanstack/react-query";
import { http } from "msw";
import { describe, expect, it, vi } from "vitest";

import { server } from "@/test/msw-server";
import { apiUrl, jsonResponse } from "@/test/typed-handlers";
import { runQueries } from "./run-queries";

const run = { id: "run-1", agent_id: "agent-1", session_id: "session-1", mode: "stream" as const, status: "failed" as const, source: "api" as const, triggered_by_user_id: null, triggered_by_api_key_id: "key-1", input: { message: "Hello" }, output: null, error: null, iterations: 1, usage: { input_tokens: null, output_tokens: null }, metadata: {}, cancel_requested_at: null, queued_at: null, started_at: null, finished_at: null, created_at: "2026-09-15T00:00:00Z" };

describe("runQueries", () => {
  it("passes every supported activity filter", async () => {
    const received = vi.fn();
    server.use(http.get(apiUrl("/v1/runs"), ({ request }) => {
      received(Object.fromEntries(new URL(request.url).searchParams));
      return jsonResponse({ items: [run], next_cursor: null });
    }));
    const client = new QueryClient();
    const filters = { agentId: "agent-1", status: "failed" as const, source: "api" as const, from: "2026-09-01T00:00:00Z", to: "2026-10-01T00:00:00Z", cursor: "cursor-1" };
    expect((await client.fetchQuery(runQueries.list(filters))).items).toEqual([run]);
    expect(received).toHaveBeenCalledWith({ limit: "25", cursor: "cursor-1", agent_id: "agent-1", status: "failed", source: "api", from: filters.from, to: filters.to });
    expect((await client.fetchQuery(runQueries.list({}))).items).toEqual([run]);
    expect(received).toHaveBeenLastCalledWith({ limit: "25" });
  });

  it("loads infinite activity, detail, and delivery data", async () => {
    server.use(
      http.get(apiUrl("/v1/runs"), () => jsonResponse({ items: [run], next_cursor: null })),
      http.get("*/v1/runs/:runId/webhook-deliveries", () => jsonResponse({ items: [], next_cursor: null })),
      http.get("*/v1/runs/:runId", () => jsonResponse(run)),
    );
    const client = new QueryClient();
    expect((await client.fetchInfiniteQuery(runQueries.infiniteList({ agentId: "agent-1", status: "failed", source: "api", from: "from", to: "to" }))).pages[0]?.items).toEqual([run]);
    expect((await client.fetchQuery(runQueries.detail("run-1"))).id).toBe("run-1");
    expect(await client.fetchQuery(runQueries.deliveries("run-1"))).toEqual([]);
  });
});
