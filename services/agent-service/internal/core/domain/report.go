package domain

import (
	"time"

	"github.com/google/uuid"
)

// OverviewRange is the window and day boundary the server actually applied.
type OverviewRange struct {
	From     time.Time
	To       time.Time
	TimeZone string
}

// OverviewTotals summarises every run in the window.
// SuccessRate is nil without requests; durations are nil below the sample floor.
type OverviewTotals struct {
	Requests        int64
	Succeeded       int64
	Failed          int64
	Cancelled       int64
	SuccessRate     *float64
	Sessions        int64
	AvgDurationMs   *int64
	P95DurationMs   *int64
	ProcessingUnits int64
}

// OverviewDailyPoint counts one calendar day in the requested time zone.
type OverviewDailyPoint struct {
	Date       time.Time
	Playground int64
	API        int64
	Failed     int64
}

// OverviewAgentRow ranks one assistant inside the window.
type OverviewAgentRow struct {
	AgentID         uuid.UUID
	AgentName       string
	Requests        int64
	Succeeded       int64
	Failed          int64
	SuccessRate     *float64
	P95DurationMs   *int64
	ProcessingUnits int64
}

// OverviewErrorRow groups failures by the stable error code.
type OverviewErrorRow struct {
	ErrorCode   string
	Count       int64
	LastSeenAt  time.Time
	SampleRunID uuid.UUID
}

// OverviewToolRow groups tool calls by the name recorded when they ran.
type OverviewToolRow struct {
	ToolName      string
	Calls         int64
	Errors        int64
	P95DurationMs *int64
}

// OverviewReport is the whole overview screen in one value.
type OverviewReport struct {
	Range         OverviewRange
	Totals        OverviewTotals
	Daily         []OverviewDailyPoint
	TopAgents     []OverviewAgentRow
	TopErrors     []OverviewErrorRow
	TopTools      []OverviewToolRow
	StepLimitHits int64
}
