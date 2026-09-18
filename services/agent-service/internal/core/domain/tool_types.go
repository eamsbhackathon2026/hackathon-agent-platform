package domain

import "strings"

// ToolStepLabel returns the first candidate that carries text, trimmed, for the
// label an end user reads while one tool call runs. Callers pass candidates in
// falling order of fitness: a saved step label is written for that moment
// ("Đang đọc chi tiêu theo tháng"), a catalog display name names the tool for an
// operator and still reads acceptably, and the provider-facing name is the last
// resort that only surfaces when a saved tool carries no label at all.
func ToolStepLabel(candidates ...string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
