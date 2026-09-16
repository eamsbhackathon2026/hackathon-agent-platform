import { Badge } from "@/shared/ui";

export function AgentStatusBadge({ ready }: { ready: boolean }) {
  return ready ? <Badge variant="success">Ready</Badge> : <Badge variant="warning">Connection needs attention</Badge>;
}
