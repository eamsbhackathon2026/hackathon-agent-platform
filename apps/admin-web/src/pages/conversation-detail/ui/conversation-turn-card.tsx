import { useQuery } from "@tanstack/react-query";
import { ArrowRight, ChevronDown, CircleDashed } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router";

import { isRunActive, runStatusLabel, runStatusVariant, stopReasonToAction, type Run } from "@/entities/run";
import { runStepQueries } from "@/entities/run-step";
import { formatDate, formatDuration } from "@/shared/lib";
import { Alert, AlertDescription, AlertTitle, Badge, Button, Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/shared/ui";
import { AgentLoop } from "@/widgets/agent-loop";
import { ChatMarkdown } from "@/widgets/chat-thread";
import type { ConversationTurn } from "../model/group-turns";

function elapsed(from: string, to: string) {
  return Math.max(0, new Date(to).valueOf() - new Date(from).valueOf());
}

/** How long the person waited: from sending the message until the request finished or the answer was saved. */
function responseTime(turn: ConversationTurn, run: Run | undefined) {
  if (!turn.request) return null;
  const end = run?.finished_at ?? turn.answer?.created_at;
  return end ? elapsed(turn.request.created_at, end) : null;
}

export function ConversationTurnCard({ turn, run, number }: { turn: ConversationTurn; run: Run | undefined; number: number }) {
  const [loopOpen, setLoopOpen] = useState(false);
  const active = isRunActive(run?.status);
  const terminal = Boolean(run && !active);
  const steps = useQuery({ ...runStepQueries.list(turn.runId ?? ""), enabled: loopOpen && terminal && Boolean(turn.runId) });
  const problem = run ? stopReasonToAction(run.error?.code, run.agent_id) : null;
  const waited = responseTime(turn, run);
  const units = run && (run.usage.input_tokens !== null || run.usage.output_tokens !== null) ? (run.usage.input_tokens ?? 0) + (run.usage.output_tokens ?? 0) : null;

  return <li className="rounded-2xl border bg-card p-4 sm:p-5" aria-label={`Turn ${number}`}>
    <div className="flex flex-wrap items-center gap-2">
      <Badge variant="secondary">Turn {number}</Badge>
      {run ? <Badge variant={runStatusVariant(run.status)}>{runStatusLabel[run.status]}</Badge> : null}
      {run ? <span className="text-xs text-muted-foreground tabular-nums">{run.iterations} model {run.iterations === 1 ? "pass" : "passes"} · {units === null ? "usage not reported" : `${units.toLocaleString("en-US")} processing units`}</span> : null}
      {turn.runId ? <Button asChild variant="ghost" size="sm" className="ml-auto"><Link to={`/activity/${turn.runId}`}>Open details <ArrowRight /></Link></Button> : null}
    </div>

    <div className="mt-4 space-y-3">
      {turn.request ? <div className="flex flex-col items-end gap-1">
        <div className="min-w-0 max-w-[85%] whitespace-pre-wrap break-words rounded-2xl bg-primary px-4 py-3 text-sm text-primary-foreground [overflow-wrap:anywhere]">{turn.request.content}</div>
        <p className="text-xs text-muted-foreground">Sent {formatDate(turn.request.created_at)}</p>
      </div> : null}

      {turn.runId ? <Collapsible open={loopOpen} onOpenChange={setLoopOpen}>
        <CollapsibleTrigger asChild><Button variant="outline" size="sm" className="group">Agent loop <ChevronDown className="transition-transform group-data-[state=open]:rotate-180" /></Button></CollapsibleTrigger>
        <CollapsibleContent className="mt-3">
          {steps.isError ? <Alert variant="destructive" className="mb-3"><AlertTitle>Unable to load the timing of each step</AlertTitle><AlertDescription>The decisions below are still complete. <button className="font-medium underline" onClick={() => void steps.refetch()}>Try again</button></AlertDescription></Alert> : null}
          <AgentLoop messages={turn.messages} steps={steps.data ?? []} active={active} totalIterations={run?.iterations ?? 0} fallbackRequest={run?.input.message ?? turn.request?.content ?? ""} />
        </CollapsibleContent>
      </Collapsible> : null}

      {turn.answer ? <div className="flex flex-col items-start gap-1">
        <div className="min-w-0 max-w-[85%] break-words rounded-2xl bg-muted px-4 py-3 text-sm [overflow-wrap:anywhere]"><ChatMarkdown>{turn.answer.content}</ChatMarkdown></div>
        <p className="text-xs text-muted-foreground">Answered {formatDate(turn.answer.created_at)}{waited === null ? "" : ` · responded in ${formatDuration(waited)}`}</p>
      </div> : active ? <p className="flex items-center gap-2 text-sm text-muted-foreground" aria-live="polite"><CircleDashed className="size-4 animate-spin" />Waiting for the answer…</p> : run ? <p className="text-sm text-muted-foreground">No answer was saved for this turn{waited === null ? "." : `. It ended after ${formatDuration(waited)}.`}</p> : null}

      {problem ? <Alert variant="destructive"><AlertTitle>{problem.title}</AlertTitle><AlertDescription>{problem.description} {problem.action?.to ? <Link className="font-medium underline" to={problem.action.to}>{problem.action.label}</Link> : null}</AlertDescription></Alert> : null}
    </div>
  </li>;
}
