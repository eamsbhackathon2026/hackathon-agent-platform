import { queryOptions } from "@tanstack/react-query";
import { apiClient, fetchAllPages } from "@/shared/api";

export const apiConnectionQueries = {
  list: () => queryOptions({
    queryKey: ["api-connections"] as const,
    queryFn: () => fetchAllPages(async (query) => {
      const { data, error } = await apiClient.GET("/v1/api-connections", { params: { query } });
      if (!data) throw error;
      return data;
    }, "API connection"),
  }),
};
