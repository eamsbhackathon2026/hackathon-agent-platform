package postgres

import (
	"context"
	"math"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// OverviewReport aggregates runs, spans and sessions for one window.
// Rates and sample floors stay in the service; this returns raw observations.
func (s *Store) OverviewReport(ctx context.Context, w outbound.OverviewWindow) (outbound.OverviewSample, error) {
	q := s.queries(ctx)
	from, to := dbTime(w.From), dbTime(w.To)

	totals, err := q.OverviewTotals(ctx, sqlcgen.OverviewTotalsParams{FromTime: from, ToTime: to})
	if err != nil {
		return outbound.OverviewSample{}, mapError(err)
	}
	hits, err := q.OverviewStepLimitHits(ctx, sqlcgen.OverviewStepLimitHitsParams{FromTime: from, ToTime: to})
	if err != nil {
		return outbound.OverviewSample{}, mapError(err)
	}
	daily, err := q.OverviewDaily(ctx, sqlcgen.OverviewDailyParams{FromTime: from, ToTime: to, TimeZone: w.TimeZone})
	if err != nil {
		return outbound.OverviewSample{}, mapError(err)
	}
	agents, err := q.OverviewTopAgents(ctx, sqlcgen.OverviewTopAgentsParams{FromTime: from, ToTime: to, RowLimit: w.RowLimit})
	if err != nil {
		return outbound.OverviewSample{}, mapError(err)
	}
	failures, err := q.OverviewTopErrors(ctx, sqlcgen.OverviewTopErrorsParams{FromTime: from, ToTime: to, RowLimit: w.RowLimit})
	if err != nil {
		return outbound.OverviewSample{}, mapError(err)
	}
	tools, err := q.OverviewTopTools(ctx, sqlcgen.OverviewTopToolsParams{FromTime: from, ToTime: to, RowLimit: w.RowLimit})
	if err != nil {
		return outbound.OverviewSample{}, mapError(err)
	}

	sample := outbound.OverviewSample{FinishedRuns: totals.Finished}
	sample.Report = domain.OverviewReport{
		Totals: domain.OverviewTotals{
			Requests:        totals.Requests,
			Succeeded:       totals.Succeeded,
			Failed:          totals.Failed,
			Cancelled:       totals.Cancelled,
			Sessions:        totals.Sessions,
			AvgDurationMs:   milliseconds(totals.AvgDurationMs),
			P95DurationMs:   milliseconds(totals.P95DurationMs),
			ProcessingUnits: totals.ProcessingUnits,
		},
		Daily:         make([]domain.OverviewDailyPoint, len(daily)),
		TopAgents:     make([]domain.OverviewAgentRow, len(agents)),
		TopErrors:     make([]domain.OverviewErrorRow, len(failures)),
		TopTools:      make([]domain.OverviewToolRow, len(tools)),
		StepLimitHits: hits,
	}
	for i, point := range daily {
		sample.Report.Daily[i] = domain.OverviewDailyPoint{Date: point.Day.Time, Playground: point.Playground, API: point.Api, Failed: point.Failed}
	}
	sample.AgentFinishedRuns = make([]int64, len(agents))
	for i, row := range agents {
		sample.AgentFinishedRuns[i] = row.Finished
		sample.Report.TopAgents[i] = domain.OverviewAgentRow{
			AgentID:         row.AgentID.Bytes,
			AgentName:       row.AgentName,
			Requests:        row.Requests,
			Succeeded:       row.Succeeded,
			Failed:          row.Failed,
			P95DurationMs:   milliseconds(row.P95DurationMs),
			ProcessingUnits: row.ProcessingUnits,
		}
	}
	for i, row := range failures {
		sample.Report.TopErrors[i] = domain.OverviewErrorRow{ErrorCode: row.ErrorCode, Count: row.Occurrences, LastSeenAt: row.LastSeenAt.Time, SampleRunID: row.SampleRunID.Bytes}
	}
	for i, row := range tools {
		sample.Report.TopTools[i] = domain.OverviewToolRow{ToolName: row.ToolName, Calls: row.Calls, Errors: row.Errors, P95DurationMs: milliseconds(row.P95DurationMs)}
	}
	return sample, nil
}

// milliseconds rounds a Postgres duration aggregate to whole milliseconds.
func milliseconds(value float64) *int64 {
	rounded := int64(math.Round(value))
	if rounded < 0 {
		rounded = 0
	}
	return &rounded
}
