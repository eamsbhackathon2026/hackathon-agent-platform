package http

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// ReportHandler exposes workspace reporting to owners and administrators.
type ReportHandler struct{ reports inbound.ReportUseCase }

// NewReportHandler binds reporting to its use case.
func NewReportHandler(reports inbound.ReportUseCase) *ReportHandler {
	return &ReportHandler{reports: reports}
}

// GetOverviewReport returns one aggregated activity window.
func (h *ReportHandler) GetOverviewReport(ctx context.Context, request gen.GetOverviewReportRequestObject) (gen.GetOverviewReportResponseObject, error) {
	report, err := h.reports.Overview(ctx, requestFrom(ctx).principal, inbound.OverviewRequest{
		From:     request.Params.From,
		To:       request.Params.To,
		TimeZone: request.Params.TimeZone,
	})
	if err != nil {
		return nil, err
	}
	return gen.GetOverviewReport200JSONResponse(overviewDTO(report)), nil
}

func overviewDTO(v domain.OverviewReport) gen.OverviewReport {
	daily := make([]gen.OverviewDailyPoint, len(v.Daily))
	for i, point := range v.Daily {
		daily[i] = gen.OverviewDailyPoint{Date: openapi_types.Date{Time: point.Date}, Playground: point.Playground, Api: point.API, Failed: point.Failed}
	}
	agents := make([]gen.OverviewAgentRow, len(v.TopAgents))
	for i, row := range v.TopAgents {
		agents[i] = gen.OverviewAgentRow{
			AgentId:         row.AgentID,
			AgentName:       row.AgentName,
			Requests:        row.Requests,
			Succeeded:       row.Succeeded,
			Failed:          row.Failed,
			SuccessRate:     nullableValue(row.SuccessRate),
			P95DurationMs:   nullableValue(row.P95DurationMs),
			ProcessingUnits: row.ProcessingUnits,
		}
	}
	failures := make([]gen.OverviewErrorRow, len(v.TopErrors))
	for i, row := range v.TopErrors {
		failures[i] = gen.OverviewErrorRow{ErrorCode: gen.ProblemCode(row.ErrorCode), Count: row.Count, LastSeenAt: row.LastSeenAt, SampleRunId: row.SampleRunID}
	}
	tools := make([]gen.OverviewToolRow, len(v.TopTools))
	for i, row := range v.TopTools {
		tools[i] = gen.OverviewToolRow{ToolName: row.ToolName, Calls: row.Calls, Errors: row.Errors, P95DurationMs: nullableValue(row.P95DurationMs)}
	}
	return gen.OverviewReport{
		Range: gen.OverviewRange{From: v.Range.From, To: v.Range.To, TimeZone: v.Range.TimeZone},
		Totals: gen.OverviewTotals{
			Requests:        v.Totals.Requests,
			Succeeded:       v.Totals.Succeeded,
			Failed:          v.Totals.Failed,
			Cancelled:       v.Totals.Cancelled,
			SuccessRate:     nullableValue(v.Totals.SuccessRate),
			Sessions:        v.Totals.Sessions,
			AvgDurationMs:   nullableValue(v.Totals.AvgDurationMs),
			P95DurationMs:   nullableValue(v.Totals.P95DurationMs),
			ProcessingUnits: v.Totals.ProcessingUnits,
		},
		Daily:         daily,
		TopAgents:     agents,
		TopErrors:     failures,
		TopTools:      tools,
		StepLimitHits: v.StepLimitHits,
	}
}
