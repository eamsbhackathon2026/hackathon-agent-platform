import { apiClient, type components } from "@/shared/api";
export type AgentDraft = components["schemas"]["AgentCreateRequest"];
export async function createAgent(body: AgentDraft) { const { data, error } = await apiClient.POST("/v1/agents", { body }); if (!data) throw error; return data; }
export async function updateAgent(id: string, body: components["schemas"]["AgentUpdateRequest"]) { const { data, error } = await apiClient.PATCH("/v1/agents/{agentId}", { params: { path: { agentId: id } }, body }); if (!data) throw error; return data; }
