import { useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Clock3, Gauge, GitBranch, Radio } from "lucide-react";
import { Link, useParams } from "react-router";

import { conversationKeys, conversationQueries } from "@/entities/conversation";
import { runQueries, stopReasonToAction, type RunStatus } from "@/entities/run";
import { runStepQueries } from "@/entities/run-step";
import { useAuthSession } from "@/shared/api";
import { formatDate, formatDuration, problemToAction, usePageHeader } from "@/shared/lib";
import { Alert, AlertDescription, AlertTitle, Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Skeleton } from "@/shared/ui";
import { AgentLoop } from "@/widgets/agent-loop";
import { RunTimeline } from "@/widgets/run-timeline";
import { WebhookDeliveryPanel } from "@/widgets/webhook-delivery-panel";

const statusLabel: Record<RunStatus, string> = { queued: "Queued", running: "Processing", succeeded: "Completed", failed: "Failed", cancelled: "Stopped" };

function statusVariant(status: RunStatus) {
  if (status === "succeeded") return "success" as const;
  if (status === "failed") return "destructive" as const;
  if (status === "queued" || status === "running") return "warning" as const;
  return "outline" as const;
}

export function ActivityDetailPage() {
  const { runId = "" } = useParams();
  const queryClient = useQueryClient();
  const wasActive = useRef(false);
  const run = useQuery({ ...runQueries.detail(runId), refetchInterval: (query) => {
    const status = query.state.data?.status;
    return status === "queued" || status === "running" ? 2_000 : false;
  } });
  const active = run.data?.status === "queued" || run.data?.status === "running";
  const terminal = run.data?.status === "succeeded" || run.data?.status === "failed" || run.data?.status === "cancelled";
  const messages = useQuery({
    ...conversationQueries.runMessages(run.data?.session_id ?? "", runId),
    enabled: Boolean(run.data?.session_id),
    refetchInterval: active ? 2_000 : false,
  });
  const steps = useQuery({ ...runStepQueries.list(runId), enabled: terminal });
  const { user } = useAuthSession();
  const canManage = user?.role === "owner" || user?.role === "admin";
  const deliveries = useQuery({ ...runQueries.deliveries(runId), enabled: canManage && terminal });

  useEffect(() => {
    if (wasActive.current && terminal && run.data?.session_id) {
      const queryKey = conversationKeys.runMessages(run.data.session_id, runId);
      void queryClient.cancelQueries({ queryKey, exact: true }).then(() => queryClient.refetchQueries({ queryKey, exact: true, type: "active" }));
    }
    wasActive.current = active;
  }, [active, queryClient, run.data?.session_id, runId, terminal]);

  usePageHeader({ title: "Activity details", description: "Follow each model decision, tool action, and observation in order." });

  if (run.isPending) return <div className="space-y-5"><Skeleton className="h-5 w-28" /><Skeleton className="h-20 w-full" /><Skeleton className="h-32 w-full" /><Skeleton className="h-80 w-full" /></div>;
  if (run.isError || !run.data) {
    const action = problemToAction((run.error as { code?: string })?.code);
    return <Alert variant="destructive"><AlertTitle>{action.title}</AlertTitle><AlertDescription>{action.description} <button className="font-medium underline" onClick={() => void run.refetch()}>{action.action?.label ?? "Try again"}</button></AlertDescription></Alert>;
  }

  const problem = stopReasonToAction(run.data.error?.code, run.data.agent_id);
  const canContinueSession = run.data.source === "playground" && run.data.triggered_by_user_id === user?.id;
  const playgroundUrl = `/playground?agent=${run.data.agent_id}${canContinueSession ? `&session=${run.data.session_id}` : ""}`;
  const elapsed = run.data.started_at && run.data.finished_at ? Math.max(0, new Date(run.data.finished_at).valueOf() - new Date(run.data.started_at).valueOf()) : null;
  const totalUsage = run.data.usage.input_tokens === null || run.data.usage.output_tokens === null ? null : run.data.usage.input_tokens + run.data.usage.output_tokens;

  return <div className="space-y-5">
    <div className="flex flex-wrap items-center gap-3">
      <Link className="inline-flex min-h-11 items-center gap-2 text-sm font-medium hover:underline" to="/activity"><ArrowLeft className="size-4" />Activity</Link>
      <Badge variant={statusVariant(run.data.status)}>{statusLabel[run.data.status]}</Badge>
      {active ? <Badge variant="warning" className="ml-auto gap-1.5" aria-live="polite"><Radio className="size-3.5 animate-pulse" />Live updates</Badge> : null}
    </div>

    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <div className="rounded-2xl border bg-card p-4"><div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground"><Clock3 className="size-4" />Started</div><p className="mt-2 text-sm font-medium">{formatDate(run.data.started_at ?? run.data.created_at)}</p></div>
      <div className="rounded-2xl border bg-card p-4"><div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground"><Gauge className="size-4" />Duration</div><p className="mt-2 text-sm font-medium tabular-nums">{elapsed === null ? active ? "In progress" : "Not available" : formatDuration(elapsed)}</p></div>
      <div className="rounded-2xl border bg-card p-4"><div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground"><GitBranch className="size-4" />Model passes</div><p className="mt-2 text-sm font-medium tabular-nums">{run.data.iterations}</p></div>
      <div className="rounded-2xl border bg-card p-4"><div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground"><Gauge className="size-4" />Processing usage</div><p className="mt-2 text-sm font-medium tabular-nums">{totalUsage === null ? "Not reported" : totalUsage.toLocaleString("en-US")}</p></div>
    </div>

    {problem ? <Alert variant="destructive"><AlertTitle>{problem.title}</AlertTitle><AlertDescription>{problem.description} {problem.action ? <Link className="font-medium underline" to={problem.action.to ?? playgroundUrl}>{problem.action.to ? problem.action.label : "Open playground"}</Link> : null}</AlertDescription></Alert> : null}

    <Card><CardHeader><CardTitle>Result</CardTitle><CardDescription>{run.data.source === "playground" ? "Requested from the admin portal" : "Requested by an external system"}</CardDescription></CardHeader><CardContent><p className="whitespace-pre-wrap break-words text-sm leading-6">{run.data.output ?? (active ? "Waiting for a result…" : "No response content.")}</p></CardContent></Card>

    <Card>
      <CardHeader><CardTitle>Agent loop</CardTitle><CardDescription>Decision → action → observation, repeated until the assistant answers or stops.</CardDescription></CardHeader>
      <CardContent>
        {messages.isPending ? <div className="space-y-3"><Skeleton className="h-20 w-full" /><Skeleton className="h-44 w-full" /></div> : messages.isError ? <Alert variant="destructive"><AlertTitle>Unable to load the agent loop</AlertTitle><AlertDescription>Reload the saved decisions and tool observations. <button className="font-medium underline" onClick={() => void messages.refetch()}>Try again</button></AlertDescription></Alert> : <AgentLoop messages={messages.data ?? []} steps={steps.data ?? []} active={active} totalIterations={run.data.iterations} fallbackRequest={run.data.input.message} />}
      </CardContent>
    </Card>

    <Card><CardHeader><CardTitle>Timing trace</CardTitle><CardDescription>Use this technical view to compare the relative duration of model and tool work.</CardDescription></CardHeader><CardContent>{!terminal ? <p className="text-sm text-muted-foreground">Timing becomes available after the activity finishes. The agent loop above continues to update while it runs.</p> : steps.isPending ? <div className="space-y-3"><Skeleton className="h-10 w-full" /><Skeleton className="h-10 w-full" /></div> : steps.isError ? <Alert variant="destructive"><AlertTitle>Unable to load the timing trace</AlertTitle><AlertDescription>Reload the processing timing. <button className="font-medium underline" onClick={() => void steps.refetch()}>Try again</button></AlertDescription></Alert> : <RunTimeline steps={steps.data ?? []} />}</CardContent></Card>

    {canManage && terminal ? deliveries.isPending ? <p className="text-sm text-muted-foreground">Loading delivery history…</p> : deliveries.isError ? <Alert variant="destructive"><AlertTitle>Unable to load delivery history</AlertTitle><AlertDescription>Reload the delivery information. <button className="font-medium underline" onClick={() => void deliveries.refetch()}>Try again</button></AlertDescription></Alert> : <WebhookDeliveryPanel runId={runId} deliveries={deliveries.data ?? []} /> : null}
  </div>;
}
