import { describe, expect, it } from "vitest";

import { providerReadiness } from "./provider-readiness";

describe("providerReadiness", () => {
  it.each([
    ["ok", "Ready", null],
    ["unchecked", "Not checked", "Test connection"],
    ["failing", "Connection failed", "Check connection"],
  ] as const)("maps %s to a label and action", (status, label, action) => {
    expect(providerReadiness(status)).toMatchObject({ label, action });
  });
});
