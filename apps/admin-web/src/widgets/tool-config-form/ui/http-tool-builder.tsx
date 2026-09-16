import { useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Button, Checkbox, Input, Label } from "@/shared/ui";
import { isStructuredParam, type ToolParam } from "@/entities/tool";
import { rowsToParams, withItemType, type ParameterRow } from "../model/tool-params";
import { ObjectFieldsEditor } from "./object-fields-editor";

type Props = { initialParams?: ToolParam[] | undefined; method?: ToolMethod; onChange?: (params: ReturnType<typeof rowsToParams>) => void };
type ToolMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

export function HttpToolBuilder({ initialParams = [], method = "POST", onChange }: Props) {
  const [rows, setRows] = useState<ParameterRow[]>(() => initialParams.map((param) => ({ ...param, rowId: crypto.randomUUID() })));
  const [error, setError] = useState("");
  // A GET sends every input in the query string, where nested JSON cannot be represented.
  const allowsStructured = method !== "GET";
  const blockedRows = allowsStructured ? [] : rows.filter((row) => isStructuredParam(row.type));
  const update = (next: ParameterRow[]) => { setRows(next); try { onChange?.(rowsToParams(next)); setError(""); } catch (cause) { setError(cause instanceof Error ? cause.message : "The input is invalid"); } };
  return <div className="space-y-3"><div className="flex items-center justify-between"><div><h3 className="font-medium">Assistant inputs</h3><p className="text-sm text-muted-foreground">Define each value with a clear name and description.</p></div><Button type="button" variant="outline" onClick={() => update([...rows, { rowId: crypto.randomUUID(), name: "", type: "string", description: "", required: false, in: "query" }])}><Plus /> Add row</Button></div>
    {rows.map((row, index) => <div className="grid gap-2 rounded-lg border p-3 md:grid-cols-[1fr_1fr_0.8fr_1fr_auto_auto]" key={row.rowId}>
      <Input aria-label={`Input name ${index + 1}`} placeholder="order_id" value={row.name} onChange={(e) => update(rows.map((item) => item.rowId === row.rowId ? { ...item, name: e.target.value } : item))} />
      <Input aria-label={`Description ${index + 1}`} placeholder="Order ID" value={row.description} onChange={(e) => update(rows.map((item) => item.rowId === row.rowId ? { ...item, description: e.target.value } : item))} />
      <select aria-label={`Input type ${index + 1}`} className="rounded-md border bg-background px-3" value={row.type} onChange={(e) => { const type = e.target.value as ParameterRow["type"]; update(rows.map((item) => item.rowId === row.rowId ? { ...item, type, in: isStructuredParam(type) ? "body" : item.in } : item)); }}><option value="string">Text</option><option value="number">Number</option><option value="integer">Integer</option><option value="boolean">Yes / No</option>{allowsStructured || isStructuredParam(row.type) ? <><option value="object">Group of values</option><option value="array">List of values</option></> : null}</select>
      <select aria-label={`Input location ${index + 1}`} className="rounded-md border bg-background px-3" value={row.in} disabled={isStructuredParam(row.type)} onChange={(e) => update(rows.map((item) => item.rowId === row.rowId ? { ...item, in: e.target.value as ParameterRow["in"] } : item))}>{isStructuredParam(row.type) ? <option value="body">Request body</option> : <><option value="path">Path</option><option value="query">Query string</option><option value="body">Request body</option></>}</select>
      <Label className="flex items-center gap-2"><Checkbox checked={row.required} onCheckedChange={(checked) => update(rows.map((item) => item.rowId === row.rowId ? { ...item, required: checked === true } : item))} />Required</Label>
      <Button type="button" size="icon" variant="ghost" aria-label="Delete row" onClick={() => update(rows.filter((item) => item.rowId !== row.rowId))}><Trash2 /></Button>
      {row.type === "object" ? <ObjectFieldsEditor inputName={row.name} fields={row.fields ?? []} onChange={(fields) => update(rows.map((item) => item.rowId === row.rowId ? { ...item, fields } : item))} /> : null}
      {row.type === "array" ? <Label className="col-span-full grid max-w-xs gap-2 text-sm">Each item is<select aria-label={`Item type ${index + 1}`} className="h-10 rounded-md border bg-background px-3" value={row.item_type ?? ""} onChange={(e) => update(rows.map((item) => item.rowId === row.rowId ? withItemType(item, e.target.value) : item))}><option value="">Anything</option><option value="string">Text</option><option value="number">Number</option><option value="integer">Integer</option><option value="boolean">Yes / No</option><option value="object">Group of values</option></select></Label> : null}
    </div>)}
    {blockedRows.length ? <p role="alert" className="text-sm text-destructive">A GET request cannot send grouped or list inputs. Change the method to POST, or change these inputs to a simple type: {blockedRows.map((row) => row.name || "unnamed").join(", ")}.</p> : null}
    {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
  </div>;
}
