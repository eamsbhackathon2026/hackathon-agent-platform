import { describe, expect, it } from "vitest";
import { conversationBudget } from "./conversation-budget";

describe("conversationBudget", () => {
  it("matches what the server holds back and where it starts summarizing", () => {
    // The assistant behind the MSB demo: 200K window, 2048 reserved for the reply.
    // The server took 20,000 as the safety margin and began summarizing at 133,464,
    // so these are the numbers a person reads on screen.
    expect(conversationBudget(200000, 2048)).toEqual({ reply: 2048, safety: 20000, working: 177952, summarizeFrom: 133464 });
  });

  it("keeps a floor under the safety margin on the smallest window", () => {
    // A tenth of 8192 is 819, already above the 512 floor; a 4096 window is what the
    // floor is for, and it must not silently become a tenth of a tiny number.
    expect(conversationBudget(8192, 1024)).toEqual({ reply: 1024, safety: 819, working: 6349, summarizeFrom: 4761 });
    expect(conversationBudget(4096, 1024)?.safety).toBe(512);
  });

  it("reserves what the server would when no reply length is set", () => {
    // Unset means the server reserves 4096, or a quarter of the window when smaller.
    expect(conversationBudget(200000, null)?.reply).toBe(4096);
    expect(conversationBudget(8192, undefined)?.reply).toBe(2048);
  });

  it("has nothing to say about a capacity that is not a number yet", () => {
    // The field is a text input mid-edit: an empty box must not render NaN tokens.
    expect(conversationBudget(Number.NaN, 2048)).toBeNull();
    expect(conversationBudget(0, 2048)).toBeNull();
  });
});
