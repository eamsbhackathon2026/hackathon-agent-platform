import type { HttpTool } from "@/entities/tool";

export function needsSideEffectConfirmation(method: HttpTool["method"]) {
  return method === "POST" || method === "PUT" || method === "PATCH" || method === "DELETE";
}
