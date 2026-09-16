import { apiClient, type components } from "@/shared/api";
export async function createHttpTool(body: components["schemas"]["HttpToolCreateRequest"]) { const { data, error } = await apiClient.POST("/v1/tools", { body }); if (!data) throw error; return data; }
export async function updateHttpTool(id: string, body: components["schemas"]["HttpToolUpdateRequest"]) { const { data, error } = await apiClient.PATCH("/v1/tools/{toolId}", { params: { path: { toolId: id } }, body }); if (!data) throw error; return data; }
