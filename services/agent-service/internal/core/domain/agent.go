package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultContextWindowTokens is conservative for custom model identifiers.
	DefaultContextWindowTokens = 32768
	// DefaultMaxOutputTokens caps provider output when an assistant does not set one.
	DefaultMaxOutputTokens = 4096
	// MinContextWindowTokens leaves room for a useful prompt and response.
	MinContextWindowTokens = 8192
	// MaxContextWindowTokens bounds persisted configuration and estimator arithmetic.
	MaxContextWindowTokens = 2000000
	// MinimumContextInputTokens prevents configurations with no usable prompt budget.
	MinimumContextInputTokens = 1024
	// MinimumContextSafetyTokens covers estimator and provider framing differences.
	MinimumContextSafetyTokens = 512
)

// ContextSafetyTokens returns the fixed-or-proportional request safety margin.
func ContextSafetyTokens(window int) int {
	safety := window / 10
	if safety < MinimumContextSafetyTokens {
		return MinimumContextSafetyTokens
	}
	return safety
}

// MaxOutputTokensForContext leaves both estimator safety and a useful input budget.
func MaxOutputTokensForContext(window int) int {
	return window - ContextSafetyTokens(window) - MinimumContextInputTokens
}

// EffectiveMaxOutputTokens resolves the provider cap used when the field is omitted.
func EffectiveMaxOutputTokens(agent Agent) int {
	if agent.MaxOutputTokens != nil {
		return *agent.MaxOutputTokens
	}
	window := agent.ContextWindowTokens
	if window == 0 {
		window = DefaultContextWindowTokens
	}
	reserve := DefaultMaxOutputTokens
	if quarter := window / 4; quarter < reserve {
		reserve = quarter
	}
	return reserve
}

// Agent is a saved assistant configuration; archived records retain history.
type Agent struct {
	ID                  uuid.UUID
	Name, Description   string
	ProviderID          uuid.UUID
	Model, SystemPrompt string
	Temperature         *float64
	MaxOutputTokens     *int
	// ShowThinking asks the provider for a summary of its own thinking so callers
	// can narrate progress. It never changes the answer.
	ShowThinking                  bool
	ContextWindowTokens           int
	MaxIterations, TimeoutSeconds int
	CreatedBy                     uuid.UUID
	ArchivedAt                    *time.Time
	CreatedAt, UpdatedAt          time.Time
}

// AgentView includes configuration readiness without making an upstream request.
type AgentView struct {
	Agent          Agent
	Ready          bool
	ReadinessError *ProviderFailure
}
