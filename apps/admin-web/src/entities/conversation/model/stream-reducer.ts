import type { RunEvent, StreamState } from "./types";

export const initialStreamState: StreamState = {
  status: "idle",
  runId: null,
  sessionId: null,
  text: "",
  steps: [],
  stopReason: null,
  errorMessage: null,
};

export type StreamAction = RunEvent | { type: "stream.reset" } | { type: "stream.new" } | { type: "stream.lost" } | { type: "stream.cancelled" } | { type: "stream.request_failed"; code: string; message: string };

export function streamReducer(state: StreamState, event: StreamAction): StreamState {
  switch (event.type) {
    case "stream.reset":
      return { ...initialStreamState, status: "streaming" };
    case "stream.new":
      return initialStreamState;
    case "stream.lost":
      return state.status === "streaming" ? { ...state, status: "lost" } : state;
    case "stream.cancelled":
      return { ...state, status: "failed", stopReason: "run_cancelled" };
    case "stream.request_failed":
      return { ...state, status: "failed", stopReason: event.code, errorMessage: event.message };
    case "run.started":
      return { ...state, status: "streaming", runId: event.run_id, sessionId: event.session_id };
    case "message.delta":
      return { ...state, text: state.text + event.text };
    case "tool.started":
      return { ...state, steps: [...state.steps, { callId: event.call_id, name: event.display_name, status: "running" }] };
    case "tool.finished":
      return {
        ...state,
        steps: state.steps.map((step) => step.callId === event.call_id
          ? { ...step, status: event.ok ? "succeeded" : "failed", durationMs: event.duration_ms }
          : step),
      };
    case "message.completed":
      return { ...state, text: event.message.content };
    case "run.completed":
      return { ...state, status: "completed", text: event.run.output ?? state.text };
    case "run.failed":
      return { ...state, status: "failed", stopReason: event.error.code, errorMessage: event.error.message, text: event.run.output ?? state.text };
    default:
      return state;
  }
}
