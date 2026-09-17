import type { McpServer } from "@/entities/mcp-server";
import type { HttpTool } from "@/entities/tool";

/**
 * One tool a skill asked for, matched against what this workspace actually has.
 * An MCP reference resolves to the server that carries the tool, because an assistant
 * is connected to a whole tool server rather than to one tool inside it.
 */
export type ResolvedSkillTool = {
  kind: "tool" | "server";
  id: string;
  ref: string;
  label: string;
  groupLabel: string;
};

export type SkillToolResolution = {
  resolved: ResolvedSkillTool[];
  unresolved: string[];
};

export type SkillToolCatalog = {
  tools: HttpTool[];
  servers: McpServer[];
  connectionNames: Map<string, string>;
};

export type SkillToolSelection = {
  toolIds: readonly string[];
  serverIds: readonly string[];
};

const DIRECT_URL_GROUP = "Direct URL";
const SERVER_GROUP = "Tool servers";

/**
 * Matches the references a skill declares against the workspace catalog. A reference
 * nothing matches is not an error: the skill still says what it wants, and the screen
 * says the workspace has no such tool yet.
 */
export function resolveSkillTools(refs: readonly string[], catalog: SkillToolCatalog): SkillToolResolution {
  const resolved: ResolvedSkillTool[] = [];
  const unresolved: string[] = [];
  const seen = new Set<string>();
  for (const ref of refs) {
    const match = matchRef(ref, catalog);
    if (!match) {
      if (!unresolved.includes(ref)) unresolved.push(ref);
      continue;
    }
    // Two references into the same tool server collapse into the one thing a person
    // would actually turn on.
    const key = `${match.kind}:${match.id}`;
    if (seen.has(key)) continue;
    seen.add(key);
    resolved.push(match);
  }
  return { resolved, unresolved };
}

/** Tools a skill needs that the current selection does not already include. */
export function missingSkillTools(resolution: SkillToolResolution, selection: SkillToolSelection): ResolvedSkillTool[] {
  return resolution.resolved.filter((item) => !isSelected(item, selection));
}

/** Whether one resolved entry is currently turned on. */
export function isSelected(item: ResolvedSkillTool, selection: SkillToolSelection): boolean {
  return item.kind === "tool" ? selection.toolIds.includes(item.id) : selection.serverIds.includes(item.id);
}

function matchRef(ref: string, catalog: SkillToolCatalog): ResolvedSkillTool | null {
  const separator = ref.indexOf(".");
  if (separator < 0) {
    const tool = catalog.tools.find((candidate) => candidate.slug === ref);
    if (!tool) return null;
    return {
      kind: "tool",
      id: tool.id,
      ref,
      label: tool.display_name,
      groupLabel: tool.connection_id ? catalog.connectionNames.get(tool.connection_id) ?? "Unknown connection" : DIRECT_URL_GROUP,
    };
  }
  const serverSlug = ref.slice(0, separator);
  const toolName = ref.slice(separator + 1);
  const server = catalog.servers.find((candidate) => candidate.slug === serverSlug);
  if (!server || !server.tools.some((candidate) => candidate.name === toolName)) return null;
  return { kind: "server", id: server.id, ref, label: server.display_name, groupLabel: SERVER_GROUP };
}
