import { apiClient } from "@/shared/api";
export async function refreshMcpServer(id: string) { const { data, error } = await apiClient.POST("/v1/mcp-servers/{serverId}/refresh", { params: { path: { serverId: id } } }); if (!data) throw error; return data; }
