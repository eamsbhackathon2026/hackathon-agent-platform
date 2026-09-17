import { queryOptions } from "@tanstack/react-query";

import { apiClient } from "@/shared/api";

export type OverviewFilters = { from: string; to: string; timeZone: string };

export const reportQueries = {
  overview: (filters: OverviewFilters) => queryOptions({
    queryKey: ["reports", "overview", filters] as const,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/reports/overview", { params: { query: {
        from: filters.from,
        to: filters.to,
        time_zone: filters.timeZone,
      } } });
      if (!data) throw error;
      return data;
    },
  }),
};
