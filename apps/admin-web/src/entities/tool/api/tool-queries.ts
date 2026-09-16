import { queryOptions } from "@tanstack/react-query";
import { apiClient, fetchAllPages } from "@/shared/api";

export const toolQueries = {
  list: () => queryOptions({
    queryKey: ["tools"] as const,
    queryFn: () => fetchAllPages(async (query) => {
      const { data, error } = await apiClient.GET("/v1/tools", { params: { query } });
      if (!data) throw error;
      return data;
    }, "Tool"),
  }),
  detail: (id: string) => queryOptions({
    queryKey: ["tools", id] as const,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/tools/{toolId}", { params: { path: { toolId: id } } });
      if (!data) throw error;
      return data;
    },
  }),
};
