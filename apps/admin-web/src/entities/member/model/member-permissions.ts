import type { Member } from "./types";

export function canManageMembers(role: Member["role"] | undefined) {
  return role === "owner" || role === "admin";
}

export function canAssignRole(actor: Member["role"] | undefined, role: Member["role"]) {
  return actor === "owner" || (actor === "admin" && role === "member");
}

export function canModifyMember(actor: Member["role"] | undefined, target: Member["role"]) {
  return actor === "owner" || (actor === "admin" && target === "member");
}
