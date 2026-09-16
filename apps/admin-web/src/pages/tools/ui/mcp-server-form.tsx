import type { McpServer } from "@/entities/mcp-server";
import { type HeaderRow } from "@/shared/lib";
import { Button, Card, CardContent, CardHeader, CardTitle, Checkbox, HeaderRowsEditor, Input, Label } from "@/shared/ui";

type Props = { editing: McpServer | null; allowedTools: string[] | null; secretHeaders: HeaderRow[]; replaceSecrets: boolean; onAllowedTools: (tools: string[] | null) => void; onSecretHeaders: (rows: HeaderRow[]) => void; onReplaceSecrets: (replace: boolean) => void; onSubmit: (event: React.FormEvent<HTMLFormElement>) => void };
export function McpServerForm(props: Props) {
  const { editing } = props;
  return <Card><CardHeader><CardTitle>{editing ? "Update tool server" : "New tool server"}</CardTitle></CardHeader><CardContent><form key={editing?.id ?? "new-server"} className="space-y-5" onSubmit={props.onSubmit}>
    <div className="grid gap-4 md:grid-cols-2"><Field label="Display name"><Input name="display_name" defaultValue={editing?.display_name} required /></Field><Field label="Short name"><Input name="slug" defaultValue={editing?.slug} required /></Field><Field label="Server address"><Input name="url" defaultValue={editing?.url} type="url" required /></Field></div>
    {editing ? <section className="space-y-2"><p className="font-medium">Allowed tools</p>{editing.tools.map((tool) => <Label key={tool.name} className="flex items-center gap-2"><Checkbox checked={props.allowedTools === null || props.allowedTools.includes(tool.name)} onCheckedChange={(checked) => props.onAllowedTools(toggleAllowed(editing, props.allowedTools, tool.name, checked === true))} />{tool.name} — {tool.description}</Label>)}</section> : null}
    <section className="space-y-2"><p className="font-medium">Authentication</p>{editing ? <><p className="text-sm text-muted-foreground">Saved: {editing.secret_header_names.join(", ") || "None"}</p><Label className="flex items-center gap-2"><Checkbox checked={props.replaceSecrets} onCheckedChange={(checked) => props.onReplaceSecrets(checked === true)} />Replace all secret headers</Label>{props.replaceSecrets ? <p className="text-sm text-warning">Saving will replace every secret value. You can rename or remove headers, or leave the list empty to clear all secrets.</p> : null}</> : null}{!editing || props.replaceSecrets ? <HeaderRowsEditor secret rows={props.secretHeaders} onChange={props.onSecretHeaders} /> : null}</section>
    <Button type="submit">Save connection</Button>
  </form></CardContent></Card>;
}
function toggleAllowed(server: McpServer, current: string[] | null, name: string, checked: boolean) { const selected = current ?? server.tools.map((tool) => tool.name); return checked ? [...new Set([...selected, name])] : selected.filter((item) => item !== name); }
function Field({ label, children }: { label: string; children: React.ReactNode }) { return <Label className="grid gap-2">{label}{children}</Label>; }
