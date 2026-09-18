import { describe, expect, it } from "vitest";

import { initialStreamState, streamReducer, type StreamAction } from "./stream-reducer";

describe("streamReducer", () => {
  it("builds the final message and tool state from events", () => {
    const events: StreamAction[] = [
      { type: "stream.reset" },
      { type: "run.started", run_id: "run-1", session_id: "session-1" },
      { type: "message.delta", text: "Hello" },
      { type: "tool.started", call_id: "call-1", tool_name: "weather", display_name: "Weather", details: [] },
      { type: "tool.finished", call_id: "call-1", ok: true, duration_ms: 25 },
      { type: "message.delta", text: " there" },
    ];
    const result = events.reduce(streamReducer, initialStreamState);
    expect(result.text).toBe("Hello there");
    expect(result.steps[0]).toMatchObject({ status: "succeeded", durationMs: 25 });
  });

  it("records a failed run and ignores an unknown event", () => {
    const running = streamReducer(initialStreamState, { type: "stream.reset" });
    const unknown = streamReducer(running, { type: "future.event" } as unknown as StreamAction);
    expect(unknown).toBe(running);
    const failed = streamReducer(running, {
      type: "run.failed",
      run: { output: null } as never,
      error: { code: "run_timeout", message: "timeout" },
    });
    expect(failed).toMatchObject({ status: "failed", stopReason: "run_timeout" });
  });

  it("handles request errors, cancellation, reset, and completed messages", () => {
    const requestError = streamReducer(initialStreamState, { type: "stream.request_failed", code: "unauthenticated", message: "Login" });
    expect(requestError).toMatchObject({ status: "failed", stopReason: "unauthenticated", errorMessage: "Login" });
    expect(streamReducer(requestError, { type: "stream.new" })).toEqual(initialStreamState);
    expect(streamReducer(initialStreamState, { type: "stream.cancelled" })).toMatchObject({ status: "failed", stopReason: "run_cancelled" });
    const message = { content: "Final" } as never;
    expect(streamReducer(initialStreamState, { type: "message.completed", message }).text).toBe("Final");
    expect(streamReducer(initialStreamState, { type: "run.completed", run: { output: "Done" } as never })).toMatchObject({ status: "completed", text: "Done" });
    expect(streamReducer(initialStreamState, { type: "stream.lost" })).toBe(initialStreamState);
  });
});
