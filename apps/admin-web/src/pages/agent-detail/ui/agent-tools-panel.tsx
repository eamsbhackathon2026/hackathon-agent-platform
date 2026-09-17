import { type HttpTool } from "@/entities/tool";
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Checkbox, Label } from "@/shared/ui";

import { useAgentToolCatalog, useAgentToolDraft } from "../model";

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

export function AgentToolsPanel({ readOnly = false }: { readOnly?: boolean }) {
  const { tools, servers, connectionNames } = useAgentToolCatalog();
  const draft = useAgentToolDraft();
  const groups = groupByConnection(tools.data ?? [], connectionNames);
  const total = (tools.data?.length ?? 0) + (servers.data?.length ?? 0);
  return <Card><CardHeader><CardTitle>Allowed tools</CardTitle><CardDescription>{readOnly ? "Tools currently connected to this assistant." : "Select only the tools this assistant needs for its work."}{draft.loaded && total ? ` ${draft.toolIds.length + draft.serverIds.length} of ${total} selected.` : ""}</CardDescription></CardHeader><CardContent className="space-y-5">
    {draft.failed ? <p className="text-sm text-destructive">Unable to load tools. <Button variant="link" onClick={draft.retry}>Try again</Button></p> : null}
    {groups.map((group) => <section key={group.key} className="space-y-2" aria-label={group.title}>
      <h3 className="text-sm font-medium">{group.title} <span className="text-muted-foreground">· {group.tools.length}</span></h3>
      <div className="grid gap-2 md:grid-cols-2">{group.tools.map((tool) => <Label key={tool.id} className="flex items-start gap-2 rounded-md border p-3"><Checkbox className="mt-0.5" disabled={readOnly} checked={draft.toolIds.includes(tool.id)} onCheckedChange={() => draft.toggleTool(tool.id)} /><span className="grid gap-1"><span>{tool.display_name}</span><span className="text-xs text-muted-foreground"><code>{tool.slug}</code> · {tool.method} {tool.url_template}</span></span></Label>)}</div>
    </section>)}
    {servers.data?.length ? <section className="space-y-2" aria-label="Tool servers">
      <h3 className="text-sm font-medium">Tool servers <span className="text-muted-foreground">· {servers.data.length}</span></h3>
      <div className="grid gap-2 md:grid-cols-2">{servers.data.map((server) => <Label key={server.id} className="flex items-center gap-2 rounded-md border p-3"><Checkbox disabled={readOnly} checked={draft.serverIds.includes(server.id)} onCheckedChange={() => draft.toggleServer(server.id)} />{server.display_name}</Label>)}</div>
    </section> : null}
    {draft.loaded && !total ? <p className="text-sm text-muted-foreground">No tools are available. Create one on the Tools page.</p> : !readOnly && draft.loaded ? <Button type="button" disabled={draft.saving} onClick={() => void draft.save()}>{draft.saving ? "Saving…" : "Save tool list"}</Button> : null}
  </CardContent></Card>;
}
