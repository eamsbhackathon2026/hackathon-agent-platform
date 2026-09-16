import { apiClient } from "@/shared/api";
export async function testHttpTool(id: string, args: Record<string, unknown>) { const { data, error } = await apiClient.POST("/v1/tools/{toolId}/test", { params: { path: { toolId: id } }, body: { args } }); if (!data) throw error; return data; }
