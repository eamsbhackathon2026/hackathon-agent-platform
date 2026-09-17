package domain

import (
	"strings"
	"testing"
	"time"
)

func saigonNoon() time.Time {
	return time.Date(2026, 9, 16, 21, 3, 51, 0, time.FixedZone("+07", 7*60*60))
}

// The failure this fixes: on 2026-09-16 an assistant asked for the August spending period
// as 202508, a year off, because nothing in the prompt said what year it was.
func TestAppendCurrentTimeStatesTheDateTheModelWouldOtherwiseGuess(t *testing.T) {
	prompt := AppendCurrentTime("Bạn là trợ lý tài chính.", saigonNoon())
	if !strings.Contains(prompt, "2026-09-16") {
		t.Fatalf("thiếu ngày hiện tại: %q", prompt)
	}
	if !strings.Contains(prompt, "Wednesday") {
		t.Fatalf("thiếu thứ trong tuần: %q", prompt)
	}
	if !strings.Contains(prompt, "Never assume a year") {
		t.Fatalf("thiếu luật cấm tự đoán năm: %q", prompt)
	}
	if !strings.Contains(prompt, "Bạn là trợ lý tài chính.") {
		t.Fatal("chỉ dẫn của người dựng bị mất")
	}
}

// A bare date is a second guess on top of the first: the day boundary a bank cares about
// is local, so the zone has to travel with the value.
func TestAppendCurrentTimeKeepsTheOffset(t *testing.T) {
	prompt := AppendCurrentTime("", saigonNoon())
	if !strings.Contains(prompt, "+07:00") {
		t.Fatalf("mất múi giờ: %q", prompt)
	}
	utc := AppendCurrentTime("", saigonNoon().UTC())
	if !strings.Contains(utc, "Z") {
		t.Fatalf("múi giờ UTC phải hiện rõ: %q", utc)
	}
}

func TestAppendCurrentTimeStandsAloneWhenPromptIsEmpty(t *testing.T) {
	for _, prompt := range []string{"", "  \n\t "} {
		if result := AppendCurrentTime(prompt, saigonNoon()); !strings.HasPrefix(result, "## Current date and time") {
			t.Fatalf("prompt rỗng không được để lại khoảng trắng thừa: %q", result)
		}
	}
}
