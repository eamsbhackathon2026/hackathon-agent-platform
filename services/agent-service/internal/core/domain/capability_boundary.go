package domain

import "strings"

const (
	// maxCapabilityDescriptionBytes keeps each line short. The full descriptions already
	// travel with the tool schemas in the same request, so this listing only has to make
	// the inventory legible as prose, not repeat it. Measured against the real anti-fraud
	// tools, the whole description averaged 215 bytes per tool while its first sentence —
	// the part that says what the tool does, before parameter and return detail — averaged
	// under 70, so the listing takes the first sentence and stops.
	maxCapabilityDescriptionBytes = 100
	// MaxCapabilitySectionBytes bounds the generated section. Nothing limits how many
	// tools an agent may be bound to, so without a cap one misconfigured agent could
	// crowd out the conversation. Past the cap the listing drops to names alone, which
	// still answers the only question that matters: is this action available at all.
	MaxCapabilitySectionBytes = 4 * 1024
)

// An assistant is configured in two places that nothing reconciles: the tools an operator
// binds, and the system prompt they write. When the prompt describes work the tools cannot
// do, the assistant adopts the prompt's framing and promises actions it has no way to
// carry out — an anti-fraud assistant with only assessment tools told a customer it would
// transfer money for them. Stating the inventory and the rule together closes that gap
// from the side the platform actually controls.
const capabilityRule = "Never promise, offer, or imply any action outside this list, and never state that " +
	"something has been done unless a tool did it. When the user asks for something you cannot do, say plainly " +
	"that you cannot do it, then say what you can do instead. Describe what you can do in everyday language: " +
	"never mention tool names or any internal identifier to the user."

const capabilityRuleWithoutTools = "You have no tools: you can only answer from this conversation, and you cannot " +
	"look anything up, change anything, or carry out any action. Never promise, offer, or imply otherwise, and " +
	"never state that something has been done. When the user asks for something that would require an action, say " +
	"plainly that you cannot do it and explain what they can do themselves."

// AppendCapabilityBoundary states what the assistant can actually do, derived from the
// tools resolved for this run so the statement can never drift from the bindings. It is
// appended after the operator's prompt and the skill instructions, matching how the
// skill section is layered, so the boundary reads as the last word on the subject.
func AppendCapabilityBoundary(prompt string, specs []ToolSpec) string {
	section := "## Capabilities\n\n"
	if len(specs) == 0 {
		section += capabilityRuleWithoutTools
	} else {
		section += "These are the only actions you can perform:\n\n" + capabilityList(specs) + "\n" + capabilityRule
	}
	return appendPromptSection(prompt, section)
}

// appendPromptSection joins a generated section onto whatever the operator wrote, and
// stands alone when they wrote nothing rather than leaving leading blank lines.
func appendPromptSection(prompt, section string) string {
	if strings.TrimSpace(prompt) == "" {
		return section
	}
	return prompt + "\n\n" + section
}

func capabilityList(specs []ToolSpec) string {
	withDescriptions := buildCapabilityList(specs, true)
	if len(withDescriptions) <= MaxCapabilitySectionBytes {
		return withDescriptions
	}
	return buildCapabilityList(specs, false)
}

func buildCapabilityList(specs []ToolSpec, describe bool) string {
	var list strings.Builder
	for _, spec := range specs {
		list.WriteString("- ")
		list.WriteString(spec.Name)
		if description := capabilitySummary(spec.Description); describe && description != "" {
			list.WriteString(": ")
			list.WriteString(description)
		}
		list.WriteString("\n")
	}
	return list.String()
}

// capabilitySummary reduces a tool description to the clause that says what it does.
// Descriptions come from external manifests and may span several lines, which would break
// the one-tool-per-line shape the listing relies on, so whitespace collapses first.
func capabilitySummary(description string) string {
	// Fields also strips the newlines that would break the one-tool-per-line shape.
	collapsed := strings.Join(strings.Fields(description), " ")
	if collapsed == "" {
		return ""
	}
	if sentence, _, found := strings.Cut(collapsed, ". "); found {
		collapsed = sentence + "."
	}
	trimmed, cut := TruncateUTF8(collapsed, maxCapabilityDescriptionBytes)
	if cut {
		return trimmed + "…"
	}
	return trimmed
}
