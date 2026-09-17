import { apiClient } from "@/shared/api";

/** Fetches the file a skill downloads as: the upload itself, or its stored SKILL.md. */
export async function fetchSkillFile(skillId: string) {
  const { data, error } = await apiClient.GET("/v1/skills/{skillId}/download", { params: { path: { skillId } }, parseAs: "arrayBuffer" });
  if (!data) throw error;
  return data;
}
