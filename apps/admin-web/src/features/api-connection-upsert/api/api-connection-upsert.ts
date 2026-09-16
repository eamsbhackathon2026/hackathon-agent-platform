import { apiClient, type components } from "@/shared/api";

export async function createApiConnection(body: components["schemas"]["ApiConnectionCreateRequest"]) {
  const { data, error } = await apiClient.POST("/v1/api-connections", { body });
  if (!data) throw error;
  return data;
}

export async function updateApiConnection(id: string, body: components["schemas"]["ApiConnectionUpdateRequest"]) {
  const { data, error } = await apiClient.PATCH("/v1/api-connections/{apiConnectionId}", { params: { path: { apiConnectionId: id } }, body });
  if (!data) throw error;
  return data;
}
