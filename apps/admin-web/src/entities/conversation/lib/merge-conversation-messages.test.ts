import { describe, expect, it } from "vitest";

import type { ConversationMessage } from "../model/types";
import { mergeConversationMessages } from "./merge-conversation-messages";

function message(id: string, content: string, createdAt = "2026-09-15T00:00:00Z"): ConversationMessage {
  return { id, session_id: "session", run_id: null, seq: 1, role: "user", content, tool_calls: [], tool_call_id: null, tool_name: null, is_error: false, created_at: createdAt };
}

describe("mergeConversationMessages", () => {
  it("removes an optimistic message once the same content is persisted", () => {
    expect(mergeConversationMessages([message("stored", "Hello")], [message("optimistic:1", "Hello")])).toHaveLength(1);
  });

  it("keeps separate messages and same text sent much later", () => {
    expect(mergeConversationMessages(
      [message("stored", "Hello")],
      [message("optimistic:1", "Other"), message("optimistic:2", "Hello", "2026-09-15T01:00:00Z")],
    )).toHaveLength(3);
  });

  it("keeps a repeated prompt when the persisted copy predates the optimistic send", () => {
    expect(mergeConversationMessages(
      [message("stored", "Lặp lại", "2026-09-15T00:00:00Z")],
      [message("optimistic:next", "Lặp lại", "2026-09-15T00:00:05Z")],
    )).toHaveLength(2);
  });
});
