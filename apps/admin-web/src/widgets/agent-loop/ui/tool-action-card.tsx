import { AlertCircle, CheckCircle2, ChevronDown, LoaderCircle, Wrench, XCircle } from "lucide-react";

import { formatDuration } from "@/shared/lib";
import { Badge, Button, Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/shared/ui";
import type { AgentLoopToolAction } from "../model/build-agent-loop";

function formatted(value: unknown) {
  if (typeof value !== "string") return JSON.stringify(value, null, 2);
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

function resultTruncated(action: AgentLoopToolAction) {
  return action.span?.attributes.result_truncated === true;
}

export function ToolActionCard({ action, active }: { action: AgentLoopToolAction; active: boolean }) {
  const failed = action.result?.is_error || action.span?.status === "error";
  const observed = Boolean(action.result);
  const status = failed ? "Failed" : observed ? "Observed" : active ? "Waiting" : "No observation";
  const StatusIcon = failed ? XCircle : observed ? CheckCircle2 : active ? LoaderCircle : AlertCircle;

  return <div className="rounded-xl border bg-card p-4">
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="flex min-w-0 items-start gap-3">
        <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-secondary text-foreground"><Wrench className="size-4" /></span>
        <div className="min-w-0">
          <p className="break-all text-sm font-semibold">{action.call.name}</p>
          <p className="mt-1 text-sm text-muted-foreground">{!observed && !active ? "The activity ended before an observation was saved for this tool action." : "The model supplied inputs, then waited for this observation before continuing."}</p>
        </div>
      </div>
      <Badge variant={failed ? "destructive" : observed ? "success" : "warning"} className="gap-1.5">
        <StatusIcon className={active && !failed && !observed ? "size-3.5 animate-spin" : "size-3.5"} />{status}
      </Badge>
    </div>
    <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
      {action.span ? <span>{formatDuration(action.span.duration_ms)}</span> : null}
      {resultTruncated(action) ? <span>Observation shortened for safe storage</span> : null}
    </div>
    <Collapsible className="mt-2">
      <CollapsibleTrigger asChild>
        <Button variant="ghost" size="sm" className="group -ml-3" aria-label={`View inputs and observation for ${action.call.name}`}>
          View inputs and observation <ChevronDown className="transition-transform group-data-[state=open]:rotate-180" />
        </Button>
      </CollapsibleTrigger>
      <CollapsibleContent className="space-y-3 pt-2">
        <p className="text-xs text-muted-foreground">These details can contain sensitive conversation data.</p>
        <div>
          <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Inputs</p>
          <pre className="max-h-72 overflow-auto rounded-lg bg-muted p-3 text-xs whitespace-pre-wrap break-words">{formatted(action.call.arguments)}</pre>
        </div>
        <div>
          <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Observation</p>
          <pre className="max-h-72 overflow-auto rounded-lg bg-muted p-3 text-xs whitespace-pre-wrap break-words">{action.result ? formatted(action.result.content) : active ? "Waiting for the tool to return an observation." : "No observation was saved before this activity ended."}</pre>
        </div>
      </CollapsibleContent>
    </Collapsible>
  </div>;
}
