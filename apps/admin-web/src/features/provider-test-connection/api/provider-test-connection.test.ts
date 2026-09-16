import { describe, expect, it } from "vitest";
import { providerTestAction } from "./provider-test-connection";
describe("providerTestAction", () => {
  it("turns a rejected credential into an actionable Vietnamese message", () => {
    expect(providerTestAction("provider_auth_failed")).toEqual({ message: "The model connection credentials are incorrect", action: "Edit connection" });
  });
});
