import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { ArrowRight, ListFilter } from "lucide-react";
import { useState } from "react";
import { Link, useSearchParams } from "react-router";

import { agentQueries } from "@/entities/agent";
import { runQueries, type RunFilters, type RunStatus } from "@/entities/run";
import { formatDate, formatDuration, problemToAction } from "@/shared/lib";
import { Alert, AlertDescription, AlertTitle, Badge, Button, Card, CardContent, Input, Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Skeleton, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui";

const statusLabel: Record<RunStatus, string> = { queued: "Queued", running: "Processing", succeeded: "Completed", failed: "Failed", cancelled: "Stopped" };

function statusVariant(status: RunStatus) {
  if (status === "succeeded") return "success" as const;
  if (status === "failed") return "destructive" as const;
  if (status === "queued" || status === "running") return "warning" as const;
  return "outline" as const;
}

export function ActivityPage() {
  const [searchParams] = useSearchParams();
  const [filters, setFilters] = useState<RunFilters>(() => ({ agentId: searchParams.get("agent") ?? undefined }));
  const runs = useInfiniteQuery(runQueries.infiniteList(filters));
  const agents = useQuery(agentQueries.list());
  const names = new Map(agents.data?.map((agent) => [agent.id, agent.name]));
  const items = runs.data?.pages.flatMap((page) => page.items) ?? [];
  const errorAction = problemToAction((runs.error as { code?: string })?.code);
  const hasFilters = Boolean(filters.status || filters.agentId || filters.source || filters.from || filters.to);
  const updateFilters = (next: Partial<RunFilters>) => setFilters((current) => ({ ...current, ...next, cursor: undefined }));

  return <div className="space-y-5">
    <div><h1 className="text-2xl font-semibold">Activity</h1><p className="mt-1 text-sm text-muted-foreground">Open any request to follow the agent loop and understand each observable decision.</p></div>

    <Card>
      <CardContent className="pt-5 sm:pt-6">
        <div className="mb-4 flex items-center gap-2 text-sm font-semibold"><ListFilter className="size-4" />Filter activity</div>
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
          <div className="space-y-2"><Label htmlFor="activity-status">Status</Label><Select value={filters.status ?? "all"} onValueChange={(value) => updateFilters({ status: value === "all" ? undefined : value as RunStatus })}><SelectTrigger id="activity-status"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">All statuses</SelectItem>{Object.entries(statusLabel).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent></Select></div>
          <div className="space-y-2"><Label htmlFor="activity-agent">Assistant</Label><Select value={filters.agentId ?? "all"} onValueChange={(value) => updateFilters({ agentId: value === "all" ? undefined : value })}><SelectTrigger id="activity-agent"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">All assistants</SelectItem>{agents.data?.map((agent) => <SelectItem key={agent.id} value={agent.id}>{agent.name}</SelectItem>)}</SelectContent></Select></div>
          <div className="space-y-2"><Label htmlFor="activity-source">Source</Label><Select value={filters.source ?? "all"} onValueChange={(value) => updateFilters({ source: value === "all" ? undefined : value as RunFilters["source"] })}><SelectTrigger id="activity-source"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">All sources</SelectItem><SelectItem value="playground">Admin portal</SelectItem><SelectItem value="api">External system</SelectItem></SelectContent></Select></div>
          <div className="space-y-2"><Label htmlFor="activity-from">From</Label><Input id="activity-from" type="datetime-local" onChange={(event) => updateFilters({ from: event.target.value ? new Date(event.target.value).toISOString() : undefined })} /></div>
          <div className="space-y-2"><Label htmlFor="activity-to">To</Label><Input id="activity-to" type="datetime-local" onChange={(event) => updateFilters({ to: event.target.value ? new Date(event.target.value).toISOString() : undefined })} /></div>
        </div>
      </CardContent>
    </Card>

    <Card><CardContent className="pt-5 sm:pt-6">
      {runs.isPending ? <div className="space-y-3"><Skeleton className="h-11 w-full" /><Skeleton className="h-16 w-full" /><Skeleton className="h-16 w-full" /></div> : runs.isError ? <Alert variant="destructive"><AlertTitle>{errorAction.title}</AlertTitle><AlertDescription>{errorAction.description} <button className="font-medium underline" onClick={() => void runs.refetch()}>{errorAction.action?.label ?? "Try again"}</button></AlertDescription></Alert> : items.length ? <Table>
        <TableHeader><TableRow><TableHead>Status</TableHead><TableHead>Request</TableHead><TableHead>Assistant</TableHead><TableHead>Loop / usage</TableHead><TableHead>Started</TableHead><TableHead className="hidden lg:table-cell">Duration</TableHead><TableHead className="hidden xl:table-cell">Source</TableHead><TableHead><span className="sr-only">Open</span></TableHead></TableRow></TableHeader>
        <TableBody>{items.map((run) => {
          const elapsed = run.started_at && run.finished_at ? new Date(run.finished_at).valueOf() - new Date(run.started_at).valueOf() : null;
          const usage = run.usage.input_tokens === null || run.usage.output_tokens === null ? null : run.usage.input_tokens + run.usage.output_tokens;
          return <TableRow key={run.id}>
            <TableCell><Badge variant={statusVariant(run.status)}>{statusLabel[run.status]}</Badge></TableCell>
            <TableCell className="max-w-72"><Link className="line-clamp-2 min-w-48 font-medium hover:text-primary hover:underline" to={`/activity/${run.id}`}>{run.input.message || "Empty request"}</Link></TableCell>
            <TableCell>{names.get(run.agent_id) ?? "Archived assistant"}</TableCell>
            <TableCell><p className="font-medium tabular-nums">{run.iterations} model {run.iterations === 1 ? "pass" : "passes"}</p><p className="mt-0.5 text-xs text-muted-foreground tabular-nums">{usage === null ? "Usage not reported" : `${usage.toLocaleString("en-US")} processing units`}</p></TableCell>
            <TableCell className="whitespace-nowrap text-sm">{formatDate(run.started_at ?? run.created_at)}</TableCell>
            <TableCell className="hidden whitespace-nowrap tabular-nums lg:table-cell">{elapsed === null ? run.status === "queued" || run.status === "running" ? "In progress" : "Not available" : formatDuration(Math.max(0, elapsed))}</TableCell>
            <TableCell className="hidden xl:table-cell">{run.source === "playground" ? "Admin portal" : "External system"}</TableCell>
            <TableCell><Button asChild variant="ghost" size="sm"><Link to={`/activity/${run.id}`}>View loop <ArrowRight /></Link></Button></TableCell>
          </TableRow>;
        })}</TableBody>
      </Table> : <div className="grid min-h-52 place-items-center text-center"><div><p className="font-medium">No activity matches these filters.</p><p className="mt-1 text-sm text-muted-foreground">Adjust the filters to see more requests.</p>{hasFilters ? <Button className="mt-4" variant="outline" onClick={() => setFilters({})}>Clear filters</Button> : null}</div></div>}
    </CardContent></Card>
    {runs.hasNextPage ? <Button variant="outline" disabled={runs.isFetchingNextPage} onClick={() => void runs.fetchNextPage()}>{runs.isFetchingNextPage ? "Loading…" : "Load more activity"}</Button> : null}
  </div>;
}
