import { apiClient, type components } from "@/shared/api";
export type ProviderDeleteConflict = components["schemas"]["Problem"];
export async function deleteProvider(id: string) { const { error, response } = await apiClient.DELETE("/v1/providers/{providerId}", { params: { path: { providerId: id } } }); if (!response.ok) throw error; }
export function relatedAgents(error: unknown) { if (!error || typeof error !== "object" || !("related_agents" in error)) return []; return (error as ProviderDeleteConflict).related_agents ?? []; }
