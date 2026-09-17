package outbound

import (
	"context"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
)

// OverviewWindow is a resolved, already-validated aggregation window.
type OverviewWindow struct {
	From     time.Time
	To       time.Time
	TimeZone string
	RowLimit int32
}

// OverviewSample carries a count alongside a metric so the service can drop
// percentiles that rest on too few observations to mean anything.
type OverviewSample struct {
	Report domain.OverviewReport
	// FinishedRuns counts runs with both timestamps set, across the window.
	FinishedRuns int64
	// AgentFinishedRuns counts the same per ranked assistant, in TopAgents order.
	AgentFinishedRuns []int64
}

// ReportRepository aggregates runs, spans and sessions for the overview report.
type ReportRepository interface {
	OverviewReport(context.Context, OverviewWindow) (OverviewSample, error)
}
