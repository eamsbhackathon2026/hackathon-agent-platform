import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { HttpResponse, http } from "msw";
import { afterEach, describe, expect, it, vi } from "vitest";

import { server } from "@/test/msw-server";
import { useStreamedRun } from "./use-streamed-run";

function Harness() {
  const { state, send, reset } = useStreamedRun();
  return <><output>{`${state.status}:${state.text}:${state.runId ?? ""}`}</output><button onClick={() => void send({ agentId: "agent-1", text: "Hello" })}>Send</button><button onClick={reset}>Reset</button></>;
}

afterEach(() => vi.unstubAllGlobals());

describe("useStreamedRun", () => {
  it("clears queued events and their animation frame on reset", async () => {
    let queuedFrame: FrameRequestCallback | null = null;
    const cancelFrame = vi.fn();
    vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => { queuedFrame = callback; return 42; });
    vi.stubGlobal("cancelAnimationFrame", cancelFrame);
    const encoder = new TextEncoder();
    server.use(http.post("*/v1/agents/:agentId/runs/stream", () => new HttpResponse(new ReadableStream({ start(controller) {
      controller.enqueue(encoder.encode(`data: ${JSON.stringify({ type: "run.started", run_id: "stale-run", session_id: "session-1" })}\n\n`));
      controller.enqueue(encoder.encode(`data: ${JSON.stringify({ type: "message.delta", text: "stale text" })}\n\n`));
    } }), { headers: { "Content-Type": "text/event-stream" } })));
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(queuedFrame).not.toBeNull());
    const staleFrame = queuedFrame;
    fireEvent.click(screen.getByRole("button", { name: "Reset" }));
    expect(cancelFrame).toHaveBeenCalledWith(42);
    if (staleFrame) (staleFrame as FrameRequestCallback)(0);
    expect(screen.getByText("idle::")).toBeInTheDocument();
  });
});
