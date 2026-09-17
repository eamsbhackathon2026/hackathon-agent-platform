import { apiClient, type components } from "@/shared/api";

export type ToolBundle = components["schemas"]["ToolBundle"];
export type ToolImportItem = components["schemas"]["ToolImportItem"];
export type ToolImportDecision = components["schemas"]["ToolImportDecision"];
export type ToolImportResult = components["schemas"]["ToolImportResult"];

export async function exportTools() {
  const { data, error } = await apiClient.GET("/v1/tools/export");
  if (!data) throw error;
  return data;
}

export async function previewToolImport(bundle: ToolBundle) {
  const { data, error } = await apiClient.POST("/v1/tools/import/preview", { body: bundle });
  if (!data) throw error;
  return data.items;
}

export async function importTools(bundle: ToolBundle, decisions: ToolImportDecision[]) {
  const { data, error } = await apiClient.POST("/v1/tools/import", { body: { bundle, decisions } });
  if (!data) throw error;
  return data;
}
