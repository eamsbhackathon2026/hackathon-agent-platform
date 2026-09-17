import { useEffect, useRef } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Clock3, Gauge, MessageSquare, MessagesSquare, Radio } from "lucide-react";
import { Link, useNavigate, useParams } from "react-router";

import { conversationKeys, conversationQueries } from "@/entities/conversation";
import { isRunActive, runQueries, type Run } from "@/entities/run";
import { DeleteConversationButton } from "@/features/delete-conversation";
import { useAuthSession } from "@/shared/api";
import { formatDate, formatDuration, problemToAction, usePageHeader } from "@/shared/lib";
import { Alert, AlertDescription, AlertTitle, Badge, Button, Card, CardContent, Skeleton } from "@/shared/ui";
import { groupTurns } from "../model/group-turns";
import { ConversationTurnCard } from "./conversation-turn-card";

function totals(runs: Run[]) {
  let units: number | null = null;
  let processing = 0;
  for (const run of runs) {
    if (run.usage.input_tokens !== null || run.usage.output_tokens !== null) units = (units ?? 0) + (run.usage.input_tokens ?? 0) + (run.usage.output_tokens ?? 0);
    if (run.started_at && run.finished_at) processing += Math.max(0, new Date(run.finished_at).valueOf() - new Date(run.started_at).valueOf());
  }
  return { units, processing, failed: runs.filter((run) => run.status === "failed").length };
}

function Stat({ icon: Icon, label, value, detail }: { icon: typeof Clock3; label: string; value: string; detail?: string | undefined }) {
  return <div className="rounded-2xl border bg-card p-4"><div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground"><Icon className="size-4" />{label}</div><p className="mt-2 text-sm font-medium tabular-nums">{value}</p>{detail ? <p className="mt-0.5 text-xs text-muted-foreground">{detail}</p> : null}</div>;
}

export function ConversationDetailPage() {
  const { sessionId = "" } = useParams();
  const navigate = useNavigate();
  const conversation = useQuery(conversationQueries.detail(sessionId));
  const runs = useInfiniteQuery({ ...runQueries.bySession(sessionId), refetchInterval: (query) => query.state.data?.pages.some((page) => page.items.some((run) => isRunActive(run.status))) ? 2_000 : false });
  const runItems = runs.data?.pages.flatMap((page) => page.items) ?? [];
  const live = runItems.some((run) => isRunActive(run.status));
  const messages = useInfiniteQuery({ ...conversationQueries.messagesInfinite(sessionId), refetchInterval: live ? 2_000 : false });
  const { user } = useAuthSession();
  const canContinue = conversation.data?.source === "playground" && conversation.data.created_by_user_id === user?.id;
  usePageHeader(conversation.data ? { title: conversation.data.title || "Conversation", description: `Updated ${formatDate(conversation.data.updated_at)}` } : null);

  // A turn shows its status only once its request is loaded, so keep loading until every
  // request is here. A failed page stops the loop; the error alert offers a retry.
  const { hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage } = runs;
  useEffect(() => {
    if (hasNextPage && !isFetchingNextPage && !isFetchNextPageError) void fetchNextPage();
  }, [fetchNextPage, hasNextPage, isFetchingNextPage, isFetchNextPageError]);

  // The answer is saved just before a request is marked finished, and the two lists poll
  // separately, so polling can stop one beat before the answer arrives. Read the messages
  // once more whenever the conversation goes quiet.
  const queryClient = useQueryClient();
  const wasLive = useRef(false);
  useEffect(() => {
    if (wasLive.current && !live) {
      const queryKey = conversationKeys.messagesInfinite(sessionId);
      void queryClient.cancelQueries({ queryKey, exact: true }).then(() => queryClient.refetchQueries({ queryKey, exact: true, type: "active" }));
    }
    wasLive.current = live;
  }, [live, queryClient, sessionId]);

  if (conversation.isPending) return <div className="space-y-5"><Skeleton className="h-5 w-40" /><Skeleton className="h-20 w-full" /><Skeleton className="h-64 w-full" /></div>;
  if (conversation.isError || !conversation.data) {
    const action = problemToAction((conversation.error as { code?: string })?.code);
    return <Alert variant="destructive"><AlertTitle>{action.title}</AlertTitle><AlertDescription>{action.description} <button className="font-medium underline" onClick={() => void conversation.refetch()}>{action.action?.label ?? "Try again"}</button></AlertDescription></Alert>;
  }
  const messageItems = messages.data?.pages.flatMap((page) => page.items) ?? [];
  const turns = groupTurns(messageItems);
  const runsById = new Map(runItems.map((run) => [run.id, run]));
  const summary = totals(runItems);

  return <div className="space-y-5">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <Link className="inline-flex min-h-11 items-center gap-2 text-sm font-medium hover:underline" to="/conversations"><ArrowLeft className="size-4" />Conversation History</Link>
      <div className="flex flex-wrap items-center gap-2">
        {live ? <Badge variant="warning" className="gap-1.5" aria-live="polite"><Radio className="size-3.5 animate-pulse" />Live updates</Badge> : null}
        {canContinue ? <Button asChild><Link to={`/playground?agent=${conversation.data.agent_id}&session=${conversation.data.id}`}><MessageSquare />Continue conversation</Link></Button> : null}
        <DeleteConversationButton conversationId={sessionId} onDeleted={() => navigate("/conversations")} />
      </div>
    </div>

    {runs.isError || runs.isFetchNextPageError ? <Alert variant="destructive"><AlertTitle>Unable to load the status of each turn</AlertTitle><AlertDescription>The messages below are still complete. <button className="font-medium underline" onClick={() => void runs.refetch()}>Try again</button></AlertDescription></Alert> : <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <Stat icon={MessagesSquare} label="Turns" value={runs.isPending ? "Loading…" : String(runItems.length)} detail={summary.failed ? `${summary.failed} failed` : undefined} />
      <Stat icon={Clock3} label="Started" value={formatDate(conversation.data.created_at)} detail={`Last activity ${formatDate(conversation.data.updated_at)}`} />
      <Stat icon={Gauge} label="Processing usage" value={summary.units === null ? "Not reported" : summary.units.toLocaleString("en-US")} />
      <Stat icon={Gauge} label="Processing time" value={formatDuration(summary.processing)} detail="Total across finished turns" />
    </div>}

    {!canContinue ? <Card><CardContent className="pt-6 text-sm text-muted-foreground">This conversation came from an external system or belongs to another user, so it cannot be continued here.</CardContent></Card> : null}

    {messages.isPending ? <div className="space-y-3"><Skeleton className="h-32 w-full" /><Skeleton className="h-32 w-full" /></div> : messages.isError ? <Alert variant="destructive"><AlertTitle>Unable to load messages</AlertTitle><AlertDescription>Reload the conversation messages. <button className="font-medium underline" onClick={() => void messages.refetch()}>Try again</button></AlertDescription></Alert> : turns.length ? <ol className="space-y-4" aria-label="Conversation turns">
      {turns.map((turn, index) => <ConversationTurnCard key={turn.key} turn={turn} number={index + 1} run={turn.runId ? runsById.get(turn.runId) : undefined} />)}
    </ol> : <Card><CardContent className="py-10 text-center text-sm text-muted-foreground">No messages in this conversation yet.{canContinue ? " Continue it in the Playground to send the first request." : ""}</CardContent></Card>}
    {messages.hasNextPage ? <Button variant="outline" disabled={messages.isFetchingNextPage} onClick={() => void messages.fetchNextPage()}>{messages.isFetchingNextPage ? "Loading…" : "Load more messages"}</Button> : null}
  </div>;
}
