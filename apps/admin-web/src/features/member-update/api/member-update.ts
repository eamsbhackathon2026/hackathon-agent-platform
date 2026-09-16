import { apiClient, type components } from "@/shared/api";
export async function updateMember(id: string, body: components["schemas"]["MemberUpdateRequest"]) { const { data, error } = await apiClient.PATCH("/v1/members/{userId}", { params: { path: { userId: id } }, body }); if (!data) throw error; return data; }
