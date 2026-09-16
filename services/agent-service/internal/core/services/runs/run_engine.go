package runs

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

var (
	errCancelRequested = errors.New("run cancellation requested")
	errEventSink       = errors.New("run event sink failed")
	errExecutionPanic  = errors.New("agent execution panicked")
)

// RunSync executes without emitting incremental events.
func (s *Service) RunSync(ctx context.Context, p domain.Principal, c inbound.RunCommand) (domain.Run, error) {
	c.Mode = domain.RunModeSync
	return s.execute(ctx, p, c, inbound.RunEventSinkFunc(func(context.Context, domain.RunEvent) error { return nil }))
}

// EnqueueAsync persists the run, user message and job atomically.
func (s *Service) EnqueueAsync(ctx context.Context, p domain.Principal, c inbound.RunCommand) (domain.Run, error) {
	c.Mode = domain.RunModeAsync
	agent, provider, err := s.validateCommand(ctx, p, c)
	if err != nil {
		return domain.Run{}, err
	}
	if _, err = s.client(ctx, provider); err != nil {
		return domain.Run{}, err
	}
	toolset, err := s.Tools.Resolve(ctx, agent.ID)
	if err != nil {
		return domain.Run{}, err
	}
	if _, err = s.Skills.ResolveSystemPrompt(ctx, agent.ID, agent.SystemPrompt); err != nil {
		_ = toolset.Close()
		return domain.Run{}, err
	}
	if err = toolset.Close(); err != nil {
		return domain.Run{}, err
	}
	started, err := s.start(ctx, p, c)
	return started.run, err
}

// RunStream executes while emitting ordered incremental events.
func (s *Service) RunStream(ctx context.Context, p domain.Principal, c inbound.RunCommand, sink inbound.RunEventSink) (domain.Run, error) {
	c.Mode = domain.RunModeStream
	if sink == nil {
		return domain.Run{}, domain.ErrValidation
	}
	return s.execute(ctx, p, c, sink)
}

func (s *Service) execute(ctx context.Context, p domain.Principal, c inbound.RunCommand, sink inbound.RunEventSink) (domain.Run, error) {
	agent, provider, err := s.validateCommand(ctx, p, c)
	if err != nil {
		return domain.Run{}, err
	}
	client, err := s.client(ctx, provider)
	if err != nil {
		return domain.Run{}, err
	}
	toolset, err := s.Tools.Resolve(ctx, agent.ID)
	if err != nil {
		return domain.Run{}, err
	}
	runID := uuid.Nil
	defer func() { s.closeToolSetSafely(runID, toolset) }()
	agent.SystemPrompt, err = s.Skills.ResolveSystemPrompt(ctx, agent.ID, agent.SystemPrompt)
	if err != nil {
		return domain.Run{}, err
	}
	started, err := s.start(ctx, p, c)
	if err != nil {
		return domain.Run{}, err
	}
	runID = started.run.ID
	if started.replayed {
		if !started.run.Terminal() {
			return started.run, &domain.Error{Kind: domain.ErrRunInProgress, RunID: &started.run.ID}
		}
		return started.run, nil
	}
	return s.executeStarted(ctx, agent, client, toolset, started, sink)
}

func (s *Service) executeStarted(ctx context.Context, agent domain.Agent, client outbound.LLMClient, toolset outbound.ToolSet, started startedRun, sink inbound.RunEventSink) (domain.Run, error) {
	if agent.MaxOutputTokens == nil {
		maxOutputTokens := domain.EffectiveMaxOutputTokens(agent)
		agent.MaxOutputTokens = &maxOutputTokens
	}
	timeoutCtx, stopTimeout := context.WithTimeout(ctx, time.Duration(agent.TimeoutSeconds)*time.Second)
	defer stopTimeout()
	execCtx, cancel := context.WithCancelCause(timeoutCtx)
	defer cancel(nil)
	watcherCtx, stopWatcher := context.WithCancel(execCtx)
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		s.watchCancellation(watcherCtx, started.run.ID, cancel)
	}()
	var stopWatcherOnce sync.Once
	stopWatcherAndWait := func() {
		stopWatcherOnce.Do(func() {
			stopWatcher()
			<-watcherDone
		})
	}
	defer stopWatcherAndWait()
	emit := func(event domain.RunEvent) {
		defer func() {
			if recover() != nil {
				s.logExecutionPanic(started.run.ID, "event_sink")
				cancel(errExecutionPanic)
			}
		}()
		eventCtx := execCtx
		if event.Type == domain.EventRunCompleted || event.Type == domain.EventRunFailed {
			eventCtx = ctx
		}
		if emitErr := sink.Emit(eventCtx, event); emitErr != nil {
			cancel(errors.Join(errEventSink, emitErr))
		}
	}
	state := executionState{run: started.run, session: started.session, rootSpanID: started.rootSpanID, usage: newUsageAccumulator(), loop: mustLoopDetector(), budgeter: newContextBudgeter(started.session)}
	failure := s.executeWithSafetyNet(started.run.ID, cancel, func() *domain.RunFailure {
		emit(domain.RunEvent{Type: domain.EventRunStarted, RunID: started.run.ID, SessionID: started.run.SessionID})
		if execCtx.Err() != nil {
			return failureFromCause(context.Cause(execCtx))
		}
		if loadFailure := s.loadContext(execCtx, agent, client, toolset.Specs(), &state); loadFailure != nil {
			return loadFailure
		}
		return s.iterate(execCtx, agent, client, toolset, &state, emit)
	})
	// Join the watcher before observing the final cause so it cannot report a
	// late dependency panic after the run has already been marked successful.
	stopWatcherAndWait()
	if failure == nil && execCtx.Err() != nil {
		failure = failureFromCause(context.Cause(execCtx))
	}
	return s.finish(ctx, &state, failure, emit)
}

func (s *Service) watchCancellation(ctx context.Context, runID uuid.UUID, cancel context.CancelCauseFunc) {
	defer func() {
		if recover() != nil {
			s.logExecutionPanic(runID, "cancel_watcher")
			cancel(errExecutionPanic)
		}
	}()
	ticker := time.NewTicker(s.CancelPoll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			requested, err := s.Runs.RunCancelRequested(ctx, runID)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				cancel(err)
				return
			}
			if requested {
				cancel(errCancelRequested)
				return
			}
		}
	}
}
