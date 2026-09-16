import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { apiConnectionQueries } from "@/entities/api-connection";
import { mcpServerQueries } from "@/entities/mcp-server";
import { toolQueries, type HttpTool } from "@/entities/tool";
import { replaceAgentTools } from "@/features/agent-tools-bind";
import { apiClient } from "@/shared/api";
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Checkbox, Label } from "@/shared/ui";

type ToolGroup = { key: string; title: string; tools: HttpTool[] };

/**
 * Tools arrive as one flat list, but people think in terms of the system a tool talks
 * to. Grouping by API connection makes forty tools from five services scannable.
 */
function groupByConnection(tools: HttpTool[], connectionNames: Map<string, string>): ToolGroup[] {
  const groups = new Map<string, ToolGroup>();
  for (const tool of tools) {
    const key = tool.connection_id ?? "direct";
    const title = tool.connection_id ? (connectionNames.get(tool.connection_id) ?? "Unknown connection") : "Direct URL";
    const group = groups.get(key) ?? { key, title, tools: [] };
    group.tools.push(tool);
    groups.set(key, group);
  }
  return [...groups.values()]
    .map((group) => ({ ...group, tools: [...group.tools].sort((left, right) => left.slug.localeCompare(right.slug)) }))
    .sort((left, right) => left.title.localeCompare(right.title));
}

export function AgentToolsPanel({ agentId, readOnly = false }: { agentId: string; readOnly?: boolean }) {
  const tools = useQuery(toolQueries.list()); const servers = useQuery(mcpServerQueries.list()); const connections = useQuery(apiConnectionQueries.list()); const client = useQueryClient();
  const bindings = useQuery({ queryKey: ["agent-tools", agentId], queryFn: async () => { const { data, error } = await apiClient.GET("/v1/agents/{agentId}/tools", { params: { path: { agentId } } }); if (!data) throw error; return data; } });
  const [toolIds, setToolIds] = useState<string[] | null>(null); const [serverIds, setServerIds] = useState<string[] | null>(null);
  const selectedToolIds = toolIds ?? bindings.data?.tool_ids ?? []; const selectedServerIds = serverIds ?? bindings.data?.mcp_server_ids ?? [];
  const toggle = (values: string[], id: string, setter: (next: string[]) => void) => setter(values.includes(id) ? values.filter((value) => value !== id) : [...values, id]);
  const save = async () => { try { await replaceAgentTools(agentId, { tool_ids: selectedToolIds, mcp_server_ids: selectedServerIds }); await client.invalidateQueries({ queryKey: ["agent-tools", agentId] }); toast.success("Assistant tools updated"); } catch { toast.error("Unable to update tools", { description: "Review the selected tools and try again." }); } };
  const loaded = tools.isSuccess && servers.isSuccess && connections.isSuccess && bindings.isSuccess;
  const failed = tools.isError || servers.isError || connections.isError || bindings.isError;
  const connectionNames = new Map((connections.data ?? []).map((connection) => [connection.id, connection.display_name]));
  const groups = groupByConnection(tools.data ?? [], connectionNames);
  const total = (tools.data?.length ?? 0) + (servers.data?.length ?? 0);
  return <Card><CardHeader><CardTitle>Allowed tools</CardTitle><CardDescription>{readOnly ? "Tools currently connected to this assistant." : "Select only the tools this assistant needs for its work."}{loaded && total ? ` ${selectedToolIds.length + selectedServerIds.length} of ${total} selected.` : ""}</CardDescription></CardHeader><CardContent className="space-y-5">
    {failed ? <p className="text-sm text-destructive">Unable to load tools. <Button variant="link" onClick={() => { void tools.refetch(); void servers.refetch(); void connections.refetch(); void bindings.refetch(); }}>Try again</Button></p> : null}
    {groups.map((group) => <section key={group.key} className="space-y-2" aria-label={group.title}>
      <h3 className="text-sm font-medium">{group.title} <span className="text-muted-foreground">· {group.tools.length}</span></h3>
      <div className="grid gap-2 md:grid-cols-2">{group.tools.map((tool) => <Label key={tool.id} className="flex items-start gap-2 rounded-md border p-3"><Checkbox className="mt-0.5" disabled={readOnly} checked={selectedToolIds.includes(tool.id)} onCheckedChange={() => toggle(selectedToolIds, tool.id, setToolIds)} /><span className="grid gap-1"><span>{tool.display_name}</span><span className="text-xs text-muted-foreground"><code>{tool.slug}</code> · {tool.method} {tool.url_template}</span></span></Label>)}</div>
    </section>)}
    {servers.data?.length ? <section className="space-y-2" aria-label="Tool servers">
      <h3 className="text-sm font-medium">Tool servers <span className="text-muted-foreground">· {servers.data.length}</span></h3>
      <div className="grid gap-2 md:grid-cols-2">{servers.data.map((server) => <Label key={server.id} className="flex items-center gap-2 rounded-md border p-3"><Checkbox disabled={readOnly} checked={selectedServerIds.includes(server.id)} onCheckedChange={() => toggle(selectedServerIds, server.id, setServerIds)} />{server.display_name}</Label>)}</div>
    </section> : null}
    {loaded && !total ? <p className="text-sm text-muted-foreground">No tools are available. Create one on the Tools page.</p> : !readOnly && loaded ? <Button type="button" onClick={save}>Save tool list</Button> : null}
  </CardContent></Card>;
}
