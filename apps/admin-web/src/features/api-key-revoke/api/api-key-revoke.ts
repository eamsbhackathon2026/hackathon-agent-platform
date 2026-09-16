import { apiClient } from "@/shared/api";
export async function revokeApiKey(id: string) { const { error, response } = await apiClient.DELETE("/v1/api-keys/{keyId}", { params: { path: { keyId: id } } }); if (!response.ok) throw error; }
