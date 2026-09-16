package domain

import "errors"

// Provider errors hide upstream payloads and request credentials.
var (
	ErrProviderAuth          = errors.New("provider authentication failed")
	ErrProviderUnreachable   = errors.New("provider unreachable")
	ErrModelNotFound         = errors.New("model not found")
	ErrProviderRateLimited   = errors.New("provider rate limited")
	ErrProviderBadRequest    = errors.New("provider request invalid")
	ErrProviderNotConfigured = errors.New("provider not configured")
	ErrModelsUnsupported     = errors.New("provider model listing unsupported")
	ErrEgressDenied          = errors.New("outbound address denied")
)
