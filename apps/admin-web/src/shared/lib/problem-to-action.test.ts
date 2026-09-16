import { describe, expect, it } from "vitest";

import { problemActions, problemToAction } from "./problem-to-action";

describe("problemToAction", () => {
  it("provides action copy for every contract code", () => {
    expect(Object.keys(problemActions)).toHaveLength(21);
    for (const value of Object.values(problemActions)) {
      expect(value.title.length).toBeGreaterThan(0);
      expect(value.description.length).toBeGreaterThan(0);
    }
  });

  it("uses a retry fallback for unknown codes", () => {
    expect(problemToAction("future_code")).toMatchObject({ title: "Something went wrong", action: { retry: true } });
  });
});
