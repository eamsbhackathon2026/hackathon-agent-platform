package fakes

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"context"
	"github.com/google/uuid"
	"sync"
)

// CatalogStore is an isolated, serialized catalog store with rollback support.
type CatalogStore struct {
	mu          sync.Mutex
	providers   map[uuid.UUID]domain.Provider
	agents      map[uuid.UUID]domain.Agent
	connections map[uuid.UUID]domain.APIConnection
	tools       map[uuid.UUID]domain.HTTPTool
	servers     map[uuid.UUID]domain.MCPServer
	bindings    map[uuid.UUID]domain.AgentToolBindings
}
type catalogTxKey struct{}
type catalogTx struct {
	store  *CatalogStore
	active bool
}

// NewCatalogStore creates an empty catalog repository and transaction manager.
func NewCatalogStore() *CatalogStore {
	return &CatalogStore{providers: map[uuid.UUID]domain.Provider{}, agents: map[uuid.UUID]domain.Agent{}, connections: map[uuid.UUID]domain.APIConnection{}, tools: map[uuid.UUID]domain.HTTPTool{}, servers: map[uuid.UUID]domain.MCPServer{}, bindings: map[uuid.UUID]domain.AgentToolBindings{}}
}
func (s *CatalogStore) lock(ctx context.Context) func() {
	if tx, ok := ctx.Value(catalogTxKey{}).(*catalogTx); ok && tx.store == s && tx.active {
		return func() {}
	}
	s.mu.Lock()
	return s.mu.Unlock
}

// WithinTx rolls back callback errors and panics, including mutable entity fields.
func (s *CatalogStore) WithinTx(ctx context.Context, fn func(context.Context) error) (err error) {
	defer s.lock(ctx)()
	snapshot := NewCatalogStore()
	for k, v := range s.providers {
		snapshot.providers[k] = cloneProvider(v)
	}
	for k, v := range s.agents {
		snapshot.agents[k] = cloneAgent(v)
	}
	for k, v := range s.connections {
		snapshot.connections[k] = cloneAPIConnection(v)
	}
	for k, v := range s.tools {
		snapshot.tools[k] = cloneHTTPTool(v)
	}
	for k, v := range s.servers {
		snapshot.servers[k] = cloneMCPServer(v)
	}
	for k, v := range s.bindings {
		snapshot.bindings[k] = cloneBindings(v)
	}
	tx := &catalogTx{store: s, active: true}
	defer func() {
		tx.active = false
		if p := recover(); p != nil {
			s.providers = snapshot.providers
			s.agents = snapshot.agents
			s.connections = snapshot.connections
			s.tools = snapshot.tools
			s.servers = snapshot.servers
			s.bindings = snapshot.bindings
			panic(p)
		}
		if err != nil {
			s.providers = snapshot.providers
			s.agents = snapshot.agents
			s.connections = snapshot.connections
			s.tools = snapshot.tools
			s.servers = snapshot.servers
			s.bindings = snapshot.bindings
		}
	}()
	return fn(context.WithValue(ctx, catalogTxKey{}, tx))
}

// LockCatalog uses the enclosing transaction's serialization.
func (s *CatalogStore) LockCatalog(ctx context.Context) error { defer s.lock(ctx)(); return nil }

func cloneProvider(p domain.Provider) domain.Provider {
	p.BaseURL = copyPointer(p.BaseURL)
	p.DefaultModel = copyPointer(p.DefaultModel)
	p.APIKeyHint = copyPointer(p.APIKeyHint)
	p.LastError = copyPointer(p.LastError)
	p.LastCheckedAt = copyPointer(p.LastCheckedAt)
	p.APIKeyCiphertext = append([]byte(nil), p.APIKeyCiphertext...)
	return p
}
func cloneAgent(a domain.Agent) domain.Agent {
	a.Temperature = copyPointer(a.Temperature)
	a.MaxOutputTokens = copyPointer(a.MaxOutputTokens)
	a.ArchivedAt = copyPointer(a.ArchivedAt)
	return a
}

var _ outbound.ProviderRepository = (*CatalogStore)(nil)
var _ outbound.AgentRepository = (*CatalogStore)(nil)
var _ outbound.TxManager = (*CatalogStore)(nil)
