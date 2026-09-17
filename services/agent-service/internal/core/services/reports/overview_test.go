package reports

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

type fakeRepo struct {
	window outbound.OverviewWindow
	sample outbound.OverviewSample
	err    error
}

func (r *fakeRepo) OverviewReport(_ context.Context, w outbound.OverviewWindow) (outbound.OverviewSample, error) {
	r.window = w
	return r.sample, r.err
}

var now = time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)

func adminPrincipal(t *testing.T) domain.Principal {
	t.Helper()
	return domain.Principal{Kind: domain.PrincipalUser, UserID: [16]byte{1}, Role: domain.RoleAdmin}
}

func newService(repo *fakeRepo) Service {
	return NewService(repo, fakeClock{now: now})
}

func TestOverviewRejectsEveryPrincipalBelowAdmin(t *testing.T) {
	member := domain.Principal{Kind: domain.PrincipalUser, UserID: [16]byte{2}, Role: domain.RoleMember}
	key := domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: [16]byte{3}, Scopes: []string{"runs:read"}}
	for name, p := range map[string]domain.Principal{"member": member, "api key": key} {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{}
			if _, err := newService(repo).Overview(t.Context(), p, inbound.OverviewRequest{}); !errors.Is(err, domain.ErrForbidden) {
				t.Fatalf("got %v; want forbidden", err)
			}
			if repo.window.RowLimit != 0 {
				t.Fatal("repository was reached despite a refused principal")
			}
		})
	}
}

func TestOverviewDefaultsToTheLastSevenDaysInUTC(t *testing.T) {
	repo := &fakeRepo{}
	if _, err := newService(repo).Overview(t.Context(), adminPrincipal(t), inbound.OverviewRequest{}); err != nil {
		t.Fatal(err)
	}
	if !repo.window.To.Equal(now) || !repo.window.From.Equal(now.AddDate(0, 0, -7)) {
		t.Fatalf("window=%v..%v", repo.window.From, repo.window.To)
	}
	if repo.window.TimeZone != "UTC" || repo.window.RowLimit != rowLimit {
		t.Fatalf("zone=%q limit=%d", repo.window.TimeZone, repo.window.RowLimit)
	}
}

func TestOverviewRejectsUnusableWindows(t *testing.T) {
	from := now.Add(time.Hour)
	tooEarly := now.AddDate(0, 0, -91)
	// Go resolves the last three, but they name the server's own zone or no place
	// at all, and Postgres rejects "Local" outright.
	unknown, local, factory, posix := "Mars/Olympus", "Local", "Factory", "posixrules"
	for name, request := range map[string]inbound.OverviewRequest{
		"start after end":  {From: &from, To: &now},
		"over ninety days": {From: &tooEarly, To: &now},
		"unknown zone":     {TimeZone: &unknown},
		"server zone":      {TimeZone: &local},
		"placeless zone":   {TimeZone: &factory},
		"rules alias":      {TimeZone: &posix},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := newService(&fakeRepo{}).Overview(t.Context(), adminPrincipal(t), request); !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("got %v; want validation", err)
			}
		})
	}
}

func TestOverviewAcceptsZonesThatNameAPlace(t *testing.T) {
	for _, zone := range []string{"UTC", "Asia/Ho_Chi_Minh", "America/Los_Angeles"} {
		repo := &fakeRepo{}
		value := zone
		if _, err := newService(repo).Overview(t.Context(), adminPrincipal(t), inbound.OverviewRequest{TimeZone: &value}); err != nil {
			t.Fatalf("zone %q: %v", zone, err)
		}
		if repo.window.TimeZone != zone {
			t.Fatalf("zone=%q; want %q", repo.window.TimeZone, zone)
		}
	}
}

func TestOverviewDropsPercentilesRestingOnTooFewRuns(t *testing.T) {
	value := int64(1200)
	repo := &fakeRepo{sample: outbound.OverviewSample{
		FinishedRuns:      sampleFloor - 1,
		AgentFinishedRuns: []int64{sampleFloor, sampleFloor - 1},
		Report: domain.OverviewReport{
			Totals: domain.OverviewTotals{Requests: 10, Succeeded: 4, AvgDurationMs: &value, P95DurationMs: &value},
			TopAgents: []domain.OverviewAgentRow{
				{Requests: 40, Succeeded: 30, P95DurationMs: &value},
				{Requests: 5, Succeeded: 5, P95DurationMs: &value},
			},
			TopTools: []domain.OverviewToolRow{
				{Calls: sampleFloor, P95DurationMs: &value},
				{Calls: sampleFloor - 1, P95DurationMs: &value},
			},
		},
	}}
	report, err := newService(repo).Overview(t.Context(), adminPrincipal(t), inbound.OverviewRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals.AvgDurationMs != nil || report.Totals.P95DurationMs != nil {
		t.Fatal("durations survived a sample below the floor")
	}
	if report.TopAgents[0].P95DurationMs == nil || report.TopAgents[1].P95DurationMs != nil {
		t.Fatal("per-agent floor was applied to the wrong row")
	}
	if report.TopTools[0].P95DurationMs == nil || report.TopTools[1].P95DurationMs != nil {
		t.Fatal("per-tool floor was applied to the wrong row")
	}
	if report.Totals.SuccessRate == nil || *report.Totals.SuccessRate != 0.4 {
		t.Fatalf("success rate=%v", report.Totals.SuccessRate)
	}
	if report.TopAgents[0].SuccessRate == nil || *report.TopAgents[0].SuccessRate != 0.75 {
		t.Fatalf("agent success rate=%v", report.TopAgents[0].SuccessRate)
	}
}

func TestOverviewLeavesSuccessRateUnsetWithoutRequests(t *testing.T) {
	repo := &fakeRepo{sample: outbound.OverviewSample{Report: domain.OverviewReport{
		TopAgents: []domain.OverviewAgentRow{{}},
	}}}
	report, err := newService(repo).Overview(t.Context(), adminPrincipal(t), inbound.OverviewRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals.SuccessRate != nil || report.TopAgents[0].SuccessRate != nil {
		t.Fatal("a rate was reported without any request to divide by")
	}
	if !report.Range.From.Equal(repo.window.From) || report.Range.TimeZone != "UTC" {
		t.Fatalf("range=%+v", report.Range)
	}
}
