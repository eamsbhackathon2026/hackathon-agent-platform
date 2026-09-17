import { describe, expect, it } from "vitest";

import type { ConversationMessage } from "@/entities/conversation";
import { groupTurns } from "./group-turns";

function message(seq: number, runId: string | null, role: ConversationMessage["role"], content: string, toolCalls: ConversationMessage["tool_calls"] = []): ConversationMessage {
  return { id: `m-${seq}`, session_id: "s-1", run_id: runId, seq, role, content, tool_calls: toolCalls, tool_call_id: null, tool_name: null, is_error: false, created_at: `2026-09-17T10:00:0${seq}Z` };
}

describe("groupTurns", () => {
  it("splits messages into one turn per request and picks the visible answer", () => {
    const turns = groupTurns([
      message(3, "run-1", "tool", "{}"),
      message(1, "run-1", "user", "Balance?"),
      message(2, "run-1", "assistant", "", [{ id: "c-1", name: "lookup", arguments: {} }]),
      message(4, "run-1", "assistant", "Your balance is 10"),
      message(5, "run-2", "user", "Thanks"),
    ]);
    expect(turns.map((turn) => turn.key)).toEqual(["run-1", "run-2"]);
    expect(turns[0]?.messages.map((item) => item.seq)).toEqual([1, 2, 3, 4]);
    expect(turns[0]?.request?.content).toBe("Balance?");
    expect(turns[0]?.answer?.content).toBe("Your balance is 10");
    expect(turns[1]?.answer).toBeUndefined();
  });

  it("keeps messages saved without a request in their own turn", () => {
    const turns = groupTurns([message(1, null, "user", "Imported"), message(2, null, "assistant", "Imported answer"), message(3, "run-1", "user", "New")]);
    expect(turns.map((turn) => [turn.key, turn.runId])).toEqual([["m-1", null], ["run-1", "run-1"]]);
    expect(turns[0]?.answer?.content).toBe("Imported answer");
  });
});
