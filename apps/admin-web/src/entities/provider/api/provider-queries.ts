import { queryOptions } from "@tanstack/react-query";

import { apiClient, queryKeys } from "@/shared/api";

export const providerQueries = {
  list: () =>
    queryOptions({
      queryKey: queryKeys.providers,
      queryFn: async () => {
        const { data, error } = await apiClient.GET("/v1/providers");
        if (!data) throw error;
        return data.items;
      },
    }),
  detail: (id: string) =>
    queryOptions({
      queryKey: queryKeys.provider(id),
      queryFn: async () => {
        const { data, error } = await apiClient.GET("/v1/providers/{providerId}", { params: { path: { providerId: id } } });
        if (!data) throw error;
        return data;
      },
    }),
};
