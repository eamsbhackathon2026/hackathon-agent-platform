//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	runservice "agent-platform/services/agent-service/internal/core/services/runs"
	"agent-platform/services/agent-service/internal/platform/clock"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
	"agent-platform/services/agent-service/internal/testsupport/pgtest"
)

func TestAsyncIdempotencyAndQueueLeases(t *testing.T) {
	ctx := t.Context()
	pool := pgtest.NewPool(t)
	store := postgres.NewStore(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID, providerID, agentID := uuid.New(), uuid.New(), uuid.New()
	if err := store.CreateUser(ctx, domain.User{ID: userID, Email: "async@example.test", Name: "Async", PasswordHash: "hash", Role: domain.RoleOwner, Status: domain.UserActive, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	cipher := fakes.Cipher{}
	providerSecret, _ := cipher.Encrypt([]byte("provider-secret"), []byte("provider:"+providerID.String()))
	if err := store.CreateProvider(ctx, domain.Provider{ID: providerID, Name: "Async provider", Kind: domain.ProviderGemini, APIKeyCiphertext: providerSecret, Status: domain.ConnectionOK, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(ctx, domain.Agent{ID: agentID, Name: "Async agent", ProviderID: providerID, Model: "model", MaxIterations: 4, TimeoutSeconds: 30, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	keyOne := createAsyncAPIKey(t, store, userID, now, "one")
	keyTwo := createAsyncAPIKey(t, store, userID, now, "two")
	llm := &fakes.ScriptedLLM{}
	service, err := runservice.NewService(runservice.Dependencies{Agents: store, Providers: store, Sessions: store, Messages: store, Snapshots: store, Runs: store, Spans: store, Tx: store, Cipher: cipher, Factory: &fakes.FakeFactory{Client: llm}, Tools: fakes.ToolResolver{Set: &fakes.ToolSet{}}, Skills: &fakes.SkillResolver{}, Queue: store, Idempotency: store, Deliveries: store, Clock: clock.RealClock{}, IDs: clock.UUIDv7IDGenerator{}})
	if err != nil {
		t.Fatal(err)
	}
	counts := func() [4]int {
		t.Helper()
		var values [4]int
		if scanErr := pool.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM sessions),
			(SELECT count(*) FROM runs),
			(SELECT count(*) FROM messages),
			(SELECT count(*) FROM run_jobs)`).Scan(&values[0], &values[1], &values[2], &values[3]); scanErr != nil {
			t.Fatal(scanErr)
		}
		return values
	}
	beforeRollback := counts()
	failingService, err := runservice.NewService(runservice.Dependencies{Agents: store, Providers: store, Sessions: store, Messages: store, Snapshots: store, Runs: store, Spans: store, Tx: store, Cipher: cipher, Factory: &fakes.FakeFactory{Client: llm}, Tools: fakes.ToolResolver{Set: &fakes.ToolSet{}}, Skills: &fakes.SkillResolver{}, Queue: failingEnqueueQueue{RunQueue: store}, Idempotency: store, Deliveries: store, Clock: clock.RealClock{}, IDs: clock.UUIDv7IDGenerator{}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = failingService.EnqueueAsync(ctx, domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: keyOne, Scopes: []string{"runs:write"}}, inbound.RunCommand{AgentID: agentID, Input: "rollback", Mode: domain.RunModeAsync, Metadata: json.RawMessage(`{}`)})
	if !errors.Is(err, errEnqueueProbe) {
		t.Fatalf("enqueue error=%v", err)
	}
	if afterRollback := counts(); afterRollback != beforeRollback {
		t.Fatalf("enqueue rollback để lại dữ liệu: before=%v after=%v", beforeRollback, afterRollback)
	}
	hash, _ := domain.IdempotencyRequestHash("POST", "/v1/agents/"+agentID.String()+"/runs", []byte(`{"input":{"message":"hello"},"mode":"async"}`))
	idempotencyKey := "same-request"
	command := inbound.RunCommand{AgentID: agentID, Input: "hello", Mode: domain.RunModeAsync, Metadata: json.RawMessage(`{}`), IdempotencyKey: &idempotencyKey, RequestHash: hash}

	const callers = 20
	ids := make(chan uuid.UUID, callers)
	errorsFound := make(chan error, callers)
	var group sync.WaitGroup
	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			run, enqueueErr := service.EnqueueAsync(ctx, domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: keyOne, Scopes: []string{"runs:write"}}, command)
			if enqueueErr != nil {
				errorsFound <- enqueueErr
				return
			}
			ids <- run.ID
		}()
	}
	group.Wait()
	close(ids)
	close(errorsFound)
	for enqueueErr := range errorsFound {
		t.Fatal(enqueueErr)
	}
	var only uuid.UUID
	for id := range ids {
		if only == uuid.Nil {
			only = id
		} else if id != only {
			t.Fatalf("idempotency tạo nhiều run: %s và %s", only, id)
		}
	}

	second, err := service.EnqueueAsync(ctx, domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: keyTwo, Scopes: []string{"runs:write"}}, command)
	if err != nil || second.ID == only {
		t.Fatalf("principal khác không có scope idempotency riêng: run=%s err=%v", second.ID, err)
	}

	claimed := make(chan []domain.RunJob, 2)
	for _, worker := range []string{"worker-a", "worker-b"} {
		group.Add(1)
		go func(id string) {
			defer group.Done()
			jobs, _ := store.Claim(ctx, id, 10, 20*time.Millisecond)
			claimed <- jobs
		}(worker)
	}
	group.Wait()
	close(claimed)
	seen := map[uuid.UUID]bool{}
	for jobs := range claimed {
		for _, job := range jobs {
			if seen[job.RunID] {
				t.Fatalf("job %s bị claim trùng", job.RunID)
			}
			seen[job.RunID] = true
		}
	}
	if len(seen) != 2 {
		t.Fatalf("claimed %d jobs, want 2", len(seen))
	}
	time.Sleep(30 * time.Millisecond)
	reclaimed, err := store.Claim(ctx, "worker-c", 10, time.Second)
	if err != nil || len(reclaimed) != 2 || reclaimed[0].Attempts != 2 {
		t.Fatalf("reclaimed=%+v err=%v", reclaimed, err)
	}
	running, err := store.StartQueuedRun(ctx, reclaimed[0].RunID, time.Now())
	if err != nil || running.Status != domain.RunRunning {
		t.Fatalf("start=%+v err=%v", running, err)
	}
	if err = service.ProcessJob(ctx, "worker-c", reclaimed[0]); err != nil {
		t.Fatal(err)
	}
	interrupted, err := store.GetRun(ctx, reclaimed[0].RunID)
	if err != nil || interrupted.Status != domain.RunFailed || interrupted.Failure == nil || interrupted.Failure.Code != "interrupted" {
		t.Fatalf("interrupted=%+v err=%v", interrupted, err)
	}
	if len(llm.Requests()) != 0 {
		t.Fatal("run đang running bị gọi lại LLM")
	}
	cancelledCommand := command
	cancelledCommand.IdempotencyKey = nil
	cancelledCommand.RequestHash = nil
	cancelled, err := service.EnqueueAsync(ctx, domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: keyOne, Scopes: []string{"runs:write", "runs:read"}}, cancelledCommand)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CancelRun(ctx, domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: keyOne, Scopes: []string{"runs:read"}}, cancelled.ID); err != nil {
		t.Fatal(err)
	}
	if err = service.ProcessJob(ctx, "worker-c", domain.RunJob{RunID: cancelled.ID}); err != nil {
		t.Fatal(err)
	}
	cancelled, err = store.GetRun(ctx, cancelled.ID)
	if err != nil || cancelled.Status != domain.RunCancelled {
		t.Fatalf("cancelled=%+v err=%v", cancelled, err)
	}

	latencies := make(chan time.Duration, 50)
	latencyErrors := make(chan error, 50)
	for range 50 {
		group.Add(1)
		go func() {
			defer group.Done()
			started := time.Now()
			_, enqueueErr := service.EnqueueAsync(ctx, domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: keyOne, Scopes: []string{"runs:write"}}, inbound.RunCommand{AgentID: agentID, Input: "latency", Mode: domain.RunModeAsync, Metadata: json.RawMessage(`{}`)})
			if enqueueErr != nil {
				latencyErrors <- enqueueErr
				return
			}
			latencies <- time.Since(started)
		}()
	}
	group.Wait()
	close(latencies)
	close(latencyErrors)
	for enqueueErr := range latencyErrors {
		t.Fatal(enqueueErr)
	}
	measured := make([]time.Duration, 0, 50)
	for latency := range latencies {
		measured = append(measured, latency)
	}
	sort.Slice(measured, func(i, j int) bool { return measured[i] < measured[j] })
	if len(measured) != 50 || measured[47] >= 100*time.Millisecond {
		t.Fatalf("p95 enqueue=%s samples=%d", measured[47], len(measured))
	}
	t.Logf("p95 enqueue 50 concurrent requests: %s", measured[47])
}

var errEnqueueProbe = errors.New("enqueue probe failure")

type failingEnqueueQueue struct{ outbound.RunQueue }

func (failingEnqueueQueue) Enqueue(context.Context, domain.RunJob) error { return errEnqueueProbe }

func createAsyncAPIKey(t *testing.T, store *postgres.Store, userID uuid.UUID, now time.Time, suffix string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := store.CreateAPIKey(t.Context(), domain.APIKey{ID: id, Name: "key " + suffix, Prefix: "prefix" + suffix, KeyHash: []byte("hash"), Scopes: []string{"runs:write", "runs:read"}, WebhookSecretCiphertext: []byte("secret"), CreatedBy: userID, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	return id
}
