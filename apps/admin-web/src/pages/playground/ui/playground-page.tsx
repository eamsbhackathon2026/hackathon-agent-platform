import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { Send } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router";

import { agentQueries } from "@/entities/agent";
import { conversationKeys, conversationQueries, mergeConversationMessages, type ConversationMessage } from "@/entities/conversation";
import { stopReasonToAction } from "@/entities/run";
import { useStreamedRun } from "@/features/send-message-stream";
import { StopRunButton } from "@/features/stop-run";
import { useAuthSession } from "@/shared/api";
import { Alert, AlertDescription, AlertTitle, Button, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Textarea } from "@/shared/ui";
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
    <div className="flex min-h-[calc(100dvh-9rem)] flex-col gap-5 xl:h-[calc(100dvh-9rem)] xl:flex-row">
      <ConversationRail activeSessionId={sessionId} disabled={switching} activeDeleteDisabled={state.status === "streaming"} onNew={startNew} onSelect={selectConversation} onActiveDeleted={startNew} />
      <div className="min-w-0 flex-1 space-y-5 xl:overflow-y-auto xl:pr-1">
        <div><h1 className="text-2xl font-semibold">Playground</h1><p className="text-sm text-muted-foreground">Send a request and follow the response in real time.</p></div>
        {warning ? <Alert><AlertTitle>Cancellation not confirmed</AlertTitle><AlertDescription>{warning}</AlertDescription></Alert> : null}
        {agents.isError ? <Alert variant="destructive"><AlertTitle>Unable to load assistants</AlertTitle><AlertDescription>Check the connection and try again. <button className="font-medium underline" onClick={() => void agents.refetch()}>Try again</button></AlertDescription></Alert> : null}
        {sessionUnavailable ? <Alert variant="destructive"><AlertTitle>This conversation can’t be continued in Playground</AlertTitle><AlertDescription>Open it in conversation history to review the transcript. <Link className="font-medium underline" to={`/conversations/${sessionId}`}>View conversation history</Link></AlertDescription></Alert> : null}
        <div className="max-w-md"><Select value={agentId} disabled={switching} onValueChange={changeAgent}><SelectTrigger aria-label="Choose assistant" data-playground-context-trigger="assistant"><SelectValue placeholder="Choose a ready assistant" /></SelectTrigger><SelectContent>{agents.data?.map((agent) => <SelectItem key={agent.id} value={agent.id} disabled={!agent.ready}>{agent.name}{agent.ready ? "" : " — not ready"}</SelectItem>)}</SelectContent></Select>
          {selectedAgent && !selectedAgent.ready ? <p className="mt-2 text-sm text-destructive">{selectedAgent.readiness_error?.message ?? "This assistant is not ready."} <Link className="underline" to={`/agents/${selectedAgent.id}`}>Edit assistant</Link></p> : null}
        </div>
        {history.isError ? <Alert variant="destructive"><AlertTitle>Unable to load previous messages</AlertTitle><AlertDescription>Reload them before continuing. <button className="font-medium underline" onClick={() => void history.refetch()}>Try again</button></AlertDescription></Alert> : null}
        {history.hasNextPage ? <Button type="button" variant="ghost" disabled={history.isFetchingNextPage} onClick={() => void history.fetchNextPage()}>Load more messages</Button> : null}
        <ChatThread messages={messages} {...(completedIsPersisted ? {} : { stream: state })} />
        {streamProblem ? <Alert variant="destructive"><AlertTitle>{streamProblem.title}</AlertTitle><AlertDescription>{state.errorMessage || streamProblem.description} {streamProblem.action?.to ? <Link className="font-medium underline" to={streamProblem.action.to}>{streamProblem.action.label}</Link> : streamProblem.action?.retry && lastMessage && canCompose ? <button type="button" className="font-medium underline" onClick={() => void send({ agentId, text: lastMessage, sessionId: sessionId || undefined })}>{streamProblem.action.label}</button> : null}</AlertDescription></Alert> : null}
        {state.status === "lost" && state.runId ? <p className="rounded-xl border border-warning/30 bg-warning-soft p-3 text-sm text-warning">Connection lost — <Link className="font-medium underline" to={`/activity/${state.runId}`}>view result</Link></p> : null}
        {state.runId && state.status !== "streaming" ? <Link className="inline-block text-sm font-medium text-primary underline" to={`/activity/${state.runId}`}>View processing steps</Link> : null}
        <div className="flex items-end gap-2"><Textarea aria-label="Message" autoComplete="off" name="message" value={draft} onChange={(event) => setDraft(event.target.value)} placeholder={canCompose ? "Enter your request…" : "Choose an available conversation and assistant"} disabled={!canCompose || state.status === "streaming" || switching} onKeyDown={(event) => { if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); void submit(); } }} />
          {state.status === "streaming" ? <StopRunButton disabled={switching} onStop={() => void stopCurrentRun()} /> : <Button type="button" disabled={!canCompose || !draft.trim()} onClick={() => void submit()}><Send />Send</Button>}
        </div>
      </div>
    </div>
    <ActiveRunNavigationDialog open={confirmationOpen} busy={switching} onKeepWaiting={keepWaiting} onStopAndSwitch={() => void stopAndSwitch()} onRestoreFocus={restoreContextFocus} />
  </>;
}
