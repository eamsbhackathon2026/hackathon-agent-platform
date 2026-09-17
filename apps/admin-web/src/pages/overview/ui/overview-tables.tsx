import { ArrowRight } from "lucide-react";
import { Link } from "react-router";

import type { OverviewAgentRow, OverviewErrorRow, OverviewToolRow } from "@/entities/report";
import { formatDate, formatDuration, problemToAction, type ProblemCode } from "@/shared/lib";
import { Badge, Button, Table, TableBody, TableCard, TableCardState, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui";

const numberFormat = new Intl.NumberFormat("en-US");

// Floor, not round: "100%" beside "1 failed" reads as a contradiction.
function percent(value: number | null) {
  return value === null ? "—" : `${Math.floor(value * 100)}%`;
}

function duration(value: number | null) {
  return value === null ? "Not enough data yet" : formatDuration(value);
}

/** Every row links back to Activity carrying the same window, so the detail matches the summary. */
function activityLink(period: { from: string; to: string }, extra: Record<string, string>) {
  const params = new URLSearchParams({ from: period.from, to: period.to, ...extra });
  return `/activity?${params.toString()}`;
}

function Empty({ message }: { message: string }) {
  return <TableCardState className="grid min-h-40 place-items-center text-center"><p className="text-sm text-muted-foreground">{message}</p></TableCardState>;
}

export function OverviewAgentsTable({ rows, period }: { rows: OverviewAgentRow[]; period: { from: string; to: string } }) {
  return <TableCard title="Assistants" description="Busiest assistants in this period.">
    {rows.length ? <Table>
      <TableHeader><TableRow><TableHead>Assistant</TableHead><TableHead className="text-right">Requests</TableHead><TableHead className="text-right">Completed</TableHead><TableHead className="hidden text-right lg:table-cell">Slowest responses</TableHead><TableHead className="hidden text-right xl:table-cell">Work processed</TableHead><TableHead className="text-right"><span className="sr-only">Open</span></TableHead></TableRow></TableHeader>
      <TableBody>{rows.map((row) => <TableRow key={row.agent_id}>
        <TableCell className="font-medium">{row.agent_name}</TableCell>
        <TableCell className="text-right tabular-nums">{numberFormat.format(row.requests)}</TableCell>
        <TableCell className="text-right tabular-nums">{percent(row.success_rate)}{row.failed ? <span className="ml-2 text-xs text-muted-foreground">{numberFormat.format(row.failed)} failed</span> : null}</TableCell>
        <TableCell className="hidden text-right tabular-nums lg:table-cell">{duration(row.p95_duration_ms)}</TableCell>
        <TableCell className="hidden text-right tabular-nums xl:table-cell">{numberFormat.format(row.processing_units)}</TableCell>
        <TableCell className="text-right"><Button asChild variant="ghost" size="sm"><Link to={activityLink(period, { agent: row.agent_id })}>View requests <ArrowRight /></Link></Button></TableCell>
      </TableRow>)}</TableBody>
    </Table> : <Empty message="No assistant handled a request in this period." />}
  </TableCard>;
}

export function OverviewErrorsTable({ rows, period }: { rows: OverviewErrorRow[]; period: { from: string; to: string } }) {
  return <TableCard title="What is failing" description="Most frequent reasons requests did not finish.">
    {rows.length ? <Table>
      <TableHeader><TableRow><TableHead>Reason</TableHead><TableHead className="text-right">Requests</TableHead><TableHead className="hidden lg:table-cell">Last seen</TableHead><TableHead className="text-right"><span className="sr-only">Open</span></TableHead></TableRow></TableHeader>
      <TableBody>{rows.map((row) => {
        const action = problemToAction(row.error_code as ProblemCode);
        return <TableRow key={row.error_code}>
          <TableCell><Link className="font-medium hover:text-primary hover:underline" to={activityLink(period, { status: "failed" })}>{action.title}</Link><p className="mt-0.5 text-xs text-muted-foreground">{action.description}</p></TableCell>
          <TableCell className="text-right tabular-nums">{numberFormat.format(row.count)}</TableCell>
          <TableCell className="hidden whitespace-nowrap text-sm lg:table-cell">{formatDate(row.last_seen_at)}</TableCell>
          <TableCell className="text-right"><Button asChild variant="ghost" size="sm"><Link to={`/activity/${row.sample_run_id}`}>View an example <ArrowRight /></Link></Button></TableCell>
        </TableRow>;
      })}</TableBody>
    </Table> : <Empty message="Nothing failed in this period." />}
  </TableCard>;
}

export function OverviewToolsTable({ rows }: { rows: OverviewToolRow[] }) {
  return <TableCard title="Tools" description="How often assistants used each tool, and how often it failed.">
    {rows.length ? <Table>
      <TableHeader><TableRow><TableHead>Tool</TableHead><TableHead className="text-right">Uses</TableHead><TableHead className="text-right">Failures</TableHead><TableHead className="hidden text-right lg:table-cell">Slowest responses</TableHead></TableRow></TableHeader>
      <TableBody>{rows.map((row) => <TableRow key={row.tool_name}>
        <TableCell className="font-medium">{row.tool_name}</TableCell>
        <TableCell className="text-right tabular-nums">{numberFormat.format(row.calls)}</TableCell>
        <TableCell className="text-right tabular-nums">{row.errors ? <Badge variant="destructive">{numberFormat.format(row.errors)}</Badge> : "0"}</TableCell>
        <TableCell className="hidden text-right tabular-nums lg:table-cell">{duration(row.p95_duration_ms)}</TableCell>
      </TableRow>)}</TableBody>
    </Table> : <Empty message="No assistant used a tool in this period." />}
  </TableCard>;
}
