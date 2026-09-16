import { queryOptions } from "@tanstack/react-query";
import { apiClient } from "@/shared/api";

export const apiKeyQueries = {
  list: () => queryOptions({
    queryKey: ["api-keys"] as const,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/api-keys");
      if (!data) throw error;
      return data.items;
    },
  }),
};
