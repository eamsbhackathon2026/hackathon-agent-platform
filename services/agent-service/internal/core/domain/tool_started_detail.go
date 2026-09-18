package domain

import (
	"encoding/json"
	"strings"
	"unicode"
)

// ToolStartedDetail is one parameter value surfaced in a tool.started event. Name
// is a parameter an operator marked show_in_progress on the tool, and Value is
// already rendered to something short — never raw JSON. A value never reaches
// here unless the resolver put its parameter's name on ToolSpec.VisibleParams, so
// a parameter an operator left off stays inside the platform instead of leaving
// and relying on a caller to filter it back out.
type ToolStartedDetail struct {
	Name, Value string
}

// Limits applied when rendering a tool call's arguments into ToolStartedDetail: a
// step label reads as one short line, not a data dump.
const (
	toolStartedDetailMaxValueRunes = 40
	toolStartedDetailMaxCount      = 3
)

// ToolStartedDetails extracts the values of visibleParams from a tool call's
// arguments, in declaration order, so a step label can carry a concrete value
// ("...tháng 6/2026") instead of staying generic. A parameter the model left out
// of the call is skipped, and so is one whose value is an object or array,
// rather than forcing a placeholder or a raw JSON dump. Only the first
// toolStartedDetailMaxCount visible parameters are kept. The result is never
// nil: a tool.started event always carries a details array, empty or not.
func ToolStartedDetails(visibleParams []string, arguments json.RawMessage) []ToolStartedDetail {
	details := make([]ToolStartedDetail, 0, min(len(visibleParams), toolStartedDetailMaxCount))
	if len(visibleParams) == 0 {
		return details
	}
	args, err := ToolArguments(arguments)
	if err != nil {
		return details
	}
	for _, name := range visibleParams {
		if len(details) >= toolStartedDetailMaxCount {
			break
		}
		value, ok := args[name]
		if !ok {
			continue
		}
		rendered, ok := renderToolStartedValue(value)
		if !ok {
			continue
		}
		details = append(details, ToolStartedDetail{Name: name, Value: rendered})
	}
	return details
}

// renderToolStartedValue turns one decoded JSON argument into short display text.
// Object and array values are skipped rather than serialized: a raw JSON dump is
// not something an end user reads as one line of running text.
func renderToolStartedValue(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		// The one-line rule that guards a saved step label has to guard this too,
		// and here it matters more: an operator writes a label once, while this
		// value is written by the model on every call. A newline breaks the line it
		// lands in and a bidi override renders it as something it does not say.
		flat := flattenToOneLine(v)
		if flat == "" {
			return "", false
		}
		return truncateRunes(flat, toolStartedDetailMaxValueRunes), true
	case json.Number:
		return truncateRunes(v.String(), toolStartedDetailMaxValueRunes), true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	default:
		return "", false
	}
}

// truncateRunes cuts value to at most limit runes, counting runes rather than
// bytes so a multi-byte character is never split.
func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

// flattenToOneLine replaces every rune that cannot sit in one line of running
// text with a space, then collapses the runs of spaces it leaves behind. The
// value is dropped rather than shown when nothing readable survives.
func flattenToOneLine(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	space := true // leading spaces collapse away
	for _, r := range value {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) || r == ' ' {
			if !space {
				builder.WriteByte(' ')
				space = true
			}
			continue
		}
		builder.WriteRune(r)
		space = false
	}
	return strings.TrimRight(builder.String(), " ")
}
