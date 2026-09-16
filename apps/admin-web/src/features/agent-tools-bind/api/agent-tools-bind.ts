import { apiClient, type components } from "@/shared/api";
export async function replaceAgentTools(id: string, body: components["schemas"]["AgentToolBindings"]) { const { data, error } = await apiClient.PUT("/v1/agents/{agentId}/tools", { params: { path: { agentId: id } }, body }); if (!data) throw error; return data; }
