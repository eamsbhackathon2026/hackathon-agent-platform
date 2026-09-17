import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import { apiClient } from "@/shared/api";
import type { RunSource, RunStatus } from "../model/types";

export type RunFilters = {
  agentId?: string | undefined;
  sessionId?: string | undefined;
  status?: RunStatus | undefined;
  source?: RunSource | undefined;
  from?: string | undefined;
  to?: string | undefined;
  cursor?: string | undefined;
};

async function fetchRuns(filters: RunFilters, cursor?: string, limit = 25) {
  const { data, error } = await apiClient.GET("/v1/runs", { params: { query: {
    limit,
    ...(cursor ? { cursor } : {}),
    ...(filters.agentId ? { agent_id: filters.agentId } : {}),
    ...(filters.sessionId ? { session_id: filters.sessionId } : {}),
    ...(filters.status ? { status: filters.status } : {}),
    ...(filters.source ? { source: filters.source } : {}),
    ...(filters.from ? { from: filters.from } : {}),
    ...(filters.to ? { to: filters.to } : {}),
  } } });
  if (!data) throw error;
  return data;
}

export const runQueries = {
  list: (filters: RunFilters = {}) => queryOptions({
    queryKey: ["runs", filters] as const,
    queryFn: () => fetchRuns(filters, filters.cursor),
  }),
  infiniteList: (filters: RunFilters = {}) => infiniteQueryOptions({
    queryKey: ["runs", "infinite", filters] as const,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => fetchRuns(filters, pageParam),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  }),
  /** Every request made in one conversation, newest first, in the largest pages the API allows. */
  bySession: (sessionId: string) => infiniteQueryOptions({
    queryKey: ["runs", "session", sessionId] as const,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => fetchRuns({ sessionId }, pageParam, 100),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  }),
  detail: (id: string) => queryOptions({
    queryKey: ["runs", id] as const,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/runs/{runId}", { params: { path: { runId: id } } });
      if (!data) throw error;
      return data;
    },
  }),
  deliveries: (id: string) => queryOptions({
    queryKey: ["runs", id, "deliveries"] as const,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/runs/{runId}/webhook-deliveries", { params: { path: { runId: id }, query: { limit: 100 } } });
      if (!data) throw error;
      return data.items;
    },
  }),
};
