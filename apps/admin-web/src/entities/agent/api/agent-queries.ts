import { queryOptions } from "@tanstack/react-query";

import { apiClient, queryKeys } from "@/shared/api";

export const agentQueries = {
  list: () =>
    queryOptions({
      queryKey: queryKeys.agents,
      queryFn: async () => {
        const { data, error } = await apiClient.GET("/v1/agents");
        if (!data) throw error;
        return data.items;
      },
    }),
  detail: (id: string) =>
    queryOptions({
      queryKey: queryKeys.agent(id),
      queryFn: async () => {
        const { data, error } = await apiClient.GET("/v1/agents/{agentId}", { params: { path: { agentId: id } } });
        if (!data) throw error;
        return data;
      },
    }),
};
