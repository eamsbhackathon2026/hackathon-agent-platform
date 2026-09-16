import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { ConversationMessage } from "@/entities/conversation";

import { ChatThread } from "./chat-thread";

const baseMessage = {
  session_id: "session-1",
  run_id: null,
  tool_calls: [],
  tool_call_id: null,
  tool_name: null,
  is_error: false,
  created_at: "2026-09-15T00:00:00Z",
} satisfies Omit<ConversationMessage, "id" | "seq" | "role" | "content">;

describe("ChatThread", () => {
  it("contains long message content and gives markdown tables local overflow", () => {
    const longToken = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz";
    const messages: ConversationMessage[] = [
      { ...baseMessage, id: "user-1", seq: 1, role: "user", content: longToken },
      { ...baseMessage, id: "assistant-1", seq: 2, role: "assistant", content: `Inline \`${longToken}\`\n\n| Column | Result |\n|---|---|\n| Long | Value |` },
    ];

    render(<ChatThread messages={messages} />);

    expect(screen.getAllByText(longToken)[0]?.parentElement).toHaveClass("break-words", "[overflow-wrap:anywhere]");
    expect(screen.getByRole("table").parentElement).toHaveClass("max-w-full", "overflow-x-auto");
    expect(screen.getByRole("columnheader", { name: "Column" })).toHaveClass("px-2", "py-1");
  });
});
