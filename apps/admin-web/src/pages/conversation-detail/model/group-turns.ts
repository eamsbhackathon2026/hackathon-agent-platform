import type { ConversationMessage } from "@/entities/conversation";

export type ConversationTurn = {
  /** Stable React key: the request ID, or the first message ID when no request is recorded. */
  key: string;
  runId: string | null;
  messages: ConversationMessage[];
  /** What the person asked in this turn. */
  request: ConversationMessage | undefined;
  /** The last assistant message with visible text; tool-only decisions are skipped. */
  answer: ConversationMessage | undefined;
};

/**
 * Splits a conversation into turns, one per request. Messages arrive in `seq` order and
 * each request writes its messages contiguously, so a change of `run_id` starts a new
 * turn. Messages saved without a request stay together as their own turn.
 */
export function groupTurns(messages: ConversationMessage[]): ConversationTurn[] {
  const turns: ConversationTurn[] = [];
  for (const message of [...messages].sort((left, right) => left.seq - right.seq)) {
    const current = turns.at(-1);
    if (!current || current.runId !== message.run_id) {
      turns.push({ key: message.run_id ?? message.id, runId: message.run_id, messages: [message], request: undefined, answer: undefined });
    } else {
      current.messages.push(message);
    }
  }
  for (const turn of turns) {
    turn.request = turn.messages.find((message) => message.role === "user");
    turn.answer = [...turn.messages].reverse().find((message) => message.role === "assistant" && message.content.trim() !== "");
  }
  return turns;
}
