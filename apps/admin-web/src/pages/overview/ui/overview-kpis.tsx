import type { OverviewTotals } from "@/entities/report";
import { formatDuration } from "@/shared/lib";
import { Card, CardContent } from "@/shared/ui";

const numberFormat = new Intl.NumberFormat("en-US");

/**
 * Reads a rate as a whole percentage, or says so when there is nothing to
 * divide. Floored, so it never claims 100% while the hint lists failures.
 */
function percent(value: number | null) {
  return value === null ? "Not available" : `${Math.floor(value * 100)}%`;
}

/** Durations stay hidden until enough requests finished for the number to mean anything. */
function duration(value: number | null) {
  return value === null ? "Not enough data yet" : formatDuration(value);
}

export function OverviewKpis({ totals }: { totals: OverviewTotals }) {
  const cards = [
    { label: "Requests", value: numberFormat.format(totals.requests), hint: `${numberFormat.format(totals.sessions)} ${totals.sessions === 1 ? "conversation" : "conversations"}` },
    { label: "Completed", value: percent(totals.success_rate), hint: `${numberFormat.format(totals.failed)} failed, ${numberFormat.format(totals.cancelled)} stopped` },
    { label: "Slowest responses", value: duration(totals.p95_duration_ms), hint: "95 out of 100 requests finish faster than this" },
    { label: "Work processed", value: numberFormat.format(totals.processing_units), hint: "Processing units across every request" },
  ];
  return <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
    {cards.map((card) => <Card key={card.label}>
      <CardContent className="pt-5 sm:pt-6">
        <p className="text-sm text-muted-foreground">{card.label}</p>
        <p className="mt-2 text-2xl font-semibold tabular-nums tracking-[-0.02em]">{card.value}</p>
        <p className="mt-1 text-xs text-muted-foreground">{card.hint}</p>
      </CardContent>
    </Card>)}
  </div>;
}
