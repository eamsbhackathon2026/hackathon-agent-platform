package domain

import "time"

// A model has no clock. Asked about "last month" it resolves the year from whatever its
// training left it with, which is a guess that looks like a fact: a real run on
// 2026-09-16 requested the August spending period as 202508 and got an empty result,
// then answered from a different tool without ever noticing the year was wrong. Every
// assistant that reads data by period hits this, so the run states the time once.
//
// The offset is part of the value. A banking day boundary is local, and a bare date with
// no zone is a second guess on top of the first.

// AppendCurrentTime states the run's wall clock and the rule for reading relative dates
// against it.
func AppendCurrentTime(prompt string, now time.Time) string {
	section := "## Current date and time\n\n" + now.Format(time.RFC3339) + " (" + now.Weekday().String() + ")\n\n" +
		"Resolve every relative date the user mentions — today, this month, last month, a bare month name — " +
		"against this value. Never assume a year, and never take a date from your own memory. " +
		"Tools that accept a period or a date range expect values derived from it."
	return appendPromptSection(prompt, section)
}
