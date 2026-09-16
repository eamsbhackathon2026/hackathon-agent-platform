import { BrainCircuit, ChevronDown, CircleDashed, Info, MessageSquareText } from "lucide-react";

import type { ConversationMessage } from "@/entities/conversation";
import type { RunStep } from "@/entities/run-step";
import { formatDuration } from "@/shared/lib";
import { Badge, Button, Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/shared/ui";
import { buildAgentLoop } from "../model/build-agent-loop";
import { ToolActionCard } from "./tool-action-card";

function signalLabel(value: unknown, usesTools: boolean) {
  if (typeof value !== "string" || !value) return "Not reported";
  if (usesTools) return `${value.replaceAll("_", " ")} · tool call emitted`;
  if (value.toLowerCase() === "tool_calls") return "Requested tools";
  if (["stop", "end_turn"].includes(value.toLowerCase())) return "Returned an answer";
  if (["length", "max_tokens"].includes(value.toLowerCase())) return "Reached the response limit";
  return value.replaceAll("_", " ");
}

function usageLabel(step: RunStep) {
  const input = step.usage.input_tokens;
  const output = step.usage.output_tokens;
  if (input === null || output === null) return "Not reported";
  return `${(input + output).toLocaleString("en-US")} processing units`;
}

export function AgentLoop({
  messages,
  steps,
  active,
  totalIterations,
  fallbackRequest,
}: {
  messages: ConversationMessage[];
  steps: RunStep[];
  active: boolean;
  totalIterations: number;
  fallbackRequest: string;
}) {
  const loop = buildAgentLoop(messages, steps);
  const missingDecisions = Math.max(loop.unrecordedModelPasses, active ? 0 : totalIterations - loop.iterations.length);

  return <div className="space-y-5">
    <div className="flex gap-3 rounded-xl border bg-brand-soft/45 p-4 text-sm">
      <Info className="mt-0.5 size-4 shrink-0 text-primary" />
      <p><span className="font-semibold">How to read this:</span> each iteration shows saved messages and operational signals, followed by tool actions and observations. Private internal reasoning is not shown in this view.</p>
    </div>

    <div className="flex gap-3 rounded-xl bg-muted/70 p-4">
      <MessageSquareText className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <div className="min-w-0"><p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Request</p><p className="mt-1 whitespace-pre-wrap break-words text-sm">{loop.request?.content || fallbackRequest}</p></div>
    </div>

    {loop.contextPreparations.length ? <p className="text-sm text-muted-foreground">The system prepared a shorter conversation context {loop.contextPreparations.length} {loop.contextPreparations.length === 1 ? "time" : "times"} before a model pass.</p> : null}
    {missingDecisions > 0 ? <p className="rounded-xl border border-warning/25 bg-warning-soft p-3 text-sm text-warning">The timing trace contains {missingDecisions} additional model {missingDecisions === 1 ? "pass" : "passes"} that did not create a saved conversational decision.</p> : null}

    {loop.iterations.length ? <ol className="space-y-0">
      {loop.iterations.map((iteration, index) => {
        const usesTools = iteration.actions.length > 0;
        const last = index === loop.iterations.length - 1;
        return <li key={iteration.message.id} className="relative grid grid-cols-[2.5rem_minmax(0,1fr)] gap-3 pb-6 last:pb-0">
          {!last || active ? <span aria-hidden className="absolute bottom-0 left-[1.22rem] top-10 w-px bg-border" /> : null}
          <span className="relative z-10 grid size-10 place-items-center rounded-full border bg-card text-sm font-semibold shadow-sm">{iteration.number}</span>
          <div className="min-w-0 rounded-2xl border bg-card p-4 sm:p-5">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div><div className="flex flex-wrap items-center gap-2"><Badge variant="secondary">Iteration {iteration.number}</Badge><Badge variant={usesTools ? "warning" : "success"}>{usesTools ? "Tool decision" : "Answer decision"}</Badge></div><h3 className="mt-3 font-semibold">{usesTools ? "Gather information with tools" : "Produce the answer"}</h3></div>
              <BrainCircuit className="size-5 text-muted-foreground" />
            </div>
            <p className="mt-2 text-sm text-muted-foreground">{usesTools ? `Observable signal: the model requested ${iteration.actions.length} tool ${iteration.actions.length === 1 ? "action" : "actions"} instead of ending the activity.` : "Observable signal: the model returned an answer without requesting another tool."}</p>
            {iteration.message.content.trim() ? <div className="mt-4 rounded-xl bg-muted/70 p-3 text-sm"><p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Public model message</p><p className="mt-1 whitespace-pre-wrap break-words leading-6">{iteration.message.content}</p></div> : null}
            {iteration.actions.length ? <div className="mt-4 space-y-3">{iteration.actions.map((action) => <ToolActionCard key={action.call.id} action={action} active={active} />)}</div> : null}
            {iteration.span ? <Collapsible className="mt-3 border-t pt-2">
              <CollapsibleTrigger asChild><Button variant="ghost" size="sm" className="group -ml-3">View model signal <ChevronDown className="transition-transform group-data-[state=open]:rotate-180" /></Button></CollapsibleTrigger>
              <CollapsibleContent className="grid gap-3 pt-2 text-sm sm:grid-cols-2 xl:grid-cols-4">
                <div><p className="text-xs text-muted-foreground">Model</p><p className="mt-1 break-all font-medium">{iteration.span.model ?? "Not reported"}</p></div>
                <div><p className="text-xs text-muted-foreground">Time</p><p className="mt-1 font-medium tabular-nums">{formatDuration(iteration.span.duration_ms)}</p></div>
                <div><p className="text-xs text-muted-foreground">Usage</p><p className="mt-1 font-medium tabular-nums">{usageLabel(iteration.span)}</p></div>
                <div><p className="text-xs text-muted-foreground">Provider finish signal</p><p className="mt-1 font-medium capitalize">{signalLabel(iteration.span.attributes.finish_reason, usesTools)}</p></div>
              </CollapsibleContent>
            </Collapsible> : null}
          </div>
        </li>;
      })}
    </ol> : active ? <div className="flex items-center gap-3 rounded-xl border border-dashed p-5 text-sm text-muted-foreground" aria-live="polite"><CircleDashed className="size-5 animate-spin" />Waiting for the first model decision…</div> : <div className="rounded-xl border border-dashed p-5 text-sm text-muted-foreground">No model decision was saved for this activity. Check the timing trace and error summary below.</div>}

    {active && loop.iterations.length ? <div className="flex items-center gap-3 pl-[3.25rem] text-sm text-muted-foreground" aria-live="polite"><CircleDashed className="size-4 animate-spin" />Waiting for the next decision…</div> : null}
  </div>;
}
