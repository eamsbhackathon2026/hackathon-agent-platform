import { queryOptions } from "@tanstack/react-query";

import { apiClient, queryKeys, updateSessionUser } from "@/shared/api";

export const meQuery = () =>
  queryOptions({
    queryKey: queryKeys.me,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/me");
      if (!data) throw error;
      updateSessionUser(data.user);
      return data.user;
    },
    staleTime: 60_000,
  });
