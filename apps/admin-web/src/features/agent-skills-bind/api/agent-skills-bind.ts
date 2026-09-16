import { apiClient, type components } from "@/shared/api";

export async function replaceAgentSkills(id: string, body: components["schemas"]["AgentSkillBindings"]) {
  const { data, error } = await apiClient.PUT("/v1/agents/{agentId}/skills", { params: { path: { agentId: id } }, body });
  if (!data) throw error;
  return data;
}
