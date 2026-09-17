import { useQuery } from "@tanstack/react-query";
import { ArrowRight, TriangleAlert } from "lucide-react";
import { useMemo } from "react";
import { Link, useSearchParams } from "react-router";

import { isOverviewRange, overviewRangeLabels, overviewWindow, reportQueries, type OverviewRange } from "@/entities/report";
import { problemToAction } from "@/shared/lib";
import { Alert, AlertDescription, AlertTitle, Button, Card, CardContent, Skeleton, Tabs, TabsList, TabsTrigger } from "@/shared/ui";

import { OverviewKpis } from "./overview-kpis";
import { OverviewAgentsTable, OverviewErrorsTable, OverviewToolsTable } from "./overview-tables";
import { OverviewTrendChart } from "./overview-trend-chart";

const ranges: OverviewRange[] = ["24h", "7d", "30d"];

export function OverviewPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const rangeParam = searchParams.get("range");
  const range: OverviewRange = isOverviewRange(rangeParam) ? rangeParam : "7d";
  // The window is pinned per range so refetches keep comparing the same period.
  const period = useMemo(() => overviewWindow(range), [range]);
  const overview = useQuery(reportQueries.overview(period));

  const selectRange = (next: string) => {
    if (!isOverviewRange(next)) return;
    const params = new URLSearchParams(searchParams);
    params.set("range", next);
    setSearchParams(params, { replace: true });
  };

  if (overview.isPending) {
    return <div className="space-y-5"><Skeleton className="h-10 w-72" /><Skeleton className="h-28 w-full" /><Skeleton className="h-72 w-full" /></div>;
  }
  if (overview.isError || !overview.data) {
    const action = problemToAction((overview.error as { code?: string })?.code);
    return <Alert variant="destructive">
      <AlertTitle>{action.title}</AlertTitle>
      <AlertDescription className="mt-2">{action.description} <button className="font-medium underline" onClick={() => void overview.refetch()}>{action.action?.label ?? "Try again"}</button></AlertDescription>
    </Alert>;
  }

  const report = overview.data;
  const quiet = report.totals.requests === 0;

  return <div className="space-y-5">
    <Tabs value={range} onValueChange={selectRange}>
      <TabsList aria-label="Reporting period">{ranges.map((value) => <TabsTrigger key={value} value={value}>{overviewRangeLabels[value]}</TabsTrigger>)}</TabsList>
    </Tabs>

    {quiet ? <Card>
      <CardContent className="grid min-h-52 place-items-center pt-5 text-center sm:pt-6">
        <div>
          <p className="font-medium">No activity in {overviewRangeLabels[range].toLowerCase()}.</p>
          <p className="mt-1 text-sm text-muted-foreground">Send a request from the Playground, or pick a longer period to see earlier activity.</p>
          <Button asChild className="mt-4"><Link to="/playground">Open Playground <ArrowRight /></Link></Button>
        </div>
      </CardContent>
    </Card> : <>
      <OverviewKpis totals={report.totals} />
      <OverviewTrendChart daily={report.daily} />
      {report.step_limit_hits > 0 ? <Alert>
        <TriangleAlert />
        <AlertTitle>{report.step_limit_hits} {report.step_limit_hits === 1 ? "request" : "requests"} ran out of steps before finishing</AlertTitle>
        <AlertDescription className="mt-2">
          An assistant stopped because it reached its step limit. Review the instructions or raise the limit on the assistant.{" "}
          <Link className="font-medium underline" to={`/activity?from=${encodeURIComponent(period.from)}&to=${encodeURIComponent(period.to)}`}>Review these requests</Link>
        </AlertDescription>
      </Alert> : null}
      <OverviewAgentsTable rows={report.top_agents} period={period} />
      <OverviewErrorsTable rows={report.top_errors} period={period} />
      <OverviewToolsTable rows={report.top_tools} />
    </>}
  </div>;
}
