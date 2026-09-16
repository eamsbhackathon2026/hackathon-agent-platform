import { beforeEach, describe, expect, it } from "vitest";

import { clearAuthSession, getAuthSession } from "./auth-token-store";
import type { components } from "./generated/schema";
import { refreshSingleFlight, resetRefreshForTests } from "./refresh-single-flight";

type TokenResponse = components["schemas"]["TokenResponse"];

const token: TokenResponse = {
  access_token: "memory-only-token",
  expires_in: 900,
  token_type: "Bearer",
  me: { user: { id: "00000000-0000-0000-0000-000000000001", email: "owner@example.test", name: "Owner", role: "owner", status: "active", must_change_password: false, last_login_at: null, created_at: "2026-09-15T00:00:00Z" } },
};

describe("refreshSingleFlight", () => {
  beforeEach(() => { clearAuthSession(); resetRefreshForTests(); });

  it("uses one refresh for three concurrent requests", async () => {
    let calls = 0;
    const operation = async () => { calls += 1; await Promise.resolve(); return token; };
    const values = await Promise.all([refreshSingleFlight(operation), refreshSingleFlight(operation), refreshSingleFlight(operation)]);
    expect(calls).toBe(1);
    expect(values).toEqual([token, token, token]);
    expect(getAuthSession().accessToken).toBe("memory-only-token");
  });

  it("clears the session when refresh fails", async () => {
    const result = await refreshSingleFlight(async () => { throw new Error("offline"); });
    expect(result).toBeNull();
    expect(getAuthSession().user).toBeNull();
  });
});
