import { describe, expect, it } from "vitest";

import type { ConversationMessage } from "@/entities/conversation";
import type { RunStep } from "@/entities/run-step";
import { buildAgentLoop } from "./build-agent-loop";

function message(overrides: Partial<ConversationMessage>): ConversationMessage {
  return {
    id: "message", session_id: "session", run_id: "run", seq: 1, role: "assistant", content: "",
    tool_calls: [], tool_call_id: null, tool_name: null, is_error: false, created_at: "2026-09-15T00:00:00Z",
    ...overrides,
  };
}

function step(overrides: Partial<RunStep>): RunStep {
  return {
    id: "step", run_id: "run", parent_span_id: null, kind: "llm_call", name: "llm.generate", status: "ok",
    model: "model", tool_name: null, usage: { input_tokens: 1, output_tokens: 1 },
    started_at: "2026-09-15T00:00:00Z", ended_at: "2026-09-15T00:00:01Z", duration_ms: 1000,
    attributes: {}, error_message: null, ...overrides,
  };
}

describe("buildAgentLoop", () => {
  it("groups tool requests, observations, and model timing into iterations", () => {
    const messages = [
      message({ id: "request", role: "user", seq: 1, content: "Check the weather" }),
      message({ id: "decision", seq: 2, tool_calls: [{ id: "call-1", name: "weather", arguments: { city: "Hue" } }] }),
      message({ id: "result", role: "tool", seq: 3, tool_call_id: "call-1", tool_name: "weather", content: "Sunny" }),
      message({ id: "answer", seq: 4, content: "It is sunny." }),
    ];
    const steps = [
      step({ id: "llm-1", attributes: { iteration: 1, finish_reason: "tool_calls" } }),
      step({ id: "tool-1", kind: "tool_call", name: "tool.execute", tool_name: "weather", attributes: { call_id: "call-1" } }),
      step({ id: "llm-2", started_at: "2026-09-15T00:00:02Z", ended_at: "2026-09-15T00:00:03Z", attributes: { iteration: 2, finish_reason: "stop" } }),
    ];

    const loop = buildAgentLoop(messages, steps);

    expect(loop.request?.content).toBe("Check the weather");
    expect(loop.iterations).toHaveLength(2);
    expect(loop.iterations[0]).toMatchObject({ number: 1, span: { id: "llm-1" } });
    expect(loop.iterations[0]?.actions[0]).toMatchObject({
      call: { name: "weather", arguments: { city: "Hue" } },
      result: { content: "Sunny" },
      span: { id: "tool-1" },
    });
    expect(loop.iterations[1]).toMatchObject({ number: 2, span: { id: "llm-2" }, actions: [] });
  });

  it("keeps retry attempts and context preparation visible without inventing decisions", () => {
    const loop = buildAgentLoop(
      [message({ id: "request", role: "user", content: "Hello" }), message({ id: "answer", seq: 2, content: "Hi" })],
      [
        step({ id: "compact", name: "llm.compact_context" }),
        step({ id: "retry", attributes: { iteration: 1, finish_reason: "STOP" } }),
        step({ id: "answer-span", attributes: { iteration: 2, finish_reason: "stop" } }),
      ],
    );

    expect(loop.contextPreparations.map((item) => item.id)).toEqual(["compact"]);
    expect(loop.iterations[0]).toMatchObject({ number: 2, span: { id: "answer-span" } });
    expect(loop.unrecordedModelPasses).toBe(1);
  });

  it("matches a Gemini tool decision after a retry by the following tool span", () => {
    const loop = buildAgentLoop(
      [
        message({ id: "request", role: "user", content: "Find it" }),
        message({ id: "decision", seq: 2, content: "I will search.", tool_calls: [{ id: "call", name: "search", arguments: {} }] }),
        message({ id: "result", role: "tool", seq: 3, tool_call_id: "call", content: "Found" }),
      ],
      [
        step({ id: "retry", ended_at: "2026-09-15T00:00:00.500Z", attributes: { iteration: 1, finish_reason: "STOP" } }),
        step({ id: "decision-span", started_at: "2026-09-15T00:00:01Z", ended_at: "2026-09-15T00:00:02Z", attributes: { iteration: 2, finish_reason: "STOP" } }),
        step({ id: "tool", kind: "tool_call", name: "tool.execute", started_at: "2026-09-15T00:00:02Z", attributes: { call_id: "call" } }),
      ],
    );

    expect(loop.iterations[0]).toMatchObject({ number: 2, span: { id: "decision-span" } });
    expect(loop.unrecordedModelPasses).toBe(1);
  });

  it("builds a live loop before terminal spans are available", () => {
    const loop = buildAgentLoop([
      message({ id: "request", role: "user", content: "Find it" }),
      message({ id: "decision", seq: 2, tool_calls: [{ id: "call", name: "search", arguments: {} }] }),
    ], []);

    expect(loop.iterations[0]).toMatchObject({ number: 1, span: undefined });
    expect(loop.iterations[0]?.actions[0]).toMatchObject({ result: undefined, span: undefined });
  });
});
