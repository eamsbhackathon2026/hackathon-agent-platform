import type { ApiConnection } from "@/entities/api-connection";
import type { HttpTool, ToolParam } from "@/entities/tool";
import { useState } from "react";
import type { HeaderRow } from "@/shared/lib";
import { Button, Checkbox, HeaderRowsEditor, Input, Label, Textarea } from "@/shared/ui";
import { ConnectionSummary } from "./connection-summary";
import { HttpToolBuilder } from "./http-tool-builder";

type Props = {
  editing: HttpTool | null;
  connections: ApiConnection[];
  publicHeaders: HeaderRow[];
  secretHeaders: HeaderRow[];
  replaceSecrets: boolean;
  onParams: (params: ToolParam[]) => void;
  onPublicHeaders: (rows: HeaderRow[]) => void;
  onSecretHeaders: (rows: HeaderRow[]) => void;
  onReplaceSecrets: (replace: boolean) => void;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
  onCancel: () => void;
};

export function ToolConfigForm(props: Props) {
  const { editing } = props;
  const [connectionId, setConnectionId] = useState(editing?.connection_id ?? "");
  // Method and path are state so the builder can enforce GET rules and the connection
  // summary can preview the resolved address as they are typed.
  const [method, setMethod] = useState<HttpTool["method"]>(editing?.method ?? "GET");
  const [path, setPath] = useState(editing?.url_template ?? "");
  const connection = props.connections.find((item) => item.id === connectionId);
  const showSecretEditor = !editing || props.replaceSecrets;
  return <form key={editing?.id ?? "new"} className="space-y-6" onSubmit={props.onSubmit}>
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Display name"><Input name="display_name" defaultValue={editing?.display_name} required /></Field>
        <Field label="Short name"><Input name="slug" defaultValue={editing?.slug} pattern="[a-zA-Z0-9_-]+" required /></Field>
        <Field label="Progress label"><Input name="step_label" defaultValue={editing?.step_label} maxLength={80} placeholder="xem lịch sử giao dịch của anh/chị" /></Field>
        <p className="self-end text-sm text-muted-foreground">People chatting with the assistant read this while the tool runs. Write a verb phrase with no subject and no leading capital, in their language: the screen builds both “Em đang xem lịch sử giao dịch…” and “Em đã xem lịch sử giao dịch.” from it. Leave it empty to show the display name instead.</p>
        <Field label="Method"><select name="method" value={method} onChange={(event) => setMethod(event.target.value as HttpTool["method"])} className="h-10 rounded-md border bg-background px-3"><option>GET</option><option>POST</option><option>PUT</option><option>PATCH</option><option>DELETE</option></select></Field>
        <Field label="API connection"><select name="connection_id" value={connectionId} onChange={(event) => setConnectionId(event.target.value)} className="h-10 rounded-md border bg-background px-3"><option value="">Direct URL</option>{props.connections.map((item) => <option key={item.id} value={item.id}>{item.display_name} — {item.base_url}</option>)}</select></Field>
        <Field label={connectionId ? "Operation path" : "Address"}><Input name="url_template" value={path} onChange={(event) => setPath(event.target.value)} type={connectionId ? "text" : "url"} placeholder={connectionId ? "/orders/{order_id}" : "https://example.com/orders/{order_id}"} required /></Field>
      </div>
      {connection ? <ConnectionSummary connection={connection} path={path} /> : <p className="text-sm text-muted-foreground">Use a full URL. Choose an API connection to reuse a base URL and credentials.</p>}
      <Field label="Description"><Textarea name="description" defaultValue={editing?.description} rows={4} placeholder="What this tool does and when the assistant should use it. The assistant reads this text." /></Field>
      <HttpToolBuilder initialParams={editing?.params} method={method} onChange={props.onParams} />
      <section className="space-y-2"><h3 className="font-medium">Public headers</h3><HeaderRowsEditor rows={props.publicHeaders} onChange={props.onPublicHeaders} /></section>
      <section className="space-y-2"><h3 className="font-medium">Operation-specific secret headers</h3><p className="text-sm text-muted-foreground">These override shared connection headers only for this tool.</p>{editing ? <><p className="text-sm text-muted-foreground">Saved: {editing.secret_header_names.join(", ") || "None"}</p><Label className="flex items-center gap-2"><Checkbox checked={props.replaceSecrets} onCheckedChange={(checked) => props.onReplaceSecrets(checked === true)} />Replace all secret headers</Label>{props.replaceSecrets ? <p className="text-sm text-warning">Saving will replace every secret value. You can rename or remove headers, or leave the list empty to clear all secrets.</p> : null}</> : null}{showSecretEditor ? <HeaderRowsEditor secret rows={props.secretHeaders} onChange={props.onSecretHeaders} /> : null}</section>
      <div className="flex flex-wrap justify-end gap-2"><Button type="button" variant="outline" onClick={props.onCancel}>Cancel</Button><Button type="submit">Save tool</Button></div>
    </form>;
}

function Field({ label, children }: { label: string; children: React.ReactNode }) { return <Label className="grid gap-2">{label}{children}</Label>; }
