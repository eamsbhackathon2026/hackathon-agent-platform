import type { ApiConnection } from "@/entities/api-connection";
import { type HeaderRow } from "@/shared/lib";
import { Button, Checkbox, DialogFooter, HeaderRowsEditor, Input, Label } from "@/shared/ui";

type Props = {
  editing: ApiConnection | null;
  publicHeaders: HeaderRow[];
  secretHeaders: HeaderRow[];
  replaceSecrets: boolean;
  onPublicHeaders: (rows: HeaderRow[]) => void;
  onSecretHeaders: (rows: HeaderRow[]) => void;
  onReplaceSecrets: (replace: boolean) => void;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
  onCancel: () => void;
};

export function ApiConnectionForm(props: Props) {
  const { editing } = props;
  const showSecretEditor = !editing || props.replaceSecrets;
  return <form className="space-y-5" onSubmit={props.onSubmit}>
    <div className="grid gap-4 md:grid-cols-2">
      <Field label="Display name"><Input name="display_name" defaultValue={editing?.display_name} required /></Field>
      <Field label="Short name"><Input name="slug" defaultValue={editing?.slug} pattern="[a-zA-Z0-9_-]+" required /></Field>
      <div className="md:col-span-2"><Field label="Base URL"><Input name="base_url" defaultValue={editing?.base_url} type="url" placeholder="https://api.example.com/v1" required /></Field></div>
    </div>
    <p className="text-sm text-muted-foreground">Tools using this connection only need an operation path, such as <code>/orders/&#123;order_id&#125;</code>.</p>
    <section className="space-y-2"><h3 className="font-medium">Shared public headers</h3><HeaderRowsEditor rows={props.publicHeaders} onChange={props.onPublicHeaders} /></section>
    <section className="space-y-2"><h3 className="font-medium">Shared secret headers</h3>
      {editing ? <><p className="text-sm text-muted-foreground">Saved: {editing.secret_header_names.join(", ") || "None"}</p><Label className="flex items-center gap-2"><Checkbox checked={props.replaceSecrets} onCheckedChange={(checked) => props.onReplaceSecrets(checked === true)} />Replace all secret headers</Label>{props.replaceSecrets ? <p className="text-sm text-warning">Saving will replace every shared secret value. Leave the list empty to clear them.</p> : null}</> : null}
      {showSecretEditor ? <HeaderRowsEditor secret rows={props.secretHeaders} onChange={props.onSecretHeaders} /> : null}
    </section>
    <DialogFooter><Button type="button" variant="outline" onClick={props.onCancel}>Cancel</Button><Button type="submit">Save connection</Button></DialogFooter>
  </form>;
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <Label className="grid gap-2">{label}{children}</Label>;
}
