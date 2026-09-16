import { useCallback, useEffect, useReducer, useRef } from "react";

import { initialStreamState, streamReducer, type RunEvent } from "@/entities/conversation";
import { parseSseStream, postSse } from "@/shared/api";

type SendMessageInput = { agentId: string; text: string; sessionId?: string | undefined };

export function useStreamedRun() {
  const [state, dispatch] = useReducer(streamReducer, initialStreamState);
  const controllerRef = useRef<AbortController | null>(null);
  const queueRef = useRef<RunEvent[]>([]);
  const frameRef = useRef<number | null>(null);
  const generationRef = useRef(0);

  const flush = useCallback(() => {
    if (frameRef.current !== null) cancelAnimationFrame(frameRef.current);
    frameRef.current = null;
    for (const event of queueRef.current.splice(0)) dispatch(event);
  }, []);

  const clearPending = useCallback(() => {
    if (frameRef.current !== null) cancelAnimationFrame(frameRef.current);
    frameRef.current = null;
    queueRef.current = [];
  }, []);

  const enqueue = useCallback((event: RunEvent) => {
    queueRef.current.push(event);
    frameRef.current ??= requestAnimationFrame(flush);
  }, [flush]);

  const send = useCallback(async ({ agentId, text, sessionId }: SendMessageInput) => {
    controllerRef.current?.abort();
    clearPending();
    const generation = ++generationRef.current;
    const controller = new AbortController();
    controllerRef.current = controller;
    dispatch({ type: "stream.reset" });
    let runId: string | null = null;
    let returnedSessionId: string | null = null;

    try {
      const stream = await postSse(`/v1/agents/${encodeURIComponent(agentId)}/runs/stream`, {
        input: { message: text },
        ...(sessionId ? { session_id: sessionId } : {}),
      }, { signal: controller.signal });
      let terminal = false;
      for await (const message of parseSseStream(stream)) {
        if (generation !== generationRef.current) break;
        const event = JSON.parse(message.data) as RunEvent;
        if (event.type === "run.started") {
          runId = event.run_id;
          returnedSessionId = event.session_id;
        }
        terminal ||= event.type === "run.completed" || event.type === "run.failed";
        enqueue(event);
      }
      if (generation !== generationRef.current) return { runId, sessionId: returnedSessionId, terminal: false };
      flush();
      if (!terminal && !controller.signal.aborted) dispatch({ type: "stream.lost" });
      return { runId, sessionId: returnedSessionId, terminal };
    } catch (error) {
      if (generation !== generationRef.current) return { runId, sessionId: returnedSessionId, terminal: false };
      flush();
      if (!controller.signal.aborted) {
        const candidate = error as { status?: number; problem?: { code?: string; detail?: string }; message?: string };
        if (candidate.status !== undefined) {
          dispatch({
            type: "stream.request_failed",
            code: candidate.problem?.code ?? (candidate.status === 401 ? "unauthenticated" : "internal"),
            message: candidate.problem?.detail ?? candidate.message ?? "Unable to start the activity.",
          });
        } else if (runId) dispatch({ type: "stream.lost" });
        else dispatch({
          type: "stream.request_failed",
          code: "internal",
          message: "Unable to start processing. Check the connection and try again.",
        });
      }
      return { runId, sessionId: returnedSessionId, terminal: false };
    }
  }, [clearPending, enqueue, flush]);

  const abort = useCallback(() => {
    generationRef.current += 1;
    controllerRef.current?.abort();
    clearPending();
    dispatch({ type: "stream.cancelled" });
  }, [clearPending]);
  const reset = useCallback(() => {
    generationRef.current += 1;
    controllerRef.current?.abort();
    clearPending();
    dispatch({ type: "stream.new" });
  }, [clearPending]);

  useEffect(() => () => {
    generationRef.current += 1;
    controllerRef.current?.abort();
    clearPending();
  }, [clearPending]);

  return { state, send, abort, reset };
}
