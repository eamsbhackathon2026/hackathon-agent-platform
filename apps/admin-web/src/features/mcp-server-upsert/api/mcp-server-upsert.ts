import { apiClient, type components } from "@/shared/api";
export async function createMcpServer(body: components["schemas"]["McpServerCreateRequest"]) { const { data, error } = await apiClient.POST("/v1/mcp-servers", { body }); if (!data) throw error; return data; }
export async function updateMcpServer(id: string, body: components["schemas"]["McpServerUpdateRequest"]) { const { data, error } = await apiClient.PATCH("/v1/mcp-servers/{serverId}", { params: { path: { serverId: id } }, body }); if (!data) throw error; return data; }
