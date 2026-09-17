import { Upload } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router";
import { Badge, Button, Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, Input, Label, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui";
import { importTools, previewToolImport, type ToolBundle, type ToolImportItem, type ToolImportResult } from "../api/tool-transfer";
import { readBundleFile, transferErrorMessage } from "../lib/bundle-file";
import { importDecisions, itemKey, type ImportChoice as Choice } from "../lib/import-decisions";

type Step = { name: "choose" } | { name: "review"; bundle: ToolBundle; items: ToolImportItem[] } | { name: "done"; result: ToolImportResult };

const kindLabel = { connection: "API connection", tool: "HTTP tool" } as const;

export function ToolImportDialog({ open, onOpenChange, onImported }: { open: boolean; onOpenChange: (open: boolean) => void; onImported: () => Promise<unknown> }) {
  const [step, setStep] = useState<Step>({ name: "choose" });
  const [file, setFile] = useState<File | null>(null);
  const [choices, setChoices] = useState<Record<string, Choice>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const close = (next: boolean) => { if (!next) { setStep({ name: "choose" }); setFile(null); setChoices({}); setError(""); } onOpenChange(next); };

  const review = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!file) return;
    setBusy(true); setError("");
    try {
      const bundle = await readBundleFile(file);
      setStep({ name: "review", bundle, items: await previewToolImport(bundle) });
    } catch (cause) {
      setError(transferErrorMessage(cause, "Unable to read this file. Check that it was created with Export tools."));
    } finally { setBusy(false); }
  };

  const confirm = async () => {
    if (step.name !== "review") return;
    setBusy(true); setError("");
    try {
      const result = await importTools(step.bundle, importDecisions(step.items, choices));
      await onImported();
      setStep({ name: "done", result });
    } catch (cause) {
      setError(transferErrorMessage(cause, "Nothing was imported. Review the list and try again."));
    } finally { setBusy(false); }
  };

  const writable = step.name === "review" ? step.items.filter((item) => item.status === "new" || (item.status === "conflict" && choices[itemKey(item)] === "overwrite")).length : 0;

  return <Dialog open={open} onOpenChange={close}><DialogContent className="max-h-[90vh] max-w-3xl overflow-y-auto">
    <DialogHeader><DialogTitle>Import tools</DialogTitle><DialogDescription>Bring in HTTP tools and the API connections they use from a file created with Export tools. Secret values are never included in the file.</DialogDescription></DialogHeader>
    {step.name === "choose" ? <form className="space-y-3" onSubmit={review}>
      <Label className="grid gap-2">Tools file<Input accept=".json,application/json" disabled={busy} name="tools-file" onChange={(event) => setFile(event.currentTarget.files?.[0] ?? null)} type="file" /></Label>
      <Button disabled={busy || !file} type="submit"><Upload />{busy ? "Checking…" : "Review file"}</Button>
    </form> : null}
    {step.name === "review" ? <div className="space-y-4">
      <Table><TableHeader><TableRow><TableHead>Item</TableHead><TableHead>Status</TableHead><TableHead>What happens</TableHead></TableRow></TableHeader><TableBody>{step.items.map((item, index) => <TableRow key={`${itemKey(item)}:${index}`}>
        <TableCell><p className="font-medium">{item.display_name || item.slug}</p><p className="text-xs text-muted-foreground">{kindLabel[item.kind]}</p></TableCell>
        <TableCell>{item.status === "new" ? <Badge variant="success">New</Badge> : item.status === "conflict" ? <Badge variant="warning">Already exists</Badge> : <Badge variant="destructive">Can’t import</Badge>}</TableCell>
        <TableCell>{item.status === "new" ? "Will be added" : item.status === "conflict" ? <select aria-label={`Choice for ${item.display_name || item.slug}`} className="h-9 rounded-md border bg-background px-3" value={choices[itemKey(item)] ?? "skip"} onChange={(event) => setChoices((current) => ({ ...current, [itemKey(item)]: event.target.value as Choice }))}><option value="skip">Keep the existing one</option><option value="overwrite">Replace with the file version</option></select> : <div className="space-y-1 text-sm"><p>Will be skipped</p>{item.fields.map((field, fieldIndex) => <p className="text-destructive" key={fieldIndex}>{field.message}</p>)}</div>}</TableCell>
      </TableRow>)}</TableBody></Table>
      {step.items.some((item) => item.kind === "connection" && choices[itemKey(item)] === "overwrite") ? <p className="text-sm text-warning">Replacing an API connection changes its address and headers for every tool that uses it.</p> : null}
      <div className="flex gap-2"><Button disabled={busy || writable === 0} onClick={() => void confirm()}>{busy ? "Importing…" : `Import ${writable} item${writable === 1 ? "" : "s"}`}</Button><Button variant="ghost" onClick={() => { setStep({ name: "choose" }); setChoices({}); setError(""); }}>Choose another file</Button></div>
      {writable === 0 ? <p className="text-sm text-muted-foreground">Nothing to import yet. Choose “Replace with the file version” for items you want to update.</p> : null}
    </div> : null}
    {step.name === "done" ? <div className="space-y-3">
      <p>Imported: {step.result.created} added, {step.result.overwritten} replaced, {step.result.skipped} skipped.</p>
      {step.result.needs_secrets.length ? <div className="space-y-2 rounded-xl border p-3"><p className="font-medium">Add the secret values again</p><p className="text-sm text-muted-foreground">These items need their secret headers before they can call the target system.</p><ul className="space-y-1 text-sm">{step.result.needs_secrets.map((item) => <li key={item.id}>{item.kind === "tool" ? <Button asChild variant="link" className="h-auto p-0"><Link to={`/tools/${item.id}/edit`}>{item.display_name}</Link></Button> : <span className="font-medium">{item.display_name}</span>} — {item.header_names.join(", ")}{item.kind === "connection" ? " (open the API connections tab and choose Edit connection)" : ""}</li>)}</ul></div> : null}
      <Button onClick={() => close(false)}>Done</Button>
    </div> : null}
    {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
  </DialogContent></Dialog>;
}
