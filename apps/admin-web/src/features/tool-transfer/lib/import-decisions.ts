import type { ToolImportDecision, ToolImportItem } from "../api/tool-transfer";

export type ImportChoice = "skip" | "overwrite";

export const itemKey = (item: Pick<ToolImportItem, "kind" | "slug">) => `${item.kind}:${item.slug}`;

/**
 * Decisions sent to the server, keyed like the server keys them (kind + slug). Existing
 * entries follow the admin's choice; invalid ones are skipped unless another entry with
 * the same slug is importable, because a repeated slug is already ignored by the server.
 */
export function importDecisions(items: ToolImportItem[], choices: Record<string, ImportChoice>): ToolImportDecision[] {
  // null reserves the slug of a new entry: it needs no decision, and must not be skipped.
  const decisions = new Map<string, ToolImportDecision | null>();
  for (const item of items) {
    if (item.status === "conflict") decisions.set(itemKey(item), { kind: item.kind, slug: item.slug, action: choices[itemKey(item)] ?? "skip" });
    if (item.status === "new") decisions.set(itemKey(item), null);
  }
  for (const item of items) {
    if (item.status === "invalid" && !decisions.has(itemKey(item))) decisions.set(itemKey(item), { kind: item.kind, slug: item.slug, action: "skip" });
  }
  return [...decisions.values()].filter((decision): decision is ToolImportDecision => decision !== null);
}
