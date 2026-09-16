import { Badge } from "@/shared/ui";
import type { Member } from "../model/types";

export function MemberStatusBadge({ status }: Pick<Member, "status">) {
  return <Badge variant={status === "active" ? "success" : "outline"}>{status === "active" ? "Active" : "Disabled"}</Badge>;
}
