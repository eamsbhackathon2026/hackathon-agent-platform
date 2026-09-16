import { describe, expect, it } from "vitest";
import { needsSideEffectConfirmation } from "./tool-test-safety";

describe("tool test safety", () => {
  it.each(["POST", "PUT", "PATCH", "DELETE"] as const)("requires confirmation for %s", (method) => expect(needsSideEffectConfirmation(method)).toBe(true));
  it("does not interrupt a read-only request", () => expect(needsSideEffectConfirmation("GET")).toBe(false));
});
