import { queryOptions } from "@tanstack/react-query";
import { apiClient } from "@/shared/api";

export const memberQueries = {
  list: () => queryOptions({
    queryKey: ["members"] as const,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/members");
      if (!data) throw error;
      return data.items;
    },
  }),
};
