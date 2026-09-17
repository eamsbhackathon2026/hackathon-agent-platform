package inbound

import (
	"context"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
)

// OverviewRequest carries the raw window before defaults are applied.
type OverviewRequest struct {
	From     *time.Time
	To       *time.Time
	TimeZone *string
}

// ReportUseCase exposes workspace-wide reporting to owners and administrators.
type ReportUseCase interface {
	Overview(context.Context, domain.Principal, OverviewRequest) (domain.OverviewReport, error)
}
