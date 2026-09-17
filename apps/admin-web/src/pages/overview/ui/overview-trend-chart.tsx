import { Bar, CartesianGrid, ComposedChart, Legend, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import type { OverviewDailyPoint } from "@/entities/report";
import { TableCard, TableCardState } from "@/shared/ui";

/**
 * Series colours come from the theme variables so the chart follows light and
 * dark mode; a hardcoded palette would look right in exactly one of them.
 */
const series = { api: "var(--brand)", playground: "var(--muted-foreground)", failed: "var(--destructive)" };

/**
 * `date` is a date-only string the server already bucketed in the viewer's own
 * zone. Reading it back in UTC keeps the label on that day; letting the browser
 * shift it would move every label a day for viewers west of UTC.
 */
function shortDate(value: string) {
  return new Date(value).toLocaleDateString("en-US", { month: "short", day: "numeric", timeZone: "UTC" });
}

export function OverviewTrendChart({ daily }: { daily: OverviewDailyPoint[] }) {
  const busy = daily.some((point) => point.api + point.playground + point.failed > 0);
  return <TableCard title="Requests over time" description="Where requests came from, and how many failed.">
    {busy ? <TableCardState>
      <div className="h-72 w-full">
        <ResponsiveContainer height="100%" width="100%">
          <ComposedChart data={daily} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
            <CartesianGrid stroke="var(--border)" vertical={false} />
            <XAxis dataKey="date" tickFormatter={shortDate} tickLine={false} axisLine={false} tick={{ fill: "var(--muted-foreground)", fontSize: 12 }} minTickGap={16} />
            <YAxis allowDecimals={false} tickLine={false} axisLine={false} tick={{ fill: "var(--muted-foreground)", fontSize: 12 }} width={36} />
            <Tooltip
              labelFormatter={(label) => (typeof label === "string" ? shortDate(label) : label)}
              contentStyle={{ background: "var(--card)", border: "1px solid var(--border)", borderRadius: 8, color: "var(--foreground)" }}
            />
            <Legend iconType="circle" wrapperStyle={{ fontSize: 12 }} />
            <Bar dataKey="api" name="External system" stackId="source" fill={series.api} radius={[0, 0, 0, 0]} />
            <Bar dataKey="playground" name="Admin portal" stackId="source" fill={series.playground} radius={[4, 4, 0, 0]} />
            <Line dataKey="failed" name="Failed" stroke={series.failed} strokeWidth={2} dot={false} type="monotone" />
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </TableCardState> : <TableCardState className="grid min-h-52 place-items-center text-center">
      <div>
        <p className="font-medium">No requests in this period.</p>
        <p className="mt-1 text-sm text-muted-foreground">Pick a longer period, or send a request from the Playground to see activity here.</p>
      </div>
    </TableCardState>}
  </TableCard>;
}
