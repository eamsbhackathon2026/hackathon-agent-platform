// Package reports aggregates workspace activity for the overview screen.
package reports

import (
	"context"
	"strings"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

const (
	defaultWindow = 7 * 24 * time.Hour
	maxWindow     = 90 * 24 * time.Hour
	// rowLimit caps every ranking table so one busy workspace cannot return
	// thousands of rows to a screen that shows ten.
	rowLimit = 10
	// sampleFloor is the number of observations a percentile needs before it
	// describes anything. Below it the report reports nothing rather than a
	// number readers would act on.
	sampleFloor = 20
)

// Service answers overview questions for owners and administrators.
type Service struct {
	Reports outbound.ReportRepository
	Clock   outbound.Clock
}

// NewService binds the aggregation repository and the clock.
func NewService(reports outbound.ReportRepository, clock outbound.Clock) Service {
	return Service{Reports: reports, Clock: clock}
}

// Overview aggregates one window of workspace activity.
func (s Service) Overview(ctx context.Context, p domain.Principal, request inbound.OverviewRequest) (domain.OverviewReport, error) {
	if err := domain.Authorize(p, domain.ActionReportsRead, nil); err != nil {
		return domain.OverviewReport{}, err
	}
	window, err := s.window(request)
	if err != nil {
		return domain.OverviewReport{}, err
	}
	sample, err := s.Reports.OverviewReport(ctx, window)
	if err != nil {
		return domain.OverviewReport{}, err
	}
	report := sample.Report
	report.Range = domain.OverviewRange{From: window.From, To: window.To, TimeZone: window.TimeZone}
	report.Totals.SuccessRate = rate(report.Totals.Succeeded, report.Totals.Requests)
	if sample.FinishedRuns < sampleFloor {
		report.Totals.AvgDurationMs = nil
		report.Totals.P95DurationMs = nil
	}
	for i := range report.TopAgents {
		row := &report.TopAgents[i]
		row.SuccessRate = rate(row.Succeeded, row.Requests)
		if i >= len(sample.AgentFinishedRuns) || sample.AgentFinishedRuns[i] < sampleFloor {
			row.P95DurationMs = nil
		}
	}
	for i := range report.TopTools {
		if report.TopTools[i].Calls < sampleFloor {
			report.TopTools[i].P95DurationMs = nil
		}
	}
	return report, nil
}

// window fills defaults and rejects windows that would scan the whole table.
func (s Service) window(request inbound.OverviewRequest) (outbound.OverviewWindow, error) {
	to := s.Clock.Now().UTC()
	if request.To != nil {
		to = request.To.UTC()
	}
	from := to.Add(-defaultWindow)
	if request.From != nil {
		from = request.From.UTC()
	}
	if !from.Before(to) {
		return outbound.OverviewWindow{}, domain.Invalid("from", "Thời điểm bắt đầu phải trước thời điểm kết thúc.")
	}
	if to.Sub(from) > maxWindow {
		return outbound.OverviewWindow{}, domain.Invalid("from", "Khoảng thời gian tối đa là 90 ngày.")
	}
	zone := "UTC"
	if request.TimeZone != nil && *request.TimeZone != "" {
		zone = *request.TimeZone
	}
	if !namedZone(zone) {
		return outbound.OverviewWindow{}, domain.Invalid("time_zone", "Múi giờ không hợp lệ.")
	}
	return outbound.OverviewWindow{From: from, To: to, TimeZone: zone, RowLimit: rowLimit}, nil
}

// namedZone accepts only zones that name a place. Go resolves "Local", "Factory"
// and "posixrules" as well, but those mean the server's own zone or nothing at
// all: Postgres rejects "Local" outright, and the others would silently bucket
// days on a boundary no viewer asked for.
func namedZone(zone string) bool {
	if zone != "UTC" && !strings.Contains(zone, "/") {
		return false
	}
	_, err := time.LoadLocation(zone)
	return err == nil
}

func rate(part, total int64) *float64 {
	if total <= 0 {
		return nil
	}
	value := float64(part) / float64(total)
	return &value
}
