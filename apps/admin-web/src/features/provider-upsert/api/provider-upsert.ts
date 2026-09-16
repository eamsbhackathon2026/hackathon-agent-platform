import { apiClient, type components } from "@/shared/api";
export async function createProvider(body: components["schemas"]["ProviderCreateRequest"]) { const { data, error } = await apiClient.POST("/v1/providers", { body }); if (!data) throw error; return data; }
export async function updateProvider(id: string, body: components["schemas"]["ProviderUpdateRequest"]) { const { data, error } = await apiClient.PATCH("/v1/providers/{providerId}", { params: { path: { providerId: id } }, body }); if (!data) throw error; return data; }
