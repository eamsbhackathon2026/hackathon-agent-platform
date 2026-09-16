import { formatDuration } from "@/shared/lib";
import type { RunStep, TimelineRow } from "../model/types";

function labelFor(step: RunStep) {
  if (step.kind === "tool_call") return `Tool: ${step.tool_name ?? step.name}`;
  if (step.name === "llm.compact_context") return "Prepare conversation context";
  if (step.kind === "llm_call") {
    const iteration = step.attributes.iteration;
    return typeof iteration === "number" ? `Model pass ${iteration}` : "Model pass";
  }
  return "Entire activity";
}

export function layoutTimeline(steps: RunStep[]): TimelineRow[] {
  if (!steps.length) return [];
  const byId = new Map(steps.map((step) => [step.id, step]));
  const start = Math.min(...steps.map((step) => new Date(step.started_at).valueOf()));
  const end = Math.max(...steps.map((step) => new Date(step.ended_at).valueOf()));
  const total = Math.max(1, end - start);

  const depthOf = (step: RunStep) => {
    let depth = 0;
    let parent = step.parent_span_id ? byId.get(step.parent_span_id) : undefined;
    const visited = new Set<string>();
    while (parent && !visited.has(parent.id)) {
      visited.add(parent.id);
      depth += 1;
      parent = parent.parent_span_id ? byId.get(parent.parent_span_id) : undefined;
    }
    return depth;
  };

  return [...steps]
    .sort((a, b) => a.started_at.localeCompare(b.started_at) || a.id.localeCompare(b.id))
    .map((step) => ({
      id: step.id,
      label: labelFor(step),
      depth: depthOf(step),
      offsetPct: Math.min(99, Math.max(0, ((new Date(step.started_at).valueOf() - start) / total) * 100)),
      widthPct: Math.max(1, (step.duration_ms / total) * 100),
      durationText: formatDuration(step.duration_ms),
      status: step.status,
      step,
    }));
}
