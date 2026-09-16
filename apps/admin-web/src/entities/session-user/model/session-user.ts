import type { SessionUser } from "@/shared/api";

export type UserRole = SessionUser["role"];

export function canManageWorkspace(role: UserRole) {
  return role === "owner" || role === "admin";
}

export function roleLabel(role: UserRole) {
  return { owner: "Owner", admin: "Administrator", member: "Member" }[role];
}
