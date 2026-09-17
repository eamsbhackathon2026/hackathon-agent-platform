import type { components } from "@/shared/api";

export type OverviewReport = components["schemas"]["OverviewReport"];
export type OverviewTotals = components["schemas"]["OverviewTotals"];
export type OverviewDailyPoint = components["schemas"]["OverviewDailyPoint"];
export type OverviewAgentRow = components["schemas"]["OverviewAgentRow"];
export type OverviewErrorRow = components["schemas"]["OverviewErrorRow"];
export type OverviewToolRow = components["schemas"]["OverviewToolRow"];

/** Presets keep the window short enough to stay readable on one screen. */
export type OverviewRange = "24h" | "7d" | "30d";

export const overviewRangeLabels: Record<OverviewRange, string> = {
  "24h": "Last 24 hours",
  "7d": "Last 7 days",
  "30d": "Last 30 days",
};

const rangeHours: Record<OverviewRange, number> = { "24h": 24, "7d": 24 * 7, "30d": 24 * 30 };

export function isOverviewRange(value: string | null): value is OverviewRange {
  return value === "24h" || value === "7d" || value === "30d";
}

/** Resolves a preset against the viewer's own clock and zone. */
export function overviewWindow(range: OverviewRange, now = new Date()) {
  const to = now.toISOString();
  const from = new Date(now.valueOf() - rangeHours[range] * 60 * 60 * 1000).toISOString();
  const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  return { from, to, timeZone };
}
