import { apiClient } from "@/shared/api";

export async function importSkill(file: File) {
  const form = new FormData();
  form.append("file", file, file.name);
  const { data, error } = await apiClient.POST("/v1/skills", {
    body: { file: file.name },
    bodySerializer: () => form,
  });
  if (!data) throw error;
  return data;
}
