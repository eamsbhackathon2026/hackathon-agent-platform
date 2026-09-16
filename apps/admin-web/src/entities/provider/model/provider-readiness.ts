import type { components } from "@/shared/api";

export type Provider = components["schemas"]["Provider"];
export type ConnectionStatus = components["schemas"]["ConnectionStatus"];

export type ProviderReadiness = {
  label: string;
  tone: "ready" | "warning" | "danger";
  action: string | null;
};

export function providerReadiness(status: ConnectionStatus): ProviderReadiness {
  if (status === "ok") return { label: "Ready", tone: "ready", action: null };
  if (status === "failing") return { label: "Connection failed", tone: "danger", action: "Check connection" };
  return { label: "Not checked", tone: "warning", action: "Test connection" };
}
