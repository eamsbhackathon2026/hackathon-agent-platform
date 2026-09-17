import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router";

import { agentQueries } from "@/entities/agent";
import { conversationKeys, conversationQueries, mergeConversationMessages, type ConversationMessage } from "@/entities/conversation";
import { stopReasonToAction } from "@/entities/run";
import { useStreamedRun } from "@/features/send-message-stream";
import { useAuthSession } from "@/shared/api";
import { Alert, AlertDescription, AlertTitle, Button } from "@/shared/ui";
import { ChatComposer } from "@/widgets/chat-composer";
import { ChatThread } from "@/widgets/chat-thread";
import { ConversationRail } from "@/widgets/conversation-rail";

import { usePlaygroundContext } from "../model/use-playground-context";
import { ActiveRunNavigationDialog } from "./active-run-navigation-dialog";

export function PlaygroundPage() {
  const [draft, setDraft] = useState("");
  const [lastMessage, setLastMessage] = useState("");
  const [localMessages, setLocalMessages] = useState<ConversationMessage[]>([]);
  const lastSyncedRun = useRef<string | null>(null);
  const lastPublishedSession = useRef<string | null>(null);
  const contextFocusTarget = useRef<HTMLElement | null>(null);
  const contextFocusFallback = useRef<"assistant" | "recent" | null>(null);
  const queryClient = useQueryClient();
  const agents = useQuery(agentQueries.list());
  const { user } = useAuthSession();
  const { state, send, abort, reset } = useStreamedRun();

  const resetConversation = useCallback(() => {
    reset();
    setDraft("");
    setLastMessage("");
    setLocalMessages([]);
    lastSyncedRun.current = null;
    lastPublishedSession.current = null;
  }, [reset]);
  const {
    agentId,
    sessionId,
    confirmationOpen,
    switching,
    warning,
    requestContextChange,
    syncStreamSession,
    normalizeSessionAgent,
    keepWaiting,
    stopAndSwitch,
    stopCurrentRun,
  } = usePlaygroundContext({
    isStreaming: state.status === "streaming",
    runId: state.runId,
    streamSessionId: state.sessionId,
    abort,
    resetContext: resetConversation,
  });
  const sessionDetails = useQuery({ ...conversationQueries.detail(sessionId), enabled: Boolean(sessionId) });
  const history = useInfiniteQuery({ ...conversationQueries.messagesInfinite(sessionId), enabled: Boolean(sessionId) && state.status !== "streaming" });
  const selectedAgent = agents.data?.find((agent) => agent.id === agentId);
  const persistedMessages = useMemo(() => history.data?.pages.flatMap((page) => page.items) ?? [], [history.data]);
  const messages = useMemo(() => mergeConversationMessages(persistedMessages, localMessages), [persistedMessages, localMessages]);
  const belongsToUser = sessionDetails.data?.source === "playground" && sessionDetails.data.created_by_user_id === user?.id;
  const sessionEligible = !sessionId || Boolean(belongsToUser && sessionDetails.data?.agent_id === agentId);
  const sessionUnavailable = Boolean(sessionId) && (sessionDetails.isError || (sessionDetails.isSuccess && !belongsToUser));
  const canCompose = Boolean(agentId && selectedAgent?.ready && sessionEligible);
  const completedIsPersisted = Boolean(state.runId && persistedMessages.some((message) => message.role === "assistant" && message.run_id === state.runId));
  const streamProblem = stopReasonToAction(state.stopReason, agentId);

  useEffect(() => {
    if (!sessionDetails.data || sessionDetails.data.agent_id === agentId) return;
    normalizeSessionAgent(sessionDetails.data.agent_id);
  }, [agentId, normalizeSessionAgent, sessionDetails.data]);

  useEffect(() => {
    if (!state.sessionId || lastPublishedSession.current === state.sessionId) return;
    lastPublishedSession.current = state.sessionId;
    syncStreamSession(state.sessionId);
    void queryClient.invalidateQueries({ queryKey: conversationKeys.all });
  }, [queryClient, state.sessionId, syncStreamSession]);

  useEffect(() => {
    if (!state.runId || !state.sessionId || lastSyncedRun.current === state.runId) return;
    if (state.status !== "completed" && state.status !== "failed") return;
    lastSyncedRun.current = state.runId;
    void Promise.all([
      queryClient.invalidateQueries({ queryKey: conversationKeys.all }),
      queryClient.invalidateQueries({ queryKey: conversationKeys.messagesInfinite(state.sessionId) }),
    ]);
  }, [queryClient, state.runId, state.sessionId, state.status]);

  async function submit() {
    const text = draft.trim();
    if (!canCompose || !text || state.status === "streaming") return;
    setLocalMessages((current) => [...current, {
      id: `optimistic:${crypto.randomUUID()}`, session_id: sessionId || "new", run_id: null, seq: current.length + 1,
      role: "user", content: text, tool_calls: [], tool_call_id: null, tool_name: null, is_error: false,
      created_at: new Date().toISOString(),
    }]);
    setDraft("");
    setLastMessage(text);
    await send({ agentId, text, sessionId: sessionId || undefined });
  }

  const rememberContextFocus = useCallback((fallback: "assistant" | "recent") => {
    if (state.status !== "streaming") return;
    contextFocusTarget.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    contextFocusFallback.current = fallback;
  }, [state.status]);
  const restoreContextFocus = useCallback(() => {
    const previous = contextFocusTarget.current;
    const fallback = contextFocusFallback.current ?? "assistant";
    contextFocusTarget.current = null;
    contextFocusFallback.current = null;
    const previousIsUsable = previous?.isConnected && !previous.matches(":disabled") && !previous.closest('[aria-hidden="true"], [data-state="closed"]');
    const next = previousIsUsable ? previous : document.querySelector<HTMLElement>(`[data-playground-context-trigger="${fallback}"]`);
    next?.focus();
  }, []);
  const startNew = useCallback(() => {
    rememberContextFocus("recent");
    requestContextChange({ agentId: selectedAgent?.ready ? agentId : "", sessionId: "" });
  }, [agentId, rememberContextFocus, requestContextChange, selectedAgent?.ready]);
  const selectConversation = useCallback((target: { agentId: string; sessionId: string }) => {
    rememberContextFocus("recent");
    requestContextChange(target);
  }, [rememberContextFocus, requestContextChange]);
  const changeAgent = useCallback((value: string) => {
    rememberContextFocus("assistant");
    requestContextChange({ agentId: value, sessionId: "" });
  }, [rememberContextFocus, requestContextChange]);

  return <>
    {/* A definite height, not a minimum: the transcript below scrolls inside this
        column so the composer stays at the bottom and grows upward. Subtracts the
        shell header plus the content padding above and below the column, with
        room for the page description wrapping to a second line in the header —
        leftover space only lifts the composer slightly, while a short column
        would push it under the fold, which is what anchoring it prevents. */}
    <div className="flex h-[calc(100dvh-9.5rem)] flex-col gap-5 md:h-[calc(100dvh-10.75rem)] xl:flex-row">
      <ConversationRail activeSessionId={sessionId} disabled={switching} activeDeleteDisabled={state.status === "streaming"} onNew={startNew} onSelect={selectConversation} onActiveDeleted={startNew} />
      <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-5">
        {warning ? <Alert className="shrink-0"><AlertTitle>Cancellation not confirmed</AlertTitle><AlertDescription>{warning}</AlertDescription></Alert> : null}
        {agents.isError ? <Alert variant="destructive" className="shrink-0"><AlertTitle>Unable to load assistants</AlertTitle><AlertDescription>Check the connection and try again. <button className="font-medium underline" onClick={() => void agents.refetch()}>Try again</button></AlertDescription></Alert> : null}
        {sessionUnavailable ? <Alert variant="destructive" className="shrink-0"><AlertTitle>This conversation can’t be continued in Playground</AlertTitle><AlertDescription>Open it in conversation history to review the transcript. <Link className="font-medium underline" to={`/conversations/${sessionId}`}>View conversation history</Link></AlertDescription></Alert> : null}
        {history.isError ? <Alert variant="destructive" className="shrink-0"><AlertTitle>Unable to load previous messages</AlertTitle><AlertDescription>Reload them before continuing. <button className="font-medium underline" onClick={() => void history.refetch()}>Try again</button></AlertDescription></Alert> : null}
        {history.hasNextPage ? <Button type="button" variant="ghost" className="shrink-0 self-start" disabled={history.isFetchingNextPage} onClick={() => void history.fetchNextPage()}>Load more messages</Button> : null}
        <ChatThread messages={messages} {...(completedIsPersisted ? {} : { stream: state })} />
        {streamProblem ? <Alert variant="destructive" className="shrink-0"><AlertTitle>{streamProblem.title}</AlertTitle><AlertDescription>{state.errorMessage || streamProblem.description} {streamProblem.action?.to ? <Link className="font-medium underline" to={streamProblem.action.to}>{streamProblem.action.label}</Link> : streamProblem.action?.retry && lastMessage && canCompose ? <button type="button" className="font-medium underline" onClick={() => void send({ agentId, text: lastMessage, sessionId: sessionId || undefined })}>{streamProblem.action.label}</button> : null}</AlertDescription></Alert> : null}
        {state.status === "lost" && state.runId ? <p className="shrink-0 rounded-xl border border-warning/30 bg-warning-soft p-3 text-sm text-warning">Connection lost — <Link className="font-medium underline" to={`/activity/${state.runId}`}>view result</Link></p> : null}
        {state.runId && state.status !== "streaming" ? <Link className="inline-block shrink-0 text-sm font-medium text-primary underline" to={`/activity/${state.runId}`}>View processing steps</Link> : null}
        <div className="shrink-0 space-y-2">
          <p className="text-center text-xs text-muted-foreground">Assistants can make mistakes. Check important information.</p>
          <ChatComposer value={draft} onChange={setDraft} onSubmit={() => void submit()} placeholder={canCompose ? "Enter your request…" : "Choose an available conversation and assistant"} disabled={!canCompose} busy={state.status === "streaming"} switching={switching} agents={agents.data ?? []} agentId={agentId} onAgentChange={changeAgent} onStop={() => void stopCurrentRun()} />
          {selectedAgent && !selectedAgent.ready ? <p className="px-2 text-sm text-destructive">{selectedAgent.readiness_error?.message ?? "This assistant is not ready."} <Link className="underline" to={`/agents/${selectedAgent.id}`}>Edit assistant</Link></p> : null}
        </div>
      </div>
    </div>
    <ActiveRunNavigationDialog open={confirmationOpen} busy={switching} onKeepWaiting={keepWaiting} onStopAndSwitch={() => void stopAndSwitch()} onRestoreFocus={restoreContextFocus} />
  </>;
}
