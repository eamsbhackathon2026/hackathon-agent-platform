import type { Provider } from "./provider-readiness";

export type ProviderKind = Provider["kind"];
export type ProviderModel = { id: string; display_name: string };

export const GREENNODE_MODELS: ProviderModel[] = [
  { id: "z-ai/glm-5.2-hackathon", display_name: "GLM 5.2 Hackathon" },
  { id: "qwen/qwen3.6-flash", display_name: "Qwen 3.6 Flash" },
];

export function providerKindLabel(kind: ProviderKind): string {
  if (kind === "gemini") return "Google Gemini";
  if (kind === "greennode") return "GreenNode";
  return "Company AI server";
}

export function modelLogo(modelId: string): { src: string; label: string; monochrome: boolean } | null {
  if (modelId.startsWith("z-ai/")) return { src: "/model-logos/zai.svg", label: "Z.ai", monochrome: true };
  if (modelId.startsWith("qwen/")) return { src: "/model-logos/qwen.svg", label: "Qwen", monochrome: false };
  return null;
}
