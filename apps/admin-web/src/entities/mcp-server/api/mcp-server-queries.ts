import { queryOptions } from "@tanstack/react-query";
import { apiClient, fetchAllPages } from "@/shared/api";

export const mcpServerQueries = {
  list: () => queryOptions({
    queryKey: ["mcp-servers"] as const,
    queryFn: () => fetchAllPages(async (query) => {
      const { data, error } = await apiClient.GET("/v1/mcp-servers", { params: { query } });
      if (!data) throw error;
      return data;
    }, "MCP server"),
  }),
};
