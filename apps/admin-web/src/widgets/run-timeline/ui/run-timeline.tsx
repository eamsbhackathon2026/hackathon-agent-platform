import { AlertCircle, CheckCircle2 } from "lucide-react";

import { layoutTimeline, type RunStep } from "@/entities/run-step";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/shared/ui";

export function RunTimeline({ steps }: { steps: RunStep[] }) {
  const rows = layoutTimeline(steps);
  if (!rows.length) return <p className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">No processing steps to display.</p>;

  return <div className="space-y-4">
    {rows.map((row) => <div key={row.id} className="grid gap-2 md:grid-cols-[12rem_1fr_5rem]" style={{ paddingLeft: `${Math.min(row.depth, 4) * 12}px` }}>
      <div className="flex items-center gap-2 text-sm font-medium">{row.status === "ok" ? <CheckCircle2 className="size-4 text-success" /> : <AlertCircle className="size-4 text-destructive" />}{row.label}</div>
      <div className="relative h-5 overflow-hidden rounded bg-muted" aria-label={`${row.label}, ${row.durationText}`}>
        <div className={`absolute top-1 h-3 rounded ${row.status === "ok" ? "bg-primary" : "bg-destructive"}`} style={{ left: `${row.offsetPct}%`, width: `${Math.min(row.widthPct, 100 - row.offsetPct)}%` }} />
      </div>
      <span className="text-right text-xs text-muted-foreground">{row.durationText}</span>
      {(row.step.error_message || Object.keys(row.step.attributes).length > 0) ? <Collapsible className="md:col-start-2 md:col-span-2"><CollapsibleTrigger className="text-xs underline">Technical details</CollapsibleTrigger><CollapsibleContent className="mt-2 overflow-auto rounded bg-muted p-3 text-xs"><pre>{JSON.stringify({ details: row.step.attributes, error: row.step.error_message }, null, 2)}</pre></CollapsibleContent></Collapsible> : null}
    </div>)}
  </div>;
}
