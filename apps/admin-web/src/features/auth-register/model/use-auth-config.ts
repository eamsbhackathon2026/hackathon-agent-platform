import { useQuery } from "@tanstack/react-query";

import { apiClient } from "@/shared/api";

export function useAuthConfig() {
  return useQuery({
    queryKey: ["auth-config"],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/auth/config");
      if (!data) throw error;
      return data;
    },
    staleTime: 30_000,
    retry: false,
  });
}
