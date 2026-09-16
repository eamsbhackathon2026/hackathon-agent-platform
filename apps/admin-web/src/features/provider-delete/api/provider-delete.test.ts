import { describe, expect, it } from "vitest";
import { relatedAgents } from "./provider-delete";

describe("relatedAgents", () => {
  it("returns the assistants that block deletion", () => {
    expect(relatedAgents({ related_agents: [{ id: "a1", name: "Orders" }] })).toEqual([{ id: "a1", name: "Orders" }]);
  });
});
