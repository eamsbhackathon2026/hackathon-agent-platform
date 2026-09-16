import type { components } from "@/shared/api";

export type Conversation = components["schemas"]["Session"];
export type ConversationMessage = components["schemas"]["Message"];
export type RunEvent = components["schemas"]["RunEvent"];

export type StreamToolStep = {
  callId: string;
  name: string;
  status: "running" | "succeeded" | "failed";
  durationMs?: number;
};

export type StreamState = {
  status: "idle" | "streaming" | "completed" | "failed" | "lost";
  runId: string | null;
  sessionId: string | null;
  text: string;
  steps: StreamToolStep[];
  stopReason: string | null;
  errorMessage: string | null;
};
