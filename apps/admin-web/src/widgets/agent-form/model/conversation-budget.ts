/** What a conversation capacity actually leaves for the conversation itself.
 *
 *  The number a person types is the model's whole window. Two slices come out of it
 *  before any message does: the room held back for the reply, and a safety margin
 *  covering the difference between our estimate and what the provider counts. Earlier
 *  messages start getting summarized at three quarters of what remains — so a 200K
 *  window begins summarizing around 133K, not 200K, and someone sizing a window has
 *  no way to guess that from the field alone.
 *
 *  The server decides these numbers (ContextSafetyTokens, EffectiveMaxOutputTokens
 *  and the 75% input share); the copies here only explain them on screen, so they
 *  have to be changed together.
 */
const DEFAULT_MAX_OUTPUT_TOKENS = 4096;
const MINIMUM_SAFETY_TOKENS = 512;
const SUMMARY_SHARE_PERCENT = 75;

export type ConversationBudget = {
  /** Room left for messages after the reply reserve and the safety margin. */
  working: number;
  /** Where summarizing earlier messages begins. */
  summarizeFrom: number;
  /** Tokens held back for the reply. */
  reply: number;
  /** Tokens held back to cover counting differences. */
  safety: number;
};

export function conversationBudget(windowTokens: number, maxOutputTokens: number | null | undefined): ConversationBudget | null {
  if (!Number.isFinite(windowTokens) || windowTokens <= 0) return null;
  const windowSize = Math.floor(windowTokens);
  const reply = replyReserve(windowSize, maxOutputTokens);
  const safety = Math.max(Math.floor(windowSize / 10), MINIMUM_SAFETY_TOKENS);
  const working = Math.max(windowSize - reply - safety, 0);
  return { working, summarizeFrom: Math.floor((working * SUMMARY_SHARE_PERCENT) / 100), reply, safety };
}

function replyReserve(windowSize: number, maxOutputTokens: number | null | undefined): number {
  if (typeof maxOutputTokens === "number" && Number.isFinite(maxOutputTokens) && maxOutputTokens > 0) return Math.floor(maxOutputTokens);
  return Math.min(DEFAULT_MAX_OUTPUT_TOKENS, Math.floor(windowSize / 4));
}
