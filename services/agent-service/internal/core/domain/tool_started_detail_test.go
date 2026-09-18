package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

// A customer's browser must only ever receive a parameter an operator explicitly
// opted into tool.started; everything else, including a call the model sent with
// no visible parameter at all, must come back as an empty array.
func TestToolStartedDetailsKeepsOnlyVisibleParams(t *testing.T) {
	arguments := json.RawMessage(`{"month":"2026-06","account_number":"0123456789"}`)
	details := domain.ToolStartedDetails([]string{"month"}, arguments)
	if len(details) != 1 || details[0] != (domain.ToolStartedDetail{Name: "month", Value: "2026-06"}) {
		t.Fatalf("details=%+v", details)
	}
}

func TestToolStartedDetailsReturnsEmptyNotNilWhenNothingIsVisible(t *testing.T) {
	details := domain.ToolStartedDetails(nil, json.RawMessage(`{"month":"2026-06"}`))
	if details == nil || len(details) != 0 {
		t.Fatalf("details=%+v", details)
	}
}

func TestToolStartedDetailsSkipsAVisibleParamTheModelLeftOut(t *testing.T) {
	details := domain.ToolStartedDetails([]string{"month"}, json.RawMessage(`{}`))
	if len(details) != 0 {
		t.Fatalf("details=%+v", details)
	}
}

func TestToolStartedDetailsSkipsObjectAndArrayValues(t *testing.T) {
	arguments := json.RawMessage(`{"filters":{"branch":"HN"},"tags":["vip","new"],"month":"2026-06"}`)
	details := domain.ToolStartedDetails([]string{"filters", "tags", "month"}, arguments)
	if len(details) != 1 || details[0].Name != "month" {
		t.Fatalf("structured values leaked into details: %+v", details)
	}
}

func TestToolStartedDetailsTruncatesALongValue(t *testing.T) {
	long := strings.Repeat("a", 60)
	arguments, err := json.Marshal(map[string]string{"note": long})
	if err != nil {
		t.Fatal(err)
	}
	details := domain.ToolStartedDetails([]string{"note"}, arguments)
	if len(details) != 1 || len([]rune(details[0].Value)) != 40 || details[0].Value != strings.Repeat("a", 40) {
		t.Fatalf("truncated value=%q len=%d", details[0].Value, len([]rune(details[0].Value)))
	}
}

func TestToolStartedDetailsRendersNumbersAndBooleans(t *testing.T) {
	arguments := json.RawMessage(`{"count":3,"urgent":true}`)
	details := domain.ToolStartedDetails([]string{"count", "urgent"}, arguments)
	if len(details) != 2 || details[0].Value != "3" || details[1].Value != "true" {
		t.Fatalf("details=%+v", details)
	}
}

func TestToolStartedDetailsCapsAtThreeParams(t *testing.T) {
	arguments := json.RawMessage(`{"a":"1","b":"2","c":"3","d":"4"}`)
	details := domain.ToolStartedDetails([]string{"a", "b", "c", "d"}, arguments)
	if len(details) != 3 {
		t.Fatalf("details=%+v", details)
	}
}

func TestToolStartedDetailsIgnoresUnparsableArguments(t *testing.T) {
	details := domain.ToolStartedDetails([]string{"month"}, json.RawMessage(`not json`))
	if len(details) != 0 {
		t.Fatalf("details=%+v", details)
	}
}

// A parameter value is written by the model on every call, not typed once by an
// operator, so the rule that keeps a saved label on one line has to hold here too.
func TestToolStartedDetailsFlattensWhatCannotSitOnOneLine(t *testing.T) {
	details := domain.ToolStartedDetails([]string{"note", "blank"}, json.RawMessage(
		`{"note":"tháng 6\nDROP\u202egnaht","blank":"\u202e\n\t"}`))
	if len(details) != 1 {
		t.Fatalf("details: %+v", details)
	}
	if details[0].Value != "tháng 6 DROP gnaht" {
		t.Fatalf("value: %q", details[0].Value)
	}
}
