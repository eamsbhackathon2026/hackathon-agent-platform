import type { components } from "@/shared/api";

export type RunStep = components["schemas"]["Span"];

export type TimelineRow = {
  id: string;
  label: string;
  depth: number;
  offsetPct: number;
  widthPct: number;
  durationText: string;
  status: RunStep["status"];
  step: RunStep;
};
