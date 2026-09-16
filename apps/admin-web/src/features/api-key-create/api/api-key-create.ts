import { apiClient, type components } from "@/shared/api";
export async function createApiKey(body: components["schemas"]["ApiKeyCreateRequest"]) { const { data, error } = await apiClient.POST("/v1/api-keys", { body }); if (!data) throw error; return data; }
