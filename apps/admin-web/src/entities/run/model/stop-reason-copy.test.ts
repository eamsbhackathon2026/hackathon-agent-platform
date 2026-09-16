import { describe, expect, it } from "vitest";

import { stopReasonToAction } from "./stop-reason-copy";

describe("stopReasonToAction", () => {
  it.each([
    ["loop_detected", "/tools"],
    ["max_iterations_reached", "/agents"],
    ["run_timeout", "/agents"],
    ["provider_auth_failed", "/connections"],
    ["provider_unreachable", "/connections"],
    ["interrupted", undefined],
  ])("maps %s to useful copy and an action", (code, target) => {
    const copy = stopReasonToAction(code);
    expect(copy?.title).toBeTruthy();
    expect(copy?.action).toBeTruthy();
    expect(copy?.action?.to).toBe(target);
  });

  it("links configurable limits to the affected assistant", () => {
    expect(stopReasonToAction("run_timeout", "agent-1")?.action?.to).toBe("/agents/agent-1#advanced");
  });

  it("sends an expired session back to login", () => {
    expect(stopReasonToAction("unauthenticated")?.action).toMatchObject({ label: "Sign in", to: "/login" });
  });
});
