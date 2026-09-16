package domain

// ToolDisplayName returns the product-facing name for a provider-neutral tool.
// Phase 5 adapters may replace this fallback with saved configuration labels.
func ToolDisplayName(name string) string { return name }
