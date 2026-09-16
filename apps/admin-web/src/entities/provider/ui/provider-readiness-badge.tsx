import { Badge } from "@/shared/ui";

import { providerReadiness, type ConnectionStatus } from "../model/provider-readiness";

export function ProviderReadinessBadge({ status }: { status: ConnectionStatus }) {
  const state = providerReadiness(status);
  return <Badge variant={state.tone === "ready" ? "success" : "warning"}>{state.label}</Badge>;
}
