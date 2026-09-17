//go:build integration

package postgres

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// saigon is seven hours ahead of UTC, so a late-evening UTC run belongs to the
// next local day. Grouping on UTC alone would file it under the wrong day.
const saigon = "Asia/Ho_Chi_Minh"

func reportFixture(t *testing.T) (*Store, domain.Agent, uuid.UUID) {
	t.Helper()
	store, provider, agent := catalogFixture(t)
	ctx := t.Context()
	if err := store.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(ctx, agent); err != nil {
		t.Fatal(err)
	}
	session := domain.Session{ID: uuid.New(), AgentID: agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &agent.CreatedBy, Title: "Report", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, err := store.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	return store, agent, session.ID
}

func seedRun(t *testing.T, store *Store, agent domain.Agent, sessionID uuid.UUID, run domain.Run) domain.Run {
	t.Helper()
	run.ID = uuid.New()
	run.AgentID = agent.ID
	run.SessionID = sessionID
	run.TriggeredByUserID = &agent.CreatedBy
	run.Input = domain.RunInput{Message: "hello"}
	run.Metadata = json.RawMessage(`{}`)
	if run.Mode == "" {
		run.Mode = domain.RunModeSync
	}
	if err := store.CreateRun(t.Context(), run); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestOverviewReportCountsStatusesSessionsAndProcessingUnits(t *testing.T) {
	store, agent, sessionID := reportFixture(t)
	day := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	started, finished := day, day.Add(2*time.Second)
	in, out := 30, 12
	seedRun(t, store, agent, sessionID, domain.Run{Status: domain.RunSucceeded, Source: domain.RunSourcePlayground, CreatedAt: day, StartedAt: &started, FinishedAt: &finished, Usage: domain.TokenUsage{InputTokens: &in, OutputTokens: &out}})
	seedRun(t, store, agent, sessionID, domain.Run{Status: domain.RunFailed, Source: domain.RunSourceAPI, CreatedAt: day.Add(time.Hour), Failure: &domain.RunFailure{Code: "tool_failed", Message: "boom"}})
	seedRun(t, store, agent, sessionID, domain.Run{Status: domain.RunCancelled, Source: domain.RunSourceAPI, CreatedAt: day.Add(2 * time.Hour)})
	// Outside the window on purpose: it must not reach any total.
	seedRun(t, store, agent, sessionID, domain.Run{Status: domain.RunSucceeded, Source: domain.RunSourcePlayground, CreatedAt: day.AddDate(0, 0, -30)})

	window := outbound.OverviewWindow{From: day.Add(-time.Hour), To: day.Add(24 * time.Hour), TimeZone: "UTC", RowLimit: 10}
	sample, err := store.OverviewReport(t.Context(), window)
	if err != nil {
		t.Fatal(err)
	}
	totals := sample.Report.Totals
	if totals.Requests != 3 || totals.Succeeded != 1 || totals.Failed != 1 || totals.Cancelled != 1 {
		t.Fatalf("totals=%+v", totals)
	}
	if totals.Sessions != 1 || totals.ProcessingUnits != 42 {
		t.Fatalf("sessions=%d units=%d", totals.Sessions, totals.ProcessingUnits)
	}
	if sample.FinishedRuns != 1 || totals.P95DurationMs == nil || *totals.P95DurationMs != 2000 {
		t.Fatalf("finished=%d p95=%v", sample.FinishedRuns, totals.P95DurationMs)
	}
	if len(sample.Report.TopErrors) != 1 || sample.Report.TopErrors[0].ErrorCode != "tool_failed" || sample.Report.TopErrors[0].Count != 1 {
		t.Fatalf("errors=%+v", sample.Report.TopErrors)
	}
	if len(sample.Report.TopAgents) != 1 || sample.Report.TopAgents[0].Requests != 3 || sample.AgentFinishedRuns[0] != 1 {
		t.Fatalf("agents=%+v finished=%v", sample.Report.TopAgents, sample.AgentFinishedRuns)
	}
}

func TestOverviewReportKeepsQuietDaysAndFollowsTheRequestedTimeZone(t *testing.T) {
	store, agent, sessionID := reportFixture(t)
	// 23:30 UTC on 10 Sep is 06:30 on 11 Sep in Saigon.
	late := time.Date(2026, 9, 10, 23, 30, 0, 0, time.UTC)
	seedRun(t, store, agent, sessionID, domain.Run{Status: domain.RunSucceeded, Source: domain.RunSourcePlayground, CreatedAt: late})

	from := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	utc, err := store.OverviewReport(t.Context(), outbound.OverviewWindow{From: from, To: to, TimeZone: "UTC", RowLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(utc.Report.Daily) != 3 {
		t.Fatalf("days=%d; want one point per day including quiet ones", len(utc.Report.Daily))
	}
	if utc.Report.Daily[0].Playground != 0 || utc.Report.Daily[1].Playground != 1 || utc.Report.Daily[2].Playground != 0 {
		t.Fatalf("utc daily=%+v", utc.Report.Daily)
	}

	local, err := store.OverviewReport(t.Context(), outbound.OverviewWindow{From: from, To: to, TimeZone: saigon, RowLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if local.Report.Daily[1].Playground != 0 || local.Report.Daily[2].Playground != 1 {
		t.Fatalf("saigon daily=%+v; run should fall on the next local day", local.Report.Daily)
	}
}

func TestOverviewReportRanksToolsAndCountsStepLimitHits(t *testing.T) {
	store, agent, sessionID := reportFixture(t)
	day := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	capped := seedRun(t, store, agent, sessionID, domain.Run{Status: domain.RunSucceeded, Source: domain.RunSourcePlayground, CreatedAt: day, Iterations: agent.MaxIterations})
	seedRun(t, store, agent, sessionID, domain.Run{Status: domain.RunSucceeded, Source: domain.RunSourcePlayground, CreatedAt: day, Iterations: 1})

	name := "lookup_invoice"
	for i, status := range []domain.SpanStatus{domain.SpanOK, domain.SpanError} {
		span := domain.Span{ID: uuid.New(), RunID: capped.ID, Kind: domain.SpanToolCall, Name: name, Status: status, ToolName: &name, StartedAt: day.Add(time.Duration(i) * time.Second), EndedAt: day.Add(time.Duration(i)*time.Second + 500*time.Millisecond), DurationMS: 500, Attributes: json.RawMessage(`{}`)}
		if err := store.CreateSpan(t.Context(), span); err != nil {
			t.Fatal(err)
		}
	}
	// An LLM span must never reach the tool ranking.
	model := "model"
	llm := domain.Span{ID: uuid.New(), RunID: capped.ID, Kind: domain.SpanLLMCall, Name: model, Status: domain.SpanOK, Model: &model, StartedAt: day, EndedAt: day.Add(time.Second), DurationMS: 1000, Attributes: json.RawMessage(`{}`)}
	if err := store.CreateSpan(t.Context(), llm); err != nil {
		t.Fatal(err)
	}

	sample, err := store.OverviewReport(t.Context(), outbound.OverviewWindow{From: day.Add(-time.Hour), To: day.Add(time.Hour), TimeZone: "UTC", RowLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if sample.Report.StepLimitHits != 1 {
		t.Fatalf("step limit hits=%d", sample.Report.StepLimitHits)
	}
	if len(sample.Report.TopTools) != 1 {
		t.Fatalf("tools=%+v; llm spans must stay out", sample.Report.TopTools)
	}
	tool := sample.Report.TopTools[0]
	if tool.ToolName != name || tool.Calls != 2 || tool.Errors != 1 || tool.P95DurationMs == nil || *tool.P95DurationMs != 500 {
		t.Fatalf("tool=%+v", tool)
	}
}
