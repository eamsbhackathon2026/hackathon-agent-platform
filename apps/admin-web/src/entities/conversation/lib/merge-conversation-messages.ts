import type { ConversationMessage } from "../model/types";

function isPersistedDuplicate(message: ConversationMessage, persisted: ConversationMessage[]) {
  if (!message.id.startsWith("optimistic:")) return false;
  const createdAt = new Date(message.created_at).valueOf();
  return persisted.some((candidate) => {
    const persistedAt = new Date(candidate.created_at).valueOf();
    return candidate.role === message.role
      && candidate.content === message.content
      && persistedAt >= createdAt
      && persistedAt - createdAt < 5 * 60_000;
  });
}

export function mergeConversationMessages(persisted: ConversationMessage[], optimistic: ConversationMessage[]) {
  const ids = new Set(persisted.map((message) => message.id));
  return [
    ...persisted,
    ...optimistic.filter((message) => !ids.has(message.id) && !isPersistedDuplicate(message, persisted)),
  ];
}
