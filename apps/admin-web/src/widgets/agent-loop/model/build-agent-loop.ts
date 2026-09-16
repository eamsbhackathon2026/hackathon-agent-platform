import type { ConversationMessage } from "@/entities/conversation";
import type { RunStep } from "@/entities/run-step";

export type AgentLoopToolAction = {
  call: ConversationMessage["tool_calls"][number];
  result: ConversationMessage | undefined;
  span: RunStep | undefined;
};

export type AgentLoopIteration = {
  number: number;
  message: ConversationMessage;
  span: RunStep | undefined;
  actions: AgentLoopToolAction[];
};

export type AgentLoopModel = {
  request: ConversationMessage | undefined;
  iterations: AgentLoopIteration[];
  contextPreparations: RunStep[];
  unrecordedModelPasses: number;
};

function attributeString(step: RunStep, name: string) {
  const value = step.attributes[name];
  return typeof value === "string" ? value : undefined;
}

function attributeNumber(step: RunStep, name: string) {
  const value = step.attributes[name];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function stepTime(value: string) {
  const time = new Date(value).valueOf();
  return Number.isFinite(time) ? time : 0;
}

function latestGeneration(spans: RunStep[]) {
  return spans.reduce<RunStep | undefined>((latest, span) => {
    if (!latest) return span;
    const iteration = attributeNumber(span, "iteration") ?? -1;
    const latestIteration = attributeNumber(latest, "iteration") ?? -1;
    if (iteration !== latestIteration) return iteration > latestIteration ? span : latest;
    return stepTime(span.ended_at) >= stepTime(latest.ended_at) ? span : latest;
  }, undefined);
}

function pickGenerationSpan(
  message: ConversationMessage,
  spans: RunStep[],
  toolSpans: Map<string, RunStep>,
  matchedSpanIds: Set<string>,
) {
  const available = spans.filter((span) => !matchedSpanIds.has(span.id));
  if (!available.length) return undefined;

  if (message.tool_calls.length > 0) {
    const firstToolStart = Math.min(...message.tool_calls.flatMap((call) => {
      const span = toolSpans.get(call.id);
      return span ? [stepTime(span.started_at)] : [];
    }));
    if (Number.isFinite(firstToolStart)) {
      const preceding = available.filter((span) => stepTime(span.ended_at) <= firstToolStart);
      if (preceding.length) return latestGeneration(preceding);
    }

    const explicitlyReported = available.find((span) => attributeString(span, "finish_reason")?.toLowerCase() === "tool_calls");
    return explicitlyReported ?? latestGeneration(available);
  }

  // A saved assistant message without tool calls ends the run. Provider retries
  // create spans but no message, so the final saved answer belongs to the latest
  // unmatched generation span rather than the first span with a STOP signal.
  return latestGeneration(available);
}

export function buildAgentLoop(messages: ConversationMessage[], steps: RunStep[]): AgentLoopModel {
  const orderedMessages = [...messages].sort((left, right) => left.seq - right.seq || left.id.localeCompare(right.id));
  const orderedSteps = [...steps].sort((left, right) => left.started_at.localeCompare(right.started_at) || left.id.localeCompare(right.id));
  const generationSpans = orderedSteps.filter((step) => step.kind === "llm_call" && step.name === "llm.generate");
  const contextPreparations = orderedSteps.filter((step) => step.kind === "llm_call" && step.name === "llm.compact_context");
  const toolResults = new Map(orderedMessages.filter((message) => message.role === "tool" && message.tool_call_id).map((message) => [message.tool_call_id as string, message]));
  const toolSpans = new Map(orderedSteps.filter((step) => step.kind === "tool_call").flatMap((step) => {
    const callId = attributeString(step, "call_id");
    return callId ? [[callId, step] as const] : [];
  }));

  const matchedSpanIds = new Set<string>();
  const iterations = orderedMessages.filter((message) => message.role === "assistant").map((message, index) => {
    const span = pickGenerationSpan(message, generationSpans, toolSpans, matchedSpanIds);
    if (span) matchedSpanIds.add(span.id);
    const number = span ? attributeNumber(span, "iteration") ?? index + 1 : index + 1;
    return {
      number,
      message,
      span,
      actions: message.tool_calls.map((call) => ({ call, result: toolResults.get(call.id), span: toolSpans.get(call.id) })),
    };
  });

  return {
    request: orderedMessages.find((message) => message.role === "user"),
    iterations,
    contextPreparations,
    unrecordedModelPasses: Math.max(0, generationSpans.length - matchedSpanIds.size),
  };
}
