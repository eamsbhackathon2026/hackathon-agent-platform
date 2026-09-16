import { Plus, Trash2 } from "lucide-react";
import { Button, Checkbox, Input, Label } from "@/shared/ui";
import type { ToolParamField } from "@/entities/tool";

type Props = { inputName: string; fields: ToolParamField[]; onChange: (fields: ToolParamField[]) => void };

/**
 * Lets an editor name the values inside a group input. Declaring them tells the model
 * which keys to fill; leaving the list empty keeps the group free-form.
 */
export function ObjectFieldsEditor({ inputName, fields, onChange }: Props) {
  const label = inputName || "this group";
  return <div className="col-span-full grid gap-2 rounded-md border border-dashed p-3">
    <div className="flex items-center justify-between gap-2">
      <p className="text-sm text-muted-foreground">Values inside {label}. Leave empty to accept any content.</p>
      <Button type="button" size="sm" variant="outline" onClick={() => onChange([...fields, { name: "", type: "boolean", description: "", required: false }])}><Plus /> Add value</Button>
    </div>
    {fields.map((field, index) => <div className="grid gap-2 md:grid-cols-[1fr_1fr_0.8fr_auto_auto]" key={index}>
      <Input aria-label={`Value name ${index + 1} in ${label}`} placeholder="screen_sharing" value={field.name} onChange={(event) => onChange(fields.map((item, position) => position === index ? { ...item, name: event.target.value } : item))} />
      <Input aria-label={`Value description ${index + 1} in ${label}`} placeholder="Screen sharing is active" value={field.description} onChange={(event) => onChange(fields.map((item, position) => position === index ? { ...item, description: event.target.value } : item))} />
      <select aria-label={`Value type ${index + 1} in ${label}`} className="rounded-md border bg-background px-3" value={field.type} onChange={(event) => onChange(fields.map((item, position) => position === index ? { ...item, type: event.target.value as ToolParamField["type"] } : item))}><option value="string">Text</option><option value="number">Number</option><option value="integer">Integer</option><option value="boolean">Yes / No</option></select>
      <Label className="flex items-center gap-2"><Checkbox checked={field.required} onCheckedChange={(checked) => onChange(fields.map((item, position) => position === index ? { ...item, required: checked === true } : item))} />Required</Label>
      <Button type="button" size="icon" variant="ghost" aria-label={`Delete value ${index + 1} in ${label}`} onClick={() => onChange(fields.filter((_, position) => position !== index))}><Trash2 /></Button>
    </div>)}
  </div>;
}
