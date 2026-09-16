import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import { apiClient, queryKeys } from "@/shared/api";

export const skillQueries = {
  list: () => infiniteQueryOptions({
    queryKey: queryKeys.skills,
    initialPageParam: null as string | null,
    queryFn: async ({ pageParam }) => {
      const query = pageParam ? { limit: 100, cursor: pageParam } : { limit: 100 };
      const { data, error } = await apiClient.GET("/v1/skills", { params: { query } });
      if (!data) throw error;
      return data;
    },
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  }),
  detail: (id: string) => queryOptions({
    queryKey: queryKeys.skill(id),
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/skills/{skillId}", { params: { path: { skillId: id } } });
      if (!data) throw error;
      return data;
    },
  }),
};
