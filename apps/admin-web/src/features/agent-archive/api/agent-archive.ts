import { apiClient } from "@/shared/api";
export async function archiveAgent(id: string) { const { error, response } = await apiClient.DELETE("/v1/agents/{agentId}", { params: { path: { agentId: id } } }); if (!response.ok) throw error; }
