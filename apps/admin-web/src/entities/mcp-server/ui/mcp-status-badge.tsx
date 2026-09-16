import { Badge } from "@/shared/ui";
import type { McpServer } from "../model/types";

export function McpStatusBadge({ status }: Pick<McpServer, "status">) {
  const label = status === "ok" ? "Ready" : status === "failing" ? "Needs attention" : "Not checked";
  return <Badge variant={status === "ok" ? "success" : status === "failing" ? "destructive" : "secondary"}>{label}</Badge>;
}
