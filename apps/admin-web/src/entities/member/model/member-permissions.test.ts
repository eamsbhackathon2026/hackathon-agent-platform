import { describe, expect, it } from "vitest";
import { canAssignRole, canManageMembers, canModifyMember } from "./member-permissions";

describe("member permissions", () => {
  it("keeps regular members read-only", () => expect(canManageMembers("member")).toBe(false));
  it("lets an admin create only regular members", () => { expect(canAssignRole("admin", "member")).toBe(true); expect(canAssignRole("admin", "admin")).toBe(false); expect(canAssignRole("admin", "owner")).toBe(false); });
  it("lets only an owner modify elevated accounts", () => { expect(canModifyMember("admin", "member")).toBe(true); expect(canModifyMember("admin", "admin")).toBe(false); expect(canModifyMember("owner", "owner")).toBe(true); });
});
