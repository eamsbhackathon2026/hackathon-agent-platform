import { apiClient } from "@/shared/api";

export async function cancelRun(runId: string) {
  const { data, error } = await apiClient.POST("/v1/runs/{runId}/cancel", { params: { path: { runId } } });
  if (!data) throw error;
  return data;
}
