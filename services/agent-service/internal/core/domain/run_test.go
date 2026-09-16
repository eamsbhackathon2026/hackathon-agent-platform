package domain_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestRunTransitionRejectsInvalidStateChanges(t *testing.T) {
	run := domain.Run{Status: domain.RunQueued}
	if err := run.Transition(domain.RunRunning); err != nil {
		t.Fatal(err)
	}
	if err := run.Transition(domain.RunSucceeded); err != nil || !run.Terminal() {
		t.Fatal("running run did not reach succeeded")
	}
	if err := run.Transition(domain.RunFailed); err == nil {
		t.Fatal("terminal run accepted another transition")
	}
}

func TestLoopDetectorCanonicalizesArgumentsAndStopsAtThirdMatch(t *testing.T) {
	detector, err := domain.NewLoopDetector(3)
	if err != nil {
		t.Fatal(err)
	}
	arguments := []string{`{"b":2,"a":1}`, `{"a":1,"b":2}`, `{"b":2,"a":1}`}
	for i, raw := range arguments {
		loop, observeErr := detector.Observe(domain.ToolCall{Name: "lookup", Arguments: json.RawMessage(raw)}, domain.ToolResult{Content: "same"})
		if observeErr != nil || loop != (i == 2) {
			t.Fatalf("observation %d: loop=%v err=%v", i+1, loop, observeErr)
		}
	}
	if _, err = detector.Observe(domain.ToolCall{Name: "lookup", Arguments: json.RawMessage(`{} {}`)}, domain.ToolResult{}); err == nil {
		t.Fatal("invalid trailing JSON was accepted")
	}
}

func TestLoopDetectorWarnsBeforeExactLoopAndDetectsSameResultAcrossArguments(t *testing.T) {
	detector, err := domain.NewLoopDetector(3)
	if err != nil {
		t.Fatal(err)
	}
	call := domain.ToolCall{Name: "lookup", Arguments: json.RawMessage(`{"q":1}`)}
	if observation, observeErr := detector.ObserveProgress(call, domain.ToolResult{Content: "same"}); observeErr != nil || observation.Warning || observation.Critical {
		t.Fatalf("first observation=%+v err=%v", observation, observeErr)
	}
	if observation, observeErr := detector.ObserveProgress(call, domain.ToolResult{Content: "same"}); observeErr != nil || !observation.Warning || observation.Critical || observation.RepetitionCount != 2 {
		t.Fatalf("second observation=%+v err=%v", observation, observeErr)
	}

	detector, _ = domain.NewLoopDetector(3)
	var observation domain.LoopObservation
	for i := 1; i <= 6; i++ {
		call.Arguments = json.RawMessage(fmt.Sprintf(`{"q":%d}`, i))
		observation, err = detector.ObserveProgress(call, domain.ToolResult{Content: "unchanged"})
		if err != nil {
			t.Fatal(err)
		}
		if i == 4 && (!observation.Warning || observation.Critical) {
			t.Fatalf("fourth observation=%+v", observation)
		}
	}
	if !observation.Critical || observation.SameResultCount != 6 {
		t.Fatalf("final observation=%+v", observation)
	}
}

func TestTruncateUTF8DoesNotSplitCodePoint(t *testing.T) {
	value, truncated := domain.TruncateUTF8("abc🙂def", 6)
	if !truncated || value != "abc" {
		t.Fatalf("value=%q truncated=%v", value, truncated)
	}
	value, truncated = domain.TruncateUTF8("đủ", 20)
	if truncated || value != "đủ" {
		t.Fatal("short UTF-8 string changed")
	}
}
