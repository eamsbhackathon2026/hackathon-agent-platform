import { apiClient, type components } from "@/shared/api";
export async function addMember(body: components["schemas"]["MemberCreateRequest"]) { const { data, error } = await apiClient.POST("/v1/members", { body }); if (!data) throw error; return data; }
