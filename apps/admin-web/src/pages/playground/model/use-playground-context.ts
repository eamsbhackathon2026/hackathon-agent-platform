import { useCallback, useEffect, useRef, useState } from "react";
import { type BlockerFunction, useBlocker, useLocation, useSearchParams } from "react-router";

import { cancelRun } from "@/features/stop-run";

export type PlaygroundTarget = { agentId: string; sessionId: string };

type UsePlaygroundContextOptions = {
  isStreaming: boolean;
  runId: string | null;
  streamSessionId: string | null;
  abort: () => void;
  resetContext: () => void;
};

function readTarget(search: string): PlaygroundTarget {
  const params = new URLSearchParams(search);
  return { agentId: params.get("agent") ?? "", sessionId: params.get("session") ?? "" };
}

function targetSearch(target: PlaygroundTarget) {
  const params = new URLSearchParams();
  if (target.agentId) params.set("agent", target.agentId);
  if (target.sessionId) params.set("session", target.sessionId);
  const search = params.toString();
  return search ? `?${search}` : "";
}

export function usePlaygroundContext({ isStreaming, runId, streamSessionId, abort, resetContext }: UsePlaygroundContextOptions) {
  const location = useLocation();
  const [, setSearchParams] = useSearchParams();
  const target = readTarget(location.search);
  const [pendingTarget, setPendingTarget] = useState<PlaygroundTarget | null>(null);
  const [switching, setSwitching] = useState(false);
  const [warning, setWarning] = useState<string | null>(null);
  const previousLocationKey = useRef(`${location.pathname}${location.search}`);
  const skipResetKey = useRef<string | null>(null);

  const shouldBlock = useCallback<BlockerFunction>(({ currentLocation, nextLocation }) => {
    if (!isStreaming) return false;
    if (currentLocation.pathname === nextLocation.pathname) {
      const next = readTarget(nextLocation.search);
      if (next.agentId === target.agentId && next.sessionId === streamSessionId) return false;
    }
    return currentLocation.pathname !== nextLocation.pathname || currentLocation.search !== nextLocation.search;
  }, [isStreaming, streamSessionId, target.agentId]);
  const blocker = useBlocker(shouldBlock);

  useEffect(() => {
    const locationKey = `${location.pathname}${location.search}`;
    if (locationKey === previousLocationKey.current) return;
    previousLocationKey.current = locationKey;
    if (skipResetKey.current === locationKey) {
      skipResetKey.current = null;
      return;
    }
    resetContext();
  }, [location.pathname, location.search, resetContext]);

  const writeTarget = useCallback((next: PlaygroundTarget, options: { replace?: boolean; reset?: boolean } = {}) => {
    const search = targetSearch(next);
    skipResetKey.current = `${location.pathname}${search}`;
    if (options.reset !== false) resetContext();
    setSearchParams(new URLSearchParams(search), options.replace === undefined ? {} : { replace: options.replace });
  }, [location.pathname, resetContext, setSearchParams]);

  const requestContextChange = useCallback((next: PlaygroundTarget) => {
    if (next.agentId === target.agentId && next.sessionId === target.sessionId) return;
    setWarning(null);
    if (isStreaming) {
      setPendingTarget(next);
      return;
    }
    writeTarget(next);
  }, [isStreaming, target.agentId, target.sessionId, writeTarget]);

  const syncStreamSession = useCallback((sessionId: string) => {
    if (!sessionId || sessionId === target.sessionId || !target.agentId) return;
    writeTarget({ agentId: target.agentId, sessionId }, { replace: true, reset: false });
  }, [target.agentId, target.sessionId, writeTarget]);

  const normalizeSessionAgent = useCallback((agentId: string) => {
    if (!target.sessionId || agentId === target.agentId) return;
    writeTarget({ agentId, sessionId: target.sessionId }, { replace: true, reset: false });
  }, [target.agentId, target.sessionId, writeTarget]);

  const cancelActiveRun = useCallback(async () => {
    abort();
    if (!runId) return;
    try {
      await cancelRun(runId);
    } catch {
      setWarning("The activity may still be running because cancellation could not be confirmed.");
    }
  }, [abort, runId]);

  const stopCurrentRun = useCallback(async () => {
    if (switching) return;
    setSwitching(true);
    setWarning(null);
    await cancelActiveRun();
    setSwitching(false);
  }, [cancelActiveRun, switching]);

  const keepWaiting = useCallback(() => {
    setPendingTarget(null);
    if (blocker.state === "blocked") blocker.reset();
  }, [blocker]);

  const stopAndSwitch = useCallback(async () => {
    if (switching) return;
    const next = pendingTarget;
    const blockedNavigation = blocker.state === "blocked" ? blocker : null;
    setSwitching(true);
    setWarning(null);
    await cancelActiveRun();
    setPendingTarget(null);
    if (blockedNavigation) {
      const nextLocationKey = `${blockedNavigation.location.pathname}${blockedNavigation.location.search}`;
      skipResetKey.current = nextLocationKey;
      resetContext();
      blockedNavigation.proceed();
    } else if (next) {
      writeTarget(next);
    }
    setSwitching(false);
  }, [blocker, cancelActiveRun, pendingTarget, resetContext, switching, writeTarget]);

  return {
    agentId: target.agentId,
    sessionId: target.sessionId,
    confirmationOpen: pendingTarget !== null || blocker.state === "blocked",
    switching,
    warning,
    requestContextChange,
    syncStreamSession,
    normalizeSessionAgent,
    keepWaiting,
    stopAndSwitch,
    stopCurrentRun,
  };
}
