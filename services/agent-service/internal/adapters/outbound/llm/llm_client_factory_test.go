package llm

import (
	"context"
	"errors"
	"testing"

	"agent-platform/services/agent-service/internal/adapters/outbound/llm/greennode"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/platform/netguard"
)

func TestFactorySelectsGreenNodeWithManagedEndpoint(t *testing.T) {
	guard, err := netguard.New(netguard.Config{})
	if err != nil {
		t.Fatal(err)
	}
	factory, err := NewFactory(guard)
	if err != nil {
		t.Fatal(err)
	}
	client, err := factory.New(context.Background(), domain.ProviderConnection{Kind: domain.ProviderGreenNode, BaseURL: "https://untrusted.example/v1", APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := client.(*greennode.Client); !ok {
		t.Fatalf("client = %T, want GreenNode adapter", client)
	}
}

func TestFactoryRejectsInvalidConfiguration(t *testing.T) {
	if _, err := NewFactory(nil); err == nil {
		t.Fatal("nil guard accepted")
	}
	guard, _ := netguard.New(netguard.Config{})
	factory, _ := NewFactory(guard)
	if _, err := factory.New(context.Background(), domain.ProviderConnection{Kind: domain.ProviderGreenNode}); !errors.Is(err, domain.ErrProviderNotConfigured) {
		t.Fatalf("missing key: %v", err)
	}
	if _, err := factory.New(context.Background(), domain.ProviderConnection{Kind: "other", APIKey: "key"}); !errors.Is(err, domain.ErrProviderBadRequest) {
		t.Fatalf("unknown kind: %v", err)
	}
}
