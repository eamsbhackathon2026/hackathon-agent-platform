import { apiClient } from "@/shared/api";

export async function deleteSkill(id: string) {
  const { response, error } = await apiClient.DELETE("/v1/skills/{skillId}", { params: { path: { skillId: id } } });
  if (!response.ok) throw error;
}
