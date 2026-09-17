package runs

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

type asyncJobAction uint8

const (
	asyncJobDone asyncJobAction = iota
	asyncJobExecute
	asyncJobFinish
)

type asyncJobClaim struct {
	action  asyncJobAction
	run     domain.Run
	rootID  uuid.UUID
	failure *domain.RunFailure
}

// ProcessJob executes a leased asynchronous run at most once.
func (s *Service) ProcessJob(ctx context.Context, _ string, job domain.RunJob) error {
	claim, err := s.claimAsyncJobSafely(ctx, job)
	if err != nil {
		return err
	}
	switch claim.action {
	case asyncJobDone:
		return nil
	case asyncJobFinish:
		return s.finishWithoutExecution(ctx, claim.run, claim.rootID, claim.failure)
	case asyncJobExecute:
		return s.processStartedJob(ctx, claim.run, claim.rootID)
	default:
		return errExecutionPanic
	}
}

func (s *Service) claimAsyncJobSafely(ctx context.Context, job domain.RunJob) (claim asyncJobClaim, err error) {
	defer func() {
		if recover() == nil {
			return
		}
		s.logExecutionPanic(job.RunID, "async_claim")
		claim, err = s.inspectAfterAsyncClaimPanic(ctx, job.RunID)
	}()

	run, err := s.Runs.GetRun(ctx, job.RunID)
	if err != nil {
		return asyncJobClaim{}, err
	}
	if run.Terminal() {
		return asyncJobClaim{action: asyncJobDone}, nil
	}
	rootID, err := s.IDs.NewID()
	if err != nil {
		return asyncJobClaim{}, err
	}
	if run.Status == domain.RunRunning {
		return asyncJobClaim{action: asyncJobFinish, run: run, rootID: rootID, failure: &domain.RunFailure{Code: "interrupted", Message: "Máy chủ khởi động lại khi lần xử lý đang chạy."}}, nil
	}
	if run.CancelRequestedAt != nil {
		return asyncJobClaim{action: asyncJobFinish, run: run, rootID: rootID, failure: &domain.RunFailure{Code: "run_cancelled", Message: "Lần xử lý đã được dừng."}}, nil
	}
	run, err = s.Runs.StartQueuedRun(ctx, run.ID, s.Clock.Now())
	if err != nil {
		return asyncJobClaim{}, err
	}
	return asyncJobClaim{action: asyncJobExecute, run: run, rootID: rootID}, nil
}

// inspectAfterAsyncClaimPanic determines whether execution ownership was
// acquired before a dependency panicked. A running run is terminalized without
// replay; a still-queued run is returned to the queue for a safe retry.
func (s *Service) inspectAfterAsyncClaimPanic(ctx context.Context, runID uuid.UUID) (claim asyncJobClaim, err error) {
	defer func() {
		if recover() != nil {
			s.logExecutionPanic(runID, "async_claim_inspection")
			claim = asyncJobClaim{}
			err = errExecutionPanic
		}
	}()
	run, err := s.Runs.GetRun(ctx, runID)
	if err != nil {
		return asyncJobClaim{}, errExecutionPanic
	}
	if run.Terminal() {
		return asyncJobClaim{action: asyncJobDone}, nil
	}
	if run.Status != domain.RunRunning {
		return asyncJobClaim{}, errExecutionPanic
	}
	rootID, err := s.IDs.NewID()
	if err != nil {
		return asyncJobClaim{}, errExecutionPanic
	}
	return asyncJobClaim{action: asyncJobFinish, run: run, rootID: rootID, failure: internalFailure()}, nil
}

func (s *Service) processStartedJob(ctx context.Context, run domain.Run, rootID uuid.UUID) error {
	var agent domain.Agent
	var client outbound.LLMClient
	var toolset outbound.ToolSet
	var session domain.Session
	failure := s.prepareAsyncExecutionSafely(run.ID, func() *domain.RunFailure {
		var err error
		agent, err = s.Agents.GetAgent(ctx, run.AgentID)
		if err != nil {
			return internalFailure()
		}
		// The tool set is resolved before the prompt because a skill names the tools it
		// needs and those references are rewritten into this run's tool names. Resolving
		// only builds a snapshot and connects nothing, so moving it earlier costs nothing.
		toolset, err = s.Tools.Resolve(ctx, agent.ID)
		if err != nil {
			return safeRunFailure(err)
		}
		agent.SystemPrompt, err = s.Skills.ResolveSystemPrompt(ctx, agent.ID, agent.SystemPrompt, toolset.Specs())
		if err != nil {
			return safeRunFailure(err)
		}
		provider, err := s.Providers.GetProvider(ctx, agent.ProviderID)
		if err != nil {
			return internalFailure()
		}
		client, err = s.client(ctx, provider)
		if err != nil {
			return safeRunFailure(err)
		}
		session, err = s.Sessions.GetSession(ctx, run.SessionID)
		if err != nil {
			return internalFailure()
		}
		return nil
	})
	if toolset != nil {
		defer s.closeToolSetSafely(run.ID, toolset)
	}
	if failure != nil {
		return s.finishWithoutExecution(ctx, run, rootID, failure)
	}
	_, err := s.executeStarted(ctx, agent, client, toolset, startedRun{run: run, session: session, rootSpanID: rootID}, inbound.RunEventSinkFunc(func(context.Context, domain.RunEvent) error { return nil }))
	return err
}

func (s *Service) finishWithoutExecution(ctx context.Context, run domain.Run, rootID uuid.UUID, failure *domain.RunFailure) error {
	state := executionState{run: run, rootSpanID: rootID, usage: newUsageAccumulator(), loop: mustLoopDetector()}
	_, err := s.finish(ctx, &state, failure, func(domain.RunEvent) {})
	return err
}

var _ inbound.AsyncRunProcessor = (*Service)(nil)
