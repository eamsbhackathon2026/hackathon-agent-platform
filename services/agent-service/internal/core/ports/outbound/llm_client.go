package outbound

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"context"
)

// LLMClient streams a response, or collects it when onDelta is nil.
// Callers must discard results on error, even if text deltas were already emitted.
type LLMClient interface {
	Stream(context.Context, domain.LLMRequest, func(domain.LLMDelta)) (domain.LLMResult, error)
	ListModels(context.Context) ([]string, error)
}

// LLMClientFactory creates a request-scoped client from decrypted credentials.
type LLMClientFactory interface {
	New(context.Context, domain.ProviderConnection) (LLMClient, error)
}

// URLPolicy checks URL syntax and configured egress rules without making requests.
// The HTTP transport must independently check resolved IPs at connection time.
type URLPolicy interface{ ValidateURL(string) error }
