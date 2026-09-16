import { Plus, Trash2 } from "lucide-react";
import { Button, Input } from "@/shared/ui";
import type { HeaderRow } from "@/shared/lib/header-config";

type Props = { rows: HeaderRow[]; onChange: (rows: HeaderRow[]) => void; secret?: boolean };
export function HeaderRowsEditor({ rows, onChange, secret = false }: Props) {
  const update = (id: string, field: "name" | "value", value: string) => onChange(rows.map((row) => row.id === id ? { ...row, [field]: value } : row));
  return <div className="space-y-2">{rows.map((row, index) => <div key={row.id} className="grid grid-cols-[1fr_1fr_auto] gap-2"><Input aria-label={`Header name ${index + 1}`} value={row.name} onChange={(event) => update(row.id, "name", event.target.value)} /><Input aria-label={`Header value ${index + 1}`} value={row.value} type={secret ? "password" : "text"} autoComplete="off" onChange={(event) => update(row.id, "value", event.target.value)} /><Button type="button" size="icon" variant="ghost" aria-label="Delete header" onClick={() => onChange(rows.filter((item) => item.id !== row.id))}><Trash2 /></Button></div>)}<Button type="button" size="sm" variant="outline" onClick={() => onChange([...rows, { id: crypto.randomUUID(), name: "", value: "" }])}><Plus />Add row</Button></div>;
}
