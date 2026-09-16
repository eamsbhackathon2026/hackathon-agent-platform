import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import { apiClient, type operations } from "@/shared/api";

type ListSessionsQuery = NonNullable<operations["ListSessions"]["parameters"]["query"]>;

export type ConversationFilters = {
  agentId?: ListSessionsQuery["agent_id"] | undefined;
  source?: ListSessionsQuery["source"] | undefined;
  scope?: ListSessionsQuery["scope"] | undefined;
  sort?: ListSessionsQuery["sort"] | undefined;
  cursor?: ListSessionsQuery["cursor"] | undefined;
};

const conversationRoot = ["conversations"] as const;
const recentMineFilters = { scope: "mine", source: "playground", sort: "updated_at" } as const satisfies ConversationFilters;

export const conversationKeys = {
  all: conversationRoot,
  lists: () => [...conversationRoot, "list"] as const,
  list: (filters: ConversationFilters) => [...conversationRoot, "list", filters] as const,
  infiniteLists: () => [...conversationRoot, "list", "infinite"] as const,
  infiniteList: (filters: ConversationFilters) => [...conversationRoot, "list", "infinite", filters] as const,
  recentMine: () => [...conversationRoot, "list", "recent-mine"] as const,
  detail: (id: string) => [...conversationRoot, id] as const,
  messages: (id: string) => [...conversationRoot, id, "messages"] as const,
  runMessages: (id: string, runId: string) => [...conversationRoot, id, "messages", "run", runId] as const,
  messagesInfinite: (id: string) => [...conversationRoot, id, "messages", "infinite"] as const,
};

async function fetchConversations(filters: ConversationFilters, cursor?: string) {
  const { data, error } = await apiClient.GET("/v1/sessions", { params: { query: {
    limit: 25,
    ...(cursor ? { cursor } : {}),
    ...(filters.agentId ? { agent_id: filters.agentId } : {}),
    ...(filters.source ? { source: filters.source } : {}),
    ...(filters.scope ? { scope: filters.scope } : {}),
    ...(filters.sort ? { sort: filters.sort } : {}),
  } } });
  if (!data) throw error;
  return data;
}

async function fetchMessages(id: string, cursor?: string) {
  const { data, error } = await apiClient.GET("/v1/sessions/{sessionId}/messages", { params: {
    path: { sessionId: id }, query: { limit: 100, ...(cursor ? { cursor } : {}) },
  } });
  if (!data) throw error;
  return data;
}

export const conversationQueries = {
  list: (filters: ConversationFilters = {}) => queryOptions({
    queryKey: conversationKeys.list(filters),
    queryFn: () => fetchConversations(filters, filters.cursor),
  }),
  infiniteList: (filters: ConversationFilters = {}) => infiniteQueryOptions({
    queryKey: conversationKeys.infiniteList(filters),
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => fetchConversations(filters, pageParam),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  }),
  recentMine: () => infiniteQueryOptions({
    queryKey: conversationKeys.recentMine(),
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => fetchConversations(recentMineFilters, pageParam),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  }),
  detail: (id: string) => queryOptions({
    queryKey: conversationKeys.detail(id),
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/sessions/{sessionId}", { params: { path: { sessionId: id } } });
      if (!data) throw error;
      return data;
    },
  }),
  messages: (id: string) => queryOptions({
    queryKey: conversationKeys.messages(id),
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/sessions/{sessionId}/messages", { params: { path: { sessionId: id }, query: { limit: 100 } } });
      if (!data) throw error;
      return data.items;
    },
  }),
  runMessages: (id: string, runId: string) => queryOptions({
    queryKey: conversationKeys.runMessages(id, runId),
    queryFn: async ({ signal }) => {
      const { data, error } = await apiClient.GET("/v1/sessions/{sessionId}/messages", { params: {
        path: { sessionId: id }, query: { limit: 100, run_id: runId },
      }, signal });
      if (!data) throw error;
      return data.items;
    },
  }),
  messagesInfinite: (id: string) => infiniteQueryOptions({
    queryKey: conversationKeys.messagesInfinite(id),
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => fetchMessages(id, pageParam),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  }),
};
