import { describe, expect, it } from "vitest";

import type { RunStep } from "../model/types";
import { layoutTimeline } from "./timeline-layout";

function step(overrides: Partial<RunStep>): RunStep {
  return {
    id: "root", run_id: "run", parent_span_id: null, kind: "run", name: "run", status: "ok",
    model: null, tool_name: null, usage: { input_tokens: null, output_tokens: null },
    started_at: "2026-01-01T00:00:00.000Z", ended_at: "2026-01-01T00:00:01.000Z",
    duration_ms: 1000, attributes: {}, error_message: null, ...overrides,
  };
}

describe("layoutTimeline", () => {
  it("calculates offsets, visible minimum widths, and nested depth", () => {
    const rows = layoutTimeline([
      step({ id: "root" }),
      step({ id: "llm", parent_span_id: "root", kind: "llm_call", started_at: "2026-01-01T00:00:00.250Z", ended_at: "2026-01-01T00:00:00.750Z", duration_ms: 500, attributes: { iteration: 1 } }),
      step({ id: "tool", parent_span_id: "llm", kind: "tool_call", tool_name: "weather", started_at: "2026-01-01T00:00:00.500Z", ended_at: "2026-01-01T00:00:00.500Z", duration_ms: 0 }),
    ]);
    expect(rows[1]).toMatchObject({ label: "Model pass 1", depth: 1, offsetPct: 25, widthPct: 50 });
    expect(rows[2]).toMatchObject({ label: "Tool: weather", depth: 2, offsetPct: 50, widthPct: 1 });
  });

  it("keeps an instant step at the end of the range visible", () => {
    const rows = layoutTimeline([
      step({ id: "root" }),
      step({ id: "instant", started_at: "2026-01-01T00:00:01.000Z", ended_at: "2026-01-01T00:00:01.000Z", duration_ms: 0 }),
    ]);
    expect(rows[1]).toMatchObject({ offsetPct: 99, widthPct: 1 });
  });
});
