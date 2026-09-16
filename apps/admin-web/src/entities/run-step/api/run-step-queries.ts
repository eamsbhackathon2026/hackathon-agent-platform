import { queryOptions } from "@tanstack/react-query";

import { apiClient } from "@/shared/api";

export const runStepQueries = {
  list: (runId: string) => queryOptions({
    queryKey: ["runs", runId, "steps"] as const,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/runs/{runId}/spans", { params: { path: { runId }, query: { limit: 100 } } });
      if (!data) throw error;
      return data.items;
    },
  }),
};
