import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
import { toast } from "sonner";

import { replaceAgentTools } from "@/features/agent-tools-bind";
import type { ResolvedSkillTool } from "@/features/agent-skill-tools-sync";
import { apiClient } from "@/shared/api";

import { AgentToolDraftContext, useAgentToolCatalog, type AgentToolDraft } from "./agent-tool-draft-context";

export function AgentToolDraftProvider({ agentId, children }: { agentId: string; children: ReactNode }) {
  const client = useQueryClient();
  const { tools, servers, connections } = useAgentToolCatalog();
  const bindings = useQuery({
    queryKey: ["agent-tools", agentId],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/v1/agents/{agentId}/tools", { params: { path: { agentId } } });
      if (!data) throw error;
      return data;
    },
  });
  // null means untouched, so the panel keeps showing saved bindings until someone edits.
  const [toolIds, setToolIds] = useState<string[] | null>(null);
  const [serverIds, setServerIds] = useState<string[] | null>(null);
  const [saving, setSaving] = useState(false);

  const selectedToolIds = toolIds ?? bindings.data?.tool_ids ?? [];
  const selectedServerIds = serverIds ?? bindings.data?.mcp_server_ids ?? [];

  const draft: AgentToolDraft = {
    agentId,
    toolIds: selectedToolIds,
    serverIds: selectedServerIds,
    loaded: tools.isSuccess && servers.isSuccess && connections.isSuccess && bindings.isSuccess,
    failed: tools.isError || servers.isError || connections.isError || bindings.isError,
    changed: toolIds !== null || serverIds !== null,
    saving,
    toggleTool: (id) => setToolIds(toggle(selectedToolIds, id)),
    toggleServer: (id) => setServerIds(toggle(selectedServerIds, id)),
    addSelections: (items) => applySelections(items, true),
    removeSelections: (items) => applySelections(items, false),
    save: async () => {
      if (saving) return;
      setSaving(true);
      try {
        await replaceAgentTools(agentId, { tool_ids: selectedToolIds, mcp_server_ids: selectedServerIds });
        await client.invalidateQueries({ queryKey: ["agent-tools", agentId] });
        setToolIds(null);
        setServerIds(null);
        toast.success("Assistant tools updated");
      } catch {
        toast.error("Unable to update tools", { description: "Review the selected tools and try again." });
      } finally {
        setSaving(false);
      }
    },
    retry: () => {
      void tools.refetch();
      void servers.refetch();
      void connections.refetch();
      void bindings.refetch();
    },
  };

  function applySelections(items: ResolvedSkillTool[], enable: boolean) {
    const toolTargets = items.filter((item) => item.kind === "tool").map((item) => item.id);
    const serverTargets = items.filter((item) => item.kind === "server").map((item) => item.id);
    if (toolTargets.length) setToolIds(apply(selectedToolIds, toolTargets, enable));
    if (serverTargets.length) setServerIds(apply(selectedServerIds, serverTargets, enable));
  }

  return <AgentToolDraftContext value={draft}>{children}</AgentToolDraftContext>;
}

function toggle(values: string[], id: string): string[] {
  return values.includes(id) ? values.filter((value) => value !== id) : [...values, id];
}

function apply(values: string[], targets: string[], enable: boolean): string[] {
  if (!enable) return values.filter((value) => !targets.includes(value));
  return [...values, ...targets.filter((target) => !values.includes(target))];
}
