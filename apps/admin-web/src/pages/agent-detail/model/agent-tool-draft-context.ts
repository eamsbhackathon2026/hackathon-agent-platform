import { useQuery } from "@tanstack/react-query";
import { createContext, use, useMemo } from "react";

import { apiConnectionQueries } from "@/entities/api-connection";
import { mcpServerQueries } from "@/entities/mcp-server";
import { toolQueries } from "@/entities/tool";
import type { ResolvedSkillTool } from "@/features/agent-skill-tools-sync";

/**
 * Skills and tools are chosen on the same screen but used to be edited by two panels
 * that knew nothing about each other, so turning on a skill could not turn on the tools
 * it needs. The draft lives above both panels: the skills panel proposes changes to it,
 * the tools panel renders it, and one Save button still writes it.
 */
export type AgentToolDraft = {
  agentId: string;
  toolIds: string[];
  serverIds: string[];
  loaded: boolean;
  failed: boolean;
  changed: boolean;
  saving: boolean;
  toggleTool: (id: string) => void;
  toggleServer: (id: string) => void;
  addSelections: (items: ResolvedSkillTool[]) => void;
  removeSelections: (items: ResolvedSkillTool[]) => void;
  save: () => Promise<void>;
  retry: () => void;
};

export const AgentToolDraftContext = createContext<AgentToolDraft | null>(null);

export function useAgentToolDraft(): AgentToolDraft {
  const draft = use(AgentToolDraftContext);
  if (!draft) throw new Error("useAgentToolDraft requires AgentToolDraftProvider");
  return draft;
}

/** Catalog queries the tools panel and the skill dialogs both read. */
export function useAgentToolCatalog() {
  const tools = useQuery(toolQueries.list());
  const servers = useQuery(mcpServerQueries.list());
  const connections = useQuery(apiConnectionQueries.list());
  const connectionNames = useMemo(
    () => new Map((connections.data ?? []).map((connection) => [connection.id, connection.display_name])),
    [connections.data],
  );
  return { tools, servers, connections, connectionNames };
}
